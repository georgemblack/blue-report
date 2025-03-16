import { AtpAgent, AppBskyFeedPost, AppBskyRichtextFacet } from "@atproto/api";
import {
  SecretsManagerClient,
  GetSecretValueCommand,
} from "@aws-sdk/client-secrets-manager";
import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import {
  DynamoDBDocumentClient,
  UpdateCommand,
  paginateScan,
} from "@aws-sdk/lib-dynamodb";
import * as dotenv from "dotenv";

dotenv.config();

const DYNAMO_FEED_TABLE_NAME = process.env.DYNAMO_FEED_TABLE_NAME;
const BLUESKY_USERNAME = process.env.BLUESKY_USERNAME;
const BLUESKY_PASSWORD = process.env.BLUESKY_PASSWORD;

type Facet = AppBskyRichtextFacet.Main;

interface FeedItem {
  urlHash: string;
  timestamp: string;
  published: boolean;
  content: string;
}

interface FeedItemContent {
  url: string;
  title: string;
  recommended_posts: FeedItemPost[];
}

interface FeedItemPost {
  at_uri: string;
  username: string;
  handle: string;
}

const encoder = new TextEncoder();

const dbClient = new DynamoDBClient({ region: "us-west-2" });
const docClient = DynamoDBDocumentClient.from(dbClient);
const secretsClient = new SecretsManagerClient({ region: "us-west-2" });
const atpAgent = new AtpAgent({ service: "https://bsky.social" });

class UnicodeString {
  utf16: string;
  utf8: Uint8Array;

  constructor(utf16: string) {
    this.utf16 = utf16;
    this.utf8 = encoder.encode(utf16);
  }

  // Helper to convert utf16 code-unit offsets to utf8 code-unit offsets
  utf16IndexToUtf8Index(i: number) {
    return encoder.encode(this.utf16.slice(0, i)).byteLength;
  }
}

async function main() {
  const blueskyPassword = BLUESKY_PASSWORD || (await getBlueskyPassword());
  if (!blueskyPassword) {
    console.error("No Bluesky password found");
    return;
  }

  await atpAgent.login({
    identifier: BLUESKY_USERNAME!,
    password: blueskyPassword!,
  });

  // Scan DynamoDB 'feed' table for unpublished entries
  const paginatedScan = paginateScan(
    { client: docClient },
    {
      TableName: DYNAMO_FEED_TABLE_NAME,
      FilterExpression: "published = :falseVal",
      ExpressionAttributeValues: {
        ":falseVal": false,
      },
    }
  );

  let entries: FeedItem[] = [];
  for await (const page of paginatedScan) {
    if (page.Items) {
      entries.push(...(page.Items as FeedItem[]));
    }
  }

  if (entries.length === 0) {
    console.log("No unpublished feed entries found, exiting");
    return;
  } else {
    console.log(`Found ${entries.length} unpublished feed entries`);
  }

  // Select the first entry to process. Other entries will be processed in subsequent runs.
  const entry = entries[0];
  console.log(`Processing feed entry: '${entry.urlHash}'`);

  // Parse entry contents
  let entryContent: FeedItemContent;
  try {
    entryContent = JSON.parse(entry.content);
  } catch (error) {
    console.error(`Error parsing entry content: ${error}`);
    markAsPublished(entry.urlHash);
    return;
  }

  // Select the first recommended post to quote.
  // If there are no recommended posts, create a post without a quote.
  let recommendedPost: FeedItemPost | undefined;
  if (entryContent.recommended_posts.length > 0) {
    recommendedPost = entryContent.recommended_posts[0];
  }

  // Find the CID of the given post. Convert AT URI into DID and rkey.
  // i.e. 'at://did:plc:u5cwb2mwiv2bfq53cjufe6yn/app.bsky.feed.post/3k44deefqdk2g' -> ['did:plc:u5cwb2mwiv2bfq53cjufe6yn', '3k44deefqdk2g']
  let cid: string | undefined;
  if (recommendedPost) {
    const uriParts = recommendedPost?.at_uri.split("/");
    const repo = uriParts[2];
    const rkey = uriParts[4];
    console.log(`Parsed repo: '${repo}', rkey: '${rkey}' from AT URI`);

    // Fetch the 'top post' associated with the entry from Bluesky.
    // Find the CID of this post, as well as the author's handle.
    try {
      const post = await atpAgent.getPost({ repo: repo, rkey: rkey });
      cid = post.cid;
    } catch (error) {
      console.error(`Error fetching post from Bluesky: ${error}`);
      return;
    }
    console.log(`Fetched post CID: '${cid}'`);
  }

  // Generate post content
  let facets: Facet[] = [];
  let text = `${entryContent.title}`;
  let newPost: Partial<AppBskyFeedPost.Record> = {};

  // Add link facet
  facets.push({
    index: {
      byteStart: 0,
      byteEnd: new UnicodeString(text).utf16IndexToUtf8Index(text.length),
    },
    features: [
      {
        $type: "app.bsky.richtext.facet#link",
        uri: entryContent.url,
      },
    ],
  });

  // Attatch recommended post if it exists
  if (recommendedPost && cid) {
    // For visual appeal, add mention on a new line if the title is long.
    // Otherwise, add it inline.
    if (text.length > 50) {
      text += `\n\nTop post by @${recommendedPost.handle}:`;
    } else {
      text += `. Top post by @${recommendedPost.handle}:`;
    }

    // Find index of '@', start of handle
    const at = text.indexOf("@");

    // Calculate start & end byte for facet
    const unicode = new UnicodeString(text);
    const byteStart = unicode.utf16IndexToUtf8Index(at);
    const byteEnd = unicode.utf16IndexToUtf8Index(
      at + recommendedPost.handle.length + 1
    );

    // Find DID of handle
    const did = recommendedPost.at_uri.split("/")[2];

    // Add handle facet
    facets.push({
      index: {
        byteStart,
        byteEnd,
      },
      features: [
        {
          $type: "app.bsky.richtext.facet#mention",
          did,
        },
      ],
    });

    // Add post as embed
    newPost.embed = {
      $type: "app.bsky.embed.record",
      record: {
        uri: recommendedPost.at_uri,
        cid: cid,
      },
    };
  }

  newPost.text = text;
  newPost.facets = facets;
  await atpAgent.post(newPost);
  console.log(`Post published to Bluesky successfully`);

  // Mark the entry as published
  await markAsPublished(entry.urlHash);
}

async function markAsPublished(urlHash: string) {
  const command = new UpdateCommand({
    TableName: DYNAMO_FEED_TABLE_NAME,
    Key: {
      urlHash,
    },
    UpdateExpression: "SET published = :trueVal",
    ExpressionAttributeValues: {
      ":trueVal": true,
    },
  });
  return await docClient.send(command);
}

async function getBlueskyPassword(): Promise<string> {
  let response;

  try {
    response = await secretsClient.send(
      new GetSecretValueCommand({
        SecretId: "blue-report/bluesky-password",
        VersionStage: "AWSCURRENT",
      })
    );
  } catch (error) {
    console.error("Error fetching secret: ", error);
    return "";
  }

  return response.SecretString || "";
}

main();

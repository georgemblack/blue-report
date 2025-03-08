import { AtpAgent, RichText, AppBskyFeedPost } from "@atproto/api";
import {
  SecretsManagerClient,
  GetSecretValueCommand,
} from "@aws-sdk/client-secrets-manager";
import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import {
  DynamoDBDocumentClient,
  UpdateCommand,
  GetCommand,
  paginateScan,
} from "@aws-sdk/lib-dynamodb";
import * as dotenv from "dotenv";

dotenv.config();

const DYNAMO_FEED_TABLE_NAME = process.env.DYNAMO_FEED_TABLE_NAME;
const DYNAMO_URL_META_TABLE_NAME = process.env.DYNAMO_URL_META_TABLE_NAME;
const BLUESKY_USERNAME = process.env.BLUESKY_USERNAME;
const BLUESKY_PASSWORD = process.env.BLUESKY_PASSWORD;

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

const dbClient = new DynamoDBClient({ region: "us-west-2" });
const docClient = DynamoDBDocumentClient.from(dbClient);
const secretsClient = new SecretsManagerClient({ region: "us-west-2" });
const atpAgent = new AtpAgent({ service: "https://bsky.social" });

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
  let text = `${entryContent.title} ${entryContent.url}`;
  if (recommendedPost && cid) {
    text += `\n\nTop post by @${recommendedPost.handle}:`;
  }
  const richText = new RichText({ text });
  await richText.detectFacets(atpAgent);

  // Create post with external embed
  let newPost: Partial<AppBskyFeedPost.Record> = {
    text: richText.text,
    facets: richText.facets,
  };
  if (recommendedPost && cid) {
    newPost.embed = {
      $type: "app.bsky.embed.record",
      record: {
        uri: recommendedPost.at_uri,
        cid: cid,
      },
    };
  }

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

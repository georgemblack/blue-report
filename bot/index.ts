import { AtpAgent, RichText } from "@atproto/api";
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
  url: string;
  timestamp: string;
  published: boolean;
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
  }

  // Select the first entry to process.
  // Other entries will be processed in subsequent runs.
  const firstEntry = entries[0];
  console.log(`Processing entry: ${firstEntry.urlHash}`);

  // Fetch title and description from DynamoDB 'metadata' table
  const metadataCommand = new GetCommand({
    TableName: DYNAMO_URL_META_TABLE_NAME,
    Key: {
      urlHash: firstEntry.urlHash,
    },
  });
  const metadata = await docClient.send(metadataCommand);

  if (!metadata.Item) {
    console.error(`No metadata found for ${firstEntry.urlHash}, skipping`);
    await markAsPublished(firstEntry.urlHash);
    return;
  } else {
    console.log(`Fetched title from URL metadata: ${metadata.Item.title}`);
  }

  // Fetch the thumbnail for the given URL
  let thumbnail: Uint8Array<ArrayBuffer>;
  try {
    const resp = await fetch(
      `https://assets.theblue.report/thumbnails/${firstEntry.urlHash}.jpg`
    );
    const arrayBuffer = await resp.arrayBuffer();
    thumbnail = new Uint8Array(arrayBuffer);
  } catch (e) {
    console.error(`No thumbnail found for ${firstEntry.urlHash}, skipping`);
    await markAsPublished(firstEntry.urlHash);
    return;
  }
  console.log(`Fetched thumbnail`);

  // Upload the thumbnail to Bluesky
  const { data } = await atpAgent.uploadBlob(thumbnail);

  // Generate post content
  const richText = new RichText({
    text: `#1 on Bluesky: ${metadata.Item.title}`,
  });
  await richText.detectFacets(atpAgent);

  // Create post with external embed
  await atpAgent.post({
    text: richText.text,
    facets: richText.facets,
    embed: {
      $type: "app.bsky.embed.external",
      external: {
        uri: firstEntry.url,
        title: metadata.Item.title,
        description: "",
        thumb: data.blob,
      },
    },
  });
  console.log(`Post published to Bluesky successfully`);

  // Mark the entry as published
  await markAsPublished(firstEntry.urlHash);
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

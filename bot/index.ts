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
  const blueskyPassword = BLUESKY_PASSWORD;
  if (!blueskyPassword) {
    console.error("No Bluesky password found");
    return;
  }

  await atpAgent.login({
    identifier: BLUESKY_USERNAME!,
    password: blueskyPassword!,
  });

  // Generate post content
  let facets: Facet[] = [];
  let text = `The Blue Report`;
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
        uri: "https://theblue.report",
      },
    ],
  });

  text +=
    " is a site that rounds up the most popular links on Bluesky. This account hosts feeds to view them in-app:\n\n- ";

  const firstFeed = "Top Links (past day)";
  const secondFeed = "Trending Links (past hour)";
  const thirdFeed = "Best Links (past week)";

  text += `${firstFeed}`;

  facets.push({
    index: {
      byteStart: new UnicodeString(text).utf16IndexToUtf8Index(
        text.length - firstFeed.length
      ),
      byteEnd: new UnicodeString(text).utf16IndexToUtf8Index(text.length),
    },
    features: [
      {
        $type: "app.bsky.richtext.facet#link",
        uri: "https://bsky.app/profile/theblue.report/feed/toplinksday",
      },
    ],
  });

  text += `\n- ${secondFeed}`;

  facets.push({
    index: {
      byteStart: new UnicodeString(text).utf16IndexToUtf8Index(
        text.length - secondFeed.length
      ),
      byteEnd: new UnicodeString(text).utf16IndexToUtf8Index(text.length),
    },
    features: [
      {
        $type: "app.bsky.richtext.facet#link",
        uri: "https://bsky.app/profile/theblue.report/feed/toplinkshour",
      },
    ],
  });

  text += `\n- ${thirdFeed}`;

  facets.push({
    index: {
      byteStart: new UnicodeString(text).utf16IndexToUtf8Index(
        text.length - thirdFeed.length
      ),
      byteEnd: new UnicodeString(text).utf16IndexToUtf8Index(text.length),
    },
    features: [
      {
        $type: "app.bsky.richtext.facet#link",
        uri: "https://bsky.app/profile/theblue.report/feed/toplinksweek",
      },
    ],
  });

  const here = "More info here";
  text += `\n\nFor the nerds, Atom/JSON feeds are also available! ${here}`;

  facets.push({
    index: {
      byteStart: new UnicodeString(text).utf16IndexToUtf8Index(
        text.length - here.length
      ),
      byteEnd: new UnicodeString(text).utf16IndexToUtf8Index(text.length),
    },
    features: [
      {
        $type: "app.bsky.richtext.facet#link",
        uri: "https://theblue.report/about",
      },
    ],
  });

  text += `.`;

  newPost.embed = {
    $type: "app.bsky.embed.record",
    record: {
      cid: "bafyreia3jwwvzrqkm32nausivhhe7do7zukub3ht32skf2tvw6iq4e44o4",
      uri: "at://did:plc:zrcqicmkxum6tir6ahthppif/app.bsky.feed.generator/toplinksday",
    },
  };

  newPost.text = text;
  newPost.facets = facets;
  await atpAgent.post(newPost);
  console.log(`Post published to Bluesky successfully`);
}

main();

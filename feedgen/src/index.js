export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    if (url.pathname === "/.well-known/did.json") {
      return handleDidRequest();
    }
    if (url.pathname === "/xrpc/app.bsky.feed.getFeedSkeleton") {
      return handleFeedRequest(env);
    }
    return notFoundError();
  },
};

async function handleDidRequest() {
  return new Response(
    JSON.stringify({
      "@context": ["https://www.w3.org/ns/did/v1"],
      id: "did:web:feedgen.theblue.report",
      service: [
        {
          id: "#bsky_fg",
          type: "BskyFeedGenerator",
          serviceEndpoint: "https://feedgen.theblue.report",
        },
      ],
    }),
    {
      headers: {
        "Content-Type": "application/json",
      },
    }
  );
}

async function handleFeedRequest(env) {
  // Fetch & parse site data from R2
  const data = await env.BLUE_REPORT.get("data/top-links.json");
  if (!data) {
    return internalError();
  }
  let parsed;
  try {
    parsed = JSON.parse(data);
  } catch (e) {
    return internalError();
  }
  if (!parsed) {
    return internalError();
  }

  // Aggregate AT URIs of top posts
  const atUris = [];
  for (const link of parsed.top_day) {
    for (const post of link.recommended_posts) {
      atUris.push(post.at_uri);
    }
  }

  return new Response(
    JSON.stringify({
      feed: [
        atUris.map((atUri) => ({
          post: atUri,
        })),
      ],
    }),
    {
      headers: {
        "Content-Type": "application/json",
      },
    }
  );
}

function notFoundError() {
  return new Response("Not Found", {
    status: 404,
  });
}

function internalError() {
  return new Response("Internal error", {
    status: 500,
  });
}

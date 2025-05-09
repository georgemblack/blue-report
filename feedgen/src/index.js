export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // Serve request from cache if available
    const cacheKey = new Request(url.toString(), request);
    const cache = caches.default;
    let response = await cache.match(cacheKey);
    if (response) {
      return response;
    }

    // Build new response & cache it
    if (url.pathname === "/.well-known/did.json") {
      response = handleDidRequest();
    } else if (url.pathname === "/xrpc/app.bsky.feed.getFeedSkeleton") {
      response = await handleFeedRequest(env);
    } else {
      return notFoundError();
    }
    ctx.waitUntil(cache.put(cacheKey, response.clone()));

    return response;
  },
};

function handleDidRequest() {
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
        "Cache-Control": "public; max-age=3600",
        "Content-Type": "application/json",
      },
    }
  );
}

async function handleFeedRequest(env) {
  // Fetch & parse site data from R2
  const data = await env.BLUE_REPORT.get("data/top-links.json");
  if (!data) {
    console.log({ message: "failed to fetch data from r2" });
    return internalError();
  }
  let parsed;
  try {
    parsed = JSON.parse(data);
  } catch (e) {
    console.log({ message: "failed to parse data from r2; " + e });
    return internalError();
  }
  if (!parsed) {
    console.log({ message: "failed to parse data from r2" });
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
        "Cache-Control": "public; max-age=600",
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

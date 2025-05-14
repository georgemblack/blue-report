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
      response = await handleFeedRequest(url, env);
    } else {
      return notFoundError();
    }

    if (response.status === 200) {
      try {
        ctx.waitUntil(cache.put(cacheKey, response.clone()));
      } catch (e) {
        console.log({ message: "failed to cache response; " + e });
      }
    }

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
        "Cache-Control": "public; max-age=28800",
        "Content-Type": "application/json",
      },
    }
  );
}

async function handleFeedRequest(url, env) {
  // Fetch & parse site data from R2
  const object = await env.BLUE_REPORT.get("data/top-links.json");
  if (!object) {
    console.log({ message: "failed to fetch object from r2" });
    return internalError();
  }
  let parsed;
  try {
    parsed = await object.json();
  } catch (e) {
    console.log({ message: "failed to parse object from r2; " + e });
    return internalError();
  }
  if (!parsed) {
    console.log({ message: "failed to parse object from r2" });
    return internalError();
  }

  // Aggregate AT URIs for each list (top hour, top day, top week)
  const atUris = {
    top_hour: [],
    top_day: [],
    top_week: [],
  };

  for (const field of ["top_hour", "top_day", "top_week"]) {
    for (const link of parsed[field]) {
      if (link.recommended_posts.length > 0) {
        atUris[field].push(link.recommended_posts[0].at_uri);
      }
    }
  }

  // Construct a feed that mixes posts from all three lists.
  //  - Show 5 top_hour posts that blend into top_day posts
  //  - Show 10 top_day posts that blend into top_week posts
  //  - Show 10 top_week posts
  const pattern = [
    ["top_hour", 0],
    ["top_hour", 1],
    ["top_hour", 2],
    ["top_day", 0],
    ["top_hour", 3],
    ["top_day", 1],
    ["top_hour", 4],
    ["top_day", 2],
    ["top_day", 3],
    ["top_day", 4],
    ["top_day", 5],
    ["top_day", 6],
    ["top_week", 0],
    ["top_day", 7],
    ["top_week", 1],
    ["top_day", 8],
    ["top_week", 2],
    ["top_day", 9],
    ["top_week", 3],
    ["top_week", 4],
    ["top_week", 5],
    ["top_week", 6],
    ["top_week", 7],
    ["top_week", 8],
    ["top_week", 9],
  ];
  const feed = [];
  for (const [field, index] of pattern) {
    if (atUris[field].length > index) {
      feed.push(atUris[field][index]);
    }
  }

  // Remove duplicates from the feed, keeping the first occurrence
  const seen = new Set();
  const deduped = [];
  for (const post of feed) {
    if (!seen.has(post)) {
      seen.add(post);
      deduped.push(post);
    }
  }

  // If the 'limit' query parameter is present, truncate the feed to that length
  const limit = parseInt(url.searchParams.get("limit"), 10);
  if (!isNaN(limit) && limit > 0) {
    deduped.splice(limit);
  }

  return new Response(
    JSON.stringify({
      feed: deduped.map((atUri) => ({
        post: atUri,
      })),
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

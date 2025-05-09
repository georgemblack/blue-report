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
        "Cache-Control": "public; max-age=3600",
        "Content-Type": "application/json",
      },
    }
  );
}

async function handleFeedRequest(url, env) {
  // Using the request URL, find the name of the JSON field containing the data we want
  const dataField = dataFieldName(url);
  if (!dataField) {
    console.log({ message: "invalid feed in url" });
    return notFoundError();
  }

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

  // Aggregate AT URIs of top posts
  const atUris = [];
  for (const link of parsed[dataField]) {
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

// AppViews request feeds via the following URL structure:
//  - https://feedgen.theblue.report/xrpc/app.bsky.feed.getFeedSkeleton?feed=at://${DID}/app.bsky.feed.generator/${ID}
// Parse the AT URI of the feed, and return the name of the JSON field containing the data we want.
//  - 'toplinkshour' -> 'top_hour'
//  - 'toplinksday' -> 'top_day'
//  - 'toplinksweek' -> 'top_week'
function dataFieldName(url) {
  // Check for AT URI of feed in 'feed' query param
  const atUri = url.searchParams.get("feed");
  if (!atUri) {
    return null;
  }

  // Parse AT URI
  const parts = atUri.split("/");
  const name = parts[parts.length - 1];
  if (!name) {
    return null;
  }

  // Check for valid feed name
  const validNames = ["toplinkshour", "toplinksday", "toplinksweek"];
  if (!validNames.includes(name)) {
    return null;
  }

  // Map feed name to JSON field name
  const fieldName = name.replace("toplinks", "top_");
  return fieldName;
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

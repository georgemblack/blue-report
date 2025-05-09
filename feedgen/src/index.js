export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    if (url.pathname === "/.well-known/did.json") return handleDidRequest();
    if (url.pathname === "/xrpc/app.bsky.feed.getFeedSkeleton")
      return handleFeedRequest();
    return new Response("Not Found", {
      status: 404,
    });
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

async function handleFeedRequest() {
  return new Response(
    JSON.stringify({
      data: "ahh",
    }),
    {
      headers: {
        "Content-Type": "application/json",
      },
    }
  );
}

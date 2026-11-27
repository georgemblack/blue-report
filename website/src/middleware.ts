import { defineMiddleware } from "astro:middleware";
import { V2_HOST } from "./util/Data";

// Cloudflare caches the worker's pages without looking at the hostname, so
// theblue.report and v2.theblue.report would otherwise share cached pages.
// "Vary: Host" asks the cache to keep a separate copy per hostname, and v2
// pages are never cached, so they can't be served to theblue.report visitors.
export const onRequest = defineMiddleware(async (context, next) => {
  const response = await next();
  response.headers.append("Vary", "Host");
  if (context.url.hostname === V2_HOST) {
    response.headers.set("Cache-Control", "private, no-store");
  }
  return response;
});

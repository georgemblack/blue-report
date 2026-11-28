# Future Enhancements

Ideas for the v2 engine that aren't needed for the switch-over. None of these are scheduled, so each needs a decision before it's built.

## Browser Run in more places

The hydrator already uses Cloudflare Browser Run (`internal/browserrun`) when CardyB can't find a page's title or image. It runs at most once per link while the link is published. Browser Run is billed by browser time: the paid plan includes 10 hours a month, then $0.09 an hour, and every response reports the time used (`X-Browser-Ms-Used`, logged as `browser_ms`).

### Resolving Apple News links

`apple.news` links redirect with JavaScript, so the resolver's plain redirect check can't follow them, and it skips them today. v1 loaded these pages with Browser Rendering (Browser Run's old name) and took the first link on the page that wasn't an Apple one.

- **How:** in `internal/resolve`, for Apple News links, call `/scrape` with the selector `a` and take the first `href` that isn't on an Apple host (`urltools.IsAppleURL`). Then clean and save it like any other redirect.
- **Cost:** small. Few Apple News links reach 1,000 users, and each answer is saved for 10 days.

### Other JavaScript redirects and blocked sites

The plain check only follows HTTP redirects. If a site redirects with JavaScript, or blocks our request, the check finds nothing and the link is saved as "doesn't redirect". Browser Run doesn't report the address the browser ended up at, but the loaded page usually names its own address in `link[rel=canonical]` or `meta[property="og:url"]`.

- **How:** only when the plain check finds nothing, load the page with Browser Run and use its canonical address if it's on a different link.
- **First:** measure how often the plain check finds nothing for popular links, and look at a sample by hand, to see whether it's worth it.
- **Watch out:** some sites set the canonical address wrongly, for example to their home page. Ignore canonical addresses that are just a site's front page.

### Merging versions of the same article

Many pages name a canonical address even without any redirect, for example AMP pages, `m.` sites and syndicated copies. Using it could merge versions of the same story. It's a bigger change to the rankings than the items above, and it has the same canonical-address risks, so compare results carefully before turning it on.

## Filtering spam and junk with Cloudflare clef

Run links, and the posts recommended for them, through Cloudflare's clef to keep spam and junk out of the published rankings and the feed.

- **Where:** in the publisher, after hydrating and before building `top-links.json`. Drop flagged links (letting the next one move up) and flagged recommended posts. Only the 30 or so published links and their posts need checking, so it's a handful of calls per publish.
- **Remember answers** the way titles are remembered, so a link isn't checked on every publish.
- **Credentials:** probably the same Cloudflare API token (`/blue-report-engine/cloudflare-api-token`), with clef's permission added. Check clef's docs for how it authenticates.
- **Decide first** whether flagged links should be hidden completely, or only kept out of the feed.

## Smaller ideas

- **A health check** (`/health`) for ECS, so a stuck engine is restarted automatically.
- **Last successful write and publish times in the `stats` line**, which the original plan listed but which aren't logged yet.
- **Retrying feed entries for links that never get a title.** Today such a link never enters the feed. That's fine, but it could be logged.
- **IPv6-only networking**, to save the cost of a public IPv4 address (about $3.65 a month), once Jetstream, CardyB, the Bluesky API and Cloudflare are confirmed to support IPv6.
- **Top sites.** The `/sites` page is being removed at switch-over. If it's wanted again, v2 could publish `top-sites.json` by grouping the ranking by host.

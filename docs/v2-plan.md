# The Blue Report v2: Rebuild Plan

This document is the plan for rebuilding the Blue Report backend from scratch. It's written for whoever (person or agent) builds it. It records **what has been decided and why**, **what's still open**, and **the order to build in**.

Read [event-schema.md](event-schema.md) alongside this. It defines the v1 and v2 event formats in S3.

---

## 1. Goals

1. **Cost.** Cheap to run long-term. The current backend costs about **$35/month** at full prices. The target is under **$10–15/month**.
2. **Simplicity.** Few moving parts and few AWS services, easy to run without ongoing attention.
3. **Freshness.** Rankings computed continuously and published about every minute, instead of hourly.

## 2. The current system (v1) in brief

- **intake:** a Fargate service that reads the Bluesky Jetstream. It uses ElastiCache Valkey to map posts to links and to count interactions per link, writes JSON Lines events to S3, and sends links with ≥1,000 interactions to SQS.
- **link_aggregation:** a Fargate Spot task, hourly. It re-reads the last 7 days of events from S3, applies redirect translations from DynamoDB, ranks links, fetches titles and thumbnails (CardyB → Cloudflare browser rendering → OpenAI for PDFs), and publishes `top-links.json` plus the Atom and JSON feeds to R2.
- **site_aggregation:** daily. Ranks top domains over 30 days and publishes `top-sites.json`.
- **link_redirect:** every 20 minutes. Takes URLs from SQS, follows redirects, and saves translations to DynamoDB.
- The website (Astro) and feedgen run on Cloudflare Workers and read data from R2 (`data.theblue.report`).

October 2026 at full prices: Fargate intake about $14.70, ElastiCache about $9.50, public IPv4 about $4.20, Secrets Manager $2.50, S3 about $2.30, Fargate Spot jobs about $1.30, cross-AZ transfer about $0.70.

### v1 problems that v2 must not carry over

- **Duplicate removal is order-dependent and probabilistic** (`pkg/links/aggregation.go`). It uses a Bloom filter where the first event seen wins, while files are processed in parallel in no particular order. A user's old like can be kept and their recent one dropped, which undercounts the hour and day rankings. The filter also drops about 0.1% of new events by mistake.
- **Translations aren't paged** (`pkg/storage/translation.go`). Only the first 1 MB of each DynamoDB query is read, and only this month's and last month's entries are loaded.
- **Events record processing time** (`time.Now()`) rather than Bluesky event time.
- **The CardyB URL isn't escaped** (`pkg/app/card.go`): `"…?url=" + url`. Links containing `&` or `#` get cut short.
- **Losing the cache loses data.** If Valkey is wiped, likes on posts made before the wipe are never counted.

## 3. Decisions (settled)

| # | Decision |
|---|---|
| D1 | **No Valkey or ElastiCache.** All state lives in intake's memory. |
| D2 | **On startup, rebuild memory from S3** by replaying recent Parquet events. |
| D3 | **The newest `time_us` in S3 is the resume point.** No separate cursor file. Files are named after their newest `time_us`. On shutdown, intake writes pending events to S3 before exiting. |
| D4 | **ECS Fargate** for compute. Spot or on-demand is still open. |
| D5 | **Real-time ranking** inside intake, published to R2 about every minute. |
| D6 | **Parquet** for event storage. The schema is in [event-schema.md](event-schema.md). |
| D7 | **Build fresh, run in parallel, then switch over.** v2 has **completely separate AWS infrastructure**: nothing shared with v1. |
| D8 | **One container**: a single Go program with several goroutines (section 5). |
| D9 | **State has a single owner**: one goroutine owns all ranking and cache state, and others talk to it through channels (the actor pattern). |
| D10 | **Event schema:** `time_us`, `type` (0 = post including quotes, 1 = repost, 2 = like), `did`, `rkey`, `subject_uri`, `url`. |
| D11 | **Use Jetstream event time** everywhere, never processing time. Jetstream v2 sends it as `time` (RFC 3339); we store it as `time_us` (microseconds). |
| D12 | **Normalize when reading.** S3 stores the cleaned URL before translation. Translations are applied in memory, and applied again to history on every rebuild. |
| D13 | **Deletes are ignored**, as in v1 (`RANKING.md`: removals aren't counted). |
| D14 | **Keep DynamoDB** for small lookup data: translations, titles, feed. |
| D15 | **Drop top sites**: no sites page, no `top-sites.json`, no 30-day job. |
| D16 | **Drop OpenAI**: no LLM-generated titles. |
| D17 | **Backfill is out of scope** for now. Converting v1 JSON to v2 Parquet will be planned separately. |
| D18 | **Same AWS region as v1:** us-west-2. |
| D19 | **Jetstream v2** (`jetstream.us-west.bsky.network` / `jetstream.us-east.bsky.network`), not v1. v1 still runs but v2 is recommended for new projects. Docs: https://bsky.network/docs/jetstream/ |

## 4. Architecture overview

```
                        ┌──────────────── intake (one ECS Fargate task, one container) ─────────────────┐
Bluesky Jetstream ────► │ reader ─► state actor ─► writer ─────────────────────────────► S3 (Parquet)    │
                        │              ▲   │                                                           │
                        │   publisher ─┘   └─► resolver ─────────────────────────────► DynamoDB          │
                        │      │                hydrator ──► CardyB / browser rendering / Bluesky API    │
                        │      └──────────────────────────────────────────────────────► R2 (JSON, feeds) │
                        └───────────────────────────────────────────────────────────────────────────────┘
                                                                   R2 ◄── Cloudflare Worker (website, feedgen)
```

**AWS resources (all new, prefixed so they can't be confused with v1):**
- ECS cluster + one service (1 task) + task definition
- S3 bucket for events
- 3 DynamoDB tables
- ECR repository
- CloudWatch log group
- SSM Parameter Store secrets
- IAM roles
- a VPC, or the default VPC (open question)

That's no ElastiCache, SQS, EventBridge Scheduler or Secrets Manager.

## 5. The intake program

One Go binary. Its goroutines:

### 5.1 `main` (supervisor)
- **Startup, in order:**
  1. Load config.
  2. Load all translations from DynamoDB (paging through every result).
  3. Find the resume point: list the newest `events/date=…/hour=…/` folders and take the largest file name.
  4. Rebuild state from S3 (section 7).
  5. Start the goroutines.
  6. Connect the reader to Jetstream with `cursor=<resume point>`.
- **Shutdown** (SIGTERM; ECS `stopTimeout` = 120 s):
  1. Stop the reader.
  2. Let the state actor finish everything queued.
  3. The actor sends its last batch to the writer.
  4. Wait for the writer to finish.
  5. Exit.
- Serves `/health` (optional; useful for ECS health checks).

### 5.2 `reader`
- Holds the Jetstream v2 WebSocket connection: `/xrpc/network.bsky.jetstream.subscribeEvents` with `collections` = post, repost, like and `kinds=commit`. No authentication is needed for the live stream.
- Each v2 message is `{"$type":"message","payload":{…}}`. Commit payloads are flat: `did`, `seq`, `time`, `operation`, `collection`, `rkey`, `record`. There's no `time_us`, `kind` or `commit` wrapper as in v1.
- Parses messages and drops everything except `kind=commit`, `operation=create`. For posts, keeps only English (`langs` contains `en`). (The "@theblue.report account is excluded" rule in `RANKING.md` has been dropped; it was never implemented in v1 either.)
- Sends parsed events to the state actor through a buffered channel.
- On disconnect, reconnects with `cursor` set to the newest `time_us` it has seen. v2 accepts a microsecond timestamp as the cursor and starts at the first event at or after it, so expect a small overlap (delivery is at-least-once anyway). It may switch Jetstream servers. (v2 also has a `seq` number per event; we resume by time so the S3 file names can stay the resume point.)
- Touches no shared state.

### 5.3 `state actor`: the only owner of state
A `for { select { … } }` loop. It owns the **post cache**, **ranking state**, **translations**, **checked-links set** and **event buffer** (section 6). Messages it handles:

| Message / timer | What it does |
|---|---|
| Event from the reader | Per-event flow (section 8). |
| `Top` request (from the publisher) | Computes the top N for hour, day and week, plus recommended-post candidates and redirect candidates. Replies with a small copied result. |
| `Merge(src, dst)` (from the resolver) | Adds the translation and merges link `src` into `dst` in the ranking state (section 6.2). |
| Write timer / buffer full | Hands the buffer to the writer (handing over ownership) and starts a new one. |
| Sweep timer (~10 min) | Sweeps the post cache and ranking state. |
| Translations reload (optional) | Replaces the translations map. |

Rule: **no network or disk work inside the actor.** Everything slow happens in other goroutines using results the actor sends out.

### 5.4 `writer`
- Receives completed batches one at a time, in order.
- Encodes each batch as Parquet (zstd), sets the file metadata (`schema_version`, `min_time_us`, `max_time_us`), and uploads to `events/date=YYYY-MM-DD/hour=HH/<max_time_us>.parquet`, using the UTC date and hour of the newest event.
- On failure, retries with an increasing delay. It **never skips a batch**, because the newest file defines the resume point.
- Limits how many batches can wait. If S3 stays down past the limit, stop reading from Jetstream rather than run out of memory, and replay later from the resume point. (Open: whether to apply that back-pressure or drop data.)

### 5.5 `publisher`
- Every **~60 seconds**, sends `Top` to the actor and receives the result. Then, outside the actor:
  - fills in titles, thumbnails and recommended posts from the hydrator's metadata cache
  - builds `top-links.json` (same shape as v1; see section 10) and uploads it to R2
  - passes the current top links to the hydrator, and redirect candidates to the resolver
- Every publish, also checks the feed rule (section 10.2) and republishes the Atom and JSON feeds when it changes.
- **Doesn't publish until the startup rebuild is done and the reader has caught up**, meaning the newest `time_us` is within about 60 s of the clock.

### 5.6 `resolver` (worker pool, about 4)
- Takes redirect candidates. It first checks DynamoDB: skip if a translation or a "no redirect" record exists.
- Otherwise:
  - Apple News URLs: Cloudflare browser rendering, taking the first non-Apple `<a href>` (as v1 does).
  - Everything else: follow HTTP redirects (reuse v1's `urltools.FindRedirect`).
- Cleans the destination, then saves to DynamoDB:
  - a redirect: `{source, destination, expires_at = now + 90d}`
  - no redirect: `{source, no_redirect: true, expires_at = now + 7d}`
- On a redirect, sends `Merge(source, destination)` to the actor.
- Strict timeouts on every request.

### 5.7 `hydrator` (worker pool, small)
Owns the **metadata cache**: final URL → title, thumbnail URL, recommended posts and when it was fetched. It's guarded by a mutex, because the hydrator writes it and the publisher reads it.

For each link in the current top N with missing or stale metadata:

1. **Title and thumbnail:** check the DynamoDB metadata table. Otherwise use **CardyB** (`https://cardyb.bsky.app/v1/extract?url=<QueryEscape(url)>`), then Cloudflare browser rendering for the `og:title` and `og:image` tags. **No OpenAI.** Fallback title: the file name for PDFs, otherwise `"(No Title)"`. Copy the thumbnail into R2 at `thumbnails/<url hash>.<ext>` and save the title to DynamoDB. Limit concurrent CardyB requests; v1 waits 1 s between calls "so Bluesky doesn't block us".
2. **Recommended posts:** from the link's top 20 posts by interaction count (from the actor's `Top` result), fetch posts from the Bluesky API and keep up to 3 that meet these rules:
   - the formatted text isn't empty
   - English
   - more than 50 likes
   - no more than one per author

   Reuse v1's `formatPost`. **Cache them per link for about 10–15 minutes**, so publishing every minute doesn't call the Bluesky API every minute.

### 5.8 Reused v1 code

Copy these into the new module rather than importing them, so v1 stays independent and untouched:
- `urltools` (Clean, Ignore, FindRedirect, Apple helpers, Hostname) and its tests
- the Bluesky client (GetPost)
- `rendering` (Cloudflare browser rendering)
- the CardyB and title logic in `card.go`, without the LLM step and with the escaping fix
- `formatTitle` and `formatPost`
- Atom and JSON feed generation (`feed.go`)
- R2 client setup (`storage.go`)

## 6. In-memory data structures (owned by the state actor)

### 6.1 Post cache: post address → link
- **Purpose:** credit likes, reposts and quotes to the link in the original post.
- **Layout:** `map[uint64]entry`. The key is a 64-bit hash of the post address (AT URI); `entry{url string, lastSeen uint32}`. Times are in minutes since the Unix epoch.
- **Lookup:** if `lastSeen` is more than 12 h old, treat it as missing. If found, set `lastSeen = now` (this refresh keeps active posts cached, like Valkey's `EXPIRE` did).
- **Sweep:** about every 10 min, delete entries older than 12 h.
- **Safety limit:** if the map passes a configured size (e.g. 2M), log a warning and sweep with a shorter time limit.
- **Collisions:** with 64-bit hashes at about 500k entries, a collision is roughly a 1-in-10⁸ event, and its effect would be one interaction credited to the wrong link. That's acceptable.

### 6.2 Ranking state: final URL → who interacted, and when
```go
type link struct {
    url   string
    users map[uint64]uint32   // userKey(didHash, type) → lastSeen (minutes since epoch)
    posts map[uint64]postStat // subject_uri hash → {uri, count, lastSeen}, for recommended posts
}
links map[uint64]*link        // hash of final URL → link
func userKey(didHash uint64, typ uint8) uint64 { return didHash&^3 | uint64(typ) }
```
- **Recording:** `users[userKey] = max(existing, eventTime)`. There's one entry per (link, user, type), which is the deduplication rule. Also add one to `posts[subject].count` and update its `lastSeen`.
- **Counting for a window:** the number of entries with `lastSeen ≥ windowStart`, per type. A user counts if they did that thing at least once in the window, which holds exactly when their latest time is in the window.
- **Score:** `10*posts + 10*reposts + likes` (same as v1 `item.go`).
- **Top N:** one pass over all links, counting hour, day and week together, keeping each window's top N in a small min-heap.
- **Sweep:** delete user and post entries older than 7 days, and delete links left empty.
- **Merge(src, dst):** for each entry in `src.users`, set `dst.users[k] = max(dst[k], src[k])`; combine `posts` the same way, adding counts. Delete `src`. A user who interacted with both versions ends up counted once.
- **Memory:** to be measured (section 12). Optional: store users in a small slice until a link passes about 16 users, then switch to a map. Most links get very few interactions, so this saves a lot.

### 6.3 Translations: cleaned URL → destination
Loaded from DynamoDB at startup. Added to by `Merge`. `final(url)` follows the chain with a limit of 5 hops, cleaning and checking `Ignore` after each hop. If the destination is ignored, the event is dropped.

### 6.4 Checked-links set
Final URLs already sent to the resolver, so they aren't sent again. Expiry isn't critical; it can be cleared on sweep.

### 6.5 Event buffer
`[]Event` rows waiting to be written. The whole buffer is handed to the writer, and the actor starts a new one.

## 7. Startup rebuild

1. **Ranking state:** read the last 7 days of Parquet (`time_us, type, did, url, subject_uri`). For each row: `url → final(url)`, skipping ignored ones, then `Record`. Because `Record` keeps the latest time, row order doesn't matter, and duplicates are harmless.
2. **Post cache:** from the same rows, for `time_us` within the last 12 h, set `postCache[hash(subject_uri)] = {url, max(time)}`. Every row (post, like, repost or quote) is evidence that its subject post was active, which matches Valkey's refresh behavior.
3. Rows are read in a streaming fashion so the full 7 days is never in memory at once.
4. **Edge cases:**
   - If the bucket is empty (first run), start from live (no cursor).
   - If the resume point is older than Jetstream's replay window (about 36 hours), v2 still connects: it sends an `#info` message named `OutdatedCursor` and starts at the oldest event it has. Log it as a gap.

**"Now" for windows:** use the newest event time seen, not the clock, so windows stay correct while catching up. Publishing is held back until caught up anyway.

## 8. Per-event flow (in the state actor)

1. **Post (not a quote):**
   - Get the link from the external embed or a link facet (v1 `ParsePost`).
   - Skip if it's empty or matches `Ignore`.
   - `cleaned = Clean(link)`, then `postCache.Put(postURI, cleaned)`.
2. **Quote post:** look up `postCache.Get(quotedURI)`. Skip if it's missing.
3. **Like or repost:** look up `postCache.Get(subjectURI)`. Skip if it's missing.
4. **Ranking:** `ranking.Record(final(cleaned), did, type, eventTime)`, and add one to `posts[subject]`.
5. **Buffer:** append `{time_us, type, did, rkey, subject_uri, url: cleaned}`. For a post, `subject_uri` is the post itself; for a quote, the quoted post; for a like or repost, the post liked or reposted.

Note: posts are looked up by their **address (AT URI)**, not by CID as in v1. Likes, reposts and quotes include both.

## 9. Storage

### 9.1 S3: events bucket
- Layout and columns: [event-schema.md](event-schema.md).
- Write about every **5 minutes** (configurable) or when the buffer hits N events, whichever comes first.
- Encryption: SSE-S3. Block all public access.
- **Lifecycle:** keep at least 7 days for rebuilds. Retention beyond that is open (section 13); the default is to keep everything, moving to cheaper storage after 30 days.

### 9.2 DynamoDB (all `PAY_PER_REQUEST`, TTL attribute `expires_at`)
| Table | Key | Attributes | Used by |
|---|---|---|---|
| `…-url-translations` | `source_url` | `destination` (empty if no redirect), `no_redirect`, `checked_at`, `expires_at` | resolver writes; startup reads all of it (paging through results) |
| `…-url-metadata` | `url_hash` | `title`, `thumbnail_key`, `updated_at` | hydrator |
| `…-feed` | `url_hash` | `timestamp`, `content` (JSON), `expires_at` (+90d) | publisher |

### 9.3 Cloudflare R2: published output
| Key | Content | Cache-Control |
|---|---|---|
| `data/top-links.json` | Rankings (section 10.1) | short, e.g. `max-age=60` |
| `feeds/top-day.xml` | Atom feed | `max-age=600` |
| `feeds/top-day.json` | JSON Feed | `max-age=600` |
| `thumbnails/<url hash>.<ext>` | Thumbnail images | `max-age=28800` |

The v2 R2 bucket or prefix and its public hostname are open (section 13). During the parallel run, v2 must not write over v1's objects.

### 9.4 Secrets: SSM Parameter Store (SecureString)
- Cloudflare account ID
- Cloudflare API token (browser rendering)
- R2 access key ID
- R2 secret access key

They're passed to the container as environment variables via the ECS task definition's `secrets`. There's no OpenAI key and no Secrets Manager.

## 10. Published data

### 10.1 `top-links.json`
Keep **v1's exact shape**, so switching the website over is just changing the URL:
```json
{
  "generated_at": "RFC3339",
  "top_hour": [Link], "top_day": [Link], "top_week": [Link]
}
Link = { "rank", "url", "title", "thumbnail_url", "post_count", "repost_count", "like_count",
         "recommended_posts": [ { "rank", "at_uri", "username", "handle", "text" } ] }
```
- 10 links per list (v1 `ListSize`).
- As in v1, `post_count`, `repost_count` and `like_count` are **week** counts in all three lists.
- Links without a title yet: publish with the fallback title, or leave them out until hydrated. Pick one and note it.

### 10.2 Feed
Carry over v1's rules (`publish.go`, `storage/feed.go`):
- If the top link of the day isn't already in the feed, **and** no feed entry was added in the last 12 h, add it (with its recommended posts).
- Entries expire after 90 days (DynamoDB TTL replaces v1's manual cleanup).
- Regenerate and upload the Atom and JSON feeds when the feed changes.

Keep the newest feed entry's time in memory (loaded at startup), so the 12-hour check doesn't scan DynamoDB every minute.

## 11. Infrastructure (Terraform)

- A new Terraform root with its **own state** (a separate key, or a separate state bucket). It must not touch v1's state.
- **ECS service:**
  - Fargate, ARM64, desired count 1
  - **`deployment_minimum_healthy_percent = 0`, `deployment_maximum_percent = 100`**, so two copies of intake never run at once
  - `stopTimeout = 120`
  - Spot or on-demand: open
- **Task size:** start at **0.5 vCPU / 2 GB**, then adjust after measuring (section 12). Set `GOMEMLIMIT` to about 85% of task memory.
- **Container image:** built with `CGO_ENABLED=0` if possible (see the Parquet library in section 13), static binary, distroless or alpine with `tzdata`.
- **IAM task role:** S3 read and write on the events bucket only; DynamoDB on the three tables only; SSM read on the v2 parameters. **Execution role:** ECR pull, logs, SSM.
- **Logs:** CloudWatch, 7-day retention.
- **Networking:** public subnet with a public IP (no NAT gateway, as in v1). IPv6-only is open.
- **Budget alert** in AWS Budgets.

## 12. Observability and validation

**Log a stats line every minute** (structured, `slog` JSON):
- events received and recorded per second
- **lag** (`now − newest time_us`)
- post cache size
- ranking: links and user entries
- buffer size
- time of the last successful write
- time of the last successful publish
- `Top` computation time
- memory in use (heap)

**Alerting (cheap):**
- a CloudWatch alarm on a "published" metric (or a log metric filter) going quiet for more than 10 minutes, or
- the Worker checking `generated_at` age

**Measure before choosing the final task size and Spot:**
1. Distinct `(url, did, type)` combinations and distinct links in a week of real data. This sets ranking memory.
2. Startup rebuild time for 7 days at the chosen CPU.
3. `Top` computation time at full size.
4. ~~Jetstream's replay window~~ Known: about 36 hours on Bluesky's v2 servers (a 30-hour-old cursor worked in testing; 72 hours returned `OutdatedCursor`).

**Parallel-run validation:**
- Compare v2's `top-links.json` with v1's over several days. Expect differences: v1 undercounts the hour and day rankings (section 2) and publishes hourly.
- Agree with the owner on what counts as "close enough" before switching.

## 13. Open questions (settle these; the default applies if nobody answers)

| Question | Default |
|---|---|
| Fargate Spot or on-demand? | On-demand during the parallel run; revisit after measurements. |
| Parquet library? | Pure Go (`github.com/parquet-go/parquet-go`) for writing and the startup rebuild, keeping builds free of C compilation. DuckDB only for ad-hoc analysis and the future backfill. |
| Separate AWS account for v2? | Same account, separate resources with a `blue-report-v2` prefix. |
| v2 R2 bucket and public hostname during the parallel run? | New R2 bucket `blue-report-v2` on a temporary hostname (e.g. `data-v2.theblue.report`). |
| Event retention beyond 7 days? | Keep everything; Intelligent-Tiering, then Glacier IR after 30 days. |
| What if S3 writes keep failing? | Stop reading Jetstream once the waiting-batch limit is hit; resume from the resume point. |
| IPv6-only networking (saves about $3.65/month)? | IPv4 for now; check whether Jetstream, CardyB, the Bluesky API and Cloudflare support IPv6. |
| Redirect candidate threshold? | A final URL with ≥ 1,000 user entries in the ranking state that isn't already checked. Configurable. |
| Publish links without a title? | Yes, with the fallback title. |
| Where the code lives in this repo? | A new top-level `backend/` directory (Go module `github.com/georgemblack/blue-report/backend`) with `backend/infra/` for Terraform. Leave `services/` and `infra/` untouched. |

## 14. Out of scope

- Backfilling v1 data into v2 (planned separately).
- Top sites (removed).
- OpenAI titles (removed).
- Handling deletes.
- Shutting down v1 infrastructure (only after the switch-over, as a separate task).
- Website changes beyond pointing it at v2 data and removing the sites page (done at switch-over).

## 15. Build order

Each phase should end with working, tested code.

1. **Skeleton.** Module, config (env vars), logging, storage interfaces for S3, R2 and DynamoDB with local fakes (local folder for S3 and R2; in-memory or DynamoDB Local), and the Dockerfile.
2. **Ingest + storage.** Reader (with fixture-based tests from v1's `testutil/test-data`), state actor with the post cache and buffer, writer (Parquet, file naming, metadata), finding the resume point, and graceful shutdown. Test: feed fixture events, then check the Parquet output and the resume point.
3. **Ranking.** Ranking state (`Record`, `Top`, `Sweep`, `Merge`) with an injectable clock and thorough unit tests:
   - duplicate removal
   - windows
   - merging
   - sweeping
   - "like, unlike, like counts once"
   - "old like plus recent like counts in the hour window"
4. **Rebuild.** Startup rebuild from Parquet. Test: process events live, then rebuild from the written files, and check the state matches.
5. **Publishing.** Publisher producing `top-links.json` (without titles at first), the readiness gate, and the R2 upload.
6. **Redirects.** Resolver, translations table, the `Merge` path, and paged loading at startup.
7. **Metadata.** Hydrator (titles, thumbnails, recommended posts with caching) and the feed.
8. **Infrastructure.** Terraform, deploy, alarms and budget. Run the section 12 measurements and adjust task size.
9. **Parallel run.** Run alongside v1, compare outputs, fix differences.
10. **Switch-over** (separate, owner-approved). Point the website (`website/src/util/Data.ts`) and feedgen (`feedgen/wrangler.jsonc`, R2 binding) at v2 data, remove the sites page, then retire v1 later.

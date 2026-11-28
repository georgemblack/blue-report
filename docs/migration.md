# Completing the v2 Migration

The v2 engine (`backend/`) is built and running in production alongside v1 (`services/`). This document covers the rest of the move: protecting the parallel run, watching production, comparing v2 with v1, switching over, and shutting v1 down.

Backfilling v1's event history is separate; see [backfill.md](backfill.md). The switch-over doesn't depend on it, but **v1's event files must not be deleted until the backfill is done.**

## Where things stand (2026-11-27)

| | v1 (live site) | v2 (parallel run) |
|---|---|---|
| Rankings file | `data/top-links.json` in R2 bucket `blue-report` | `engine/data/top-links.json` in the same bucket |
| Feeds | `feeds/top-day.xml` and `.json` | `engine/feeds/top-day.xml` and `.json` |
| Website | theblue.report | v2.theblue.report (same worker; picks the data by host) |
| Feed table | `blue-report-feed` | `blue-report-feed-entries` |
| Shared with the other | titles table `blue-report-url-metadata`, `thumbnails/` in R2 | same |

v2 writes the same files as v1, in the same format, under the `engine/` prefix (`PUBLISH_PREFIX`). Switching over is mainly a matter of removing that prefix once v1 has stopped publishing.

## 1. Protect the parallel run

Do these first, so problems show up while the comparison data builds up.

- **Alarm: publishing has stopped.** Add a CloudWatch log metric filter on the engine's `"published top links"` log line, and an alarm when it's missing for more than 10 minutes, sent by email (SNS). This would have caught the two-hour gap on 2026-11-27, when the R2 keys were replaced but the engine still had the old ones.
- **Alarm: the engine keeps restarting.** Alarm on ECS task stops for the `engine` service, or on more than a couple of `"starting engine"` lines an hour.
- **Budget.** An AWS budget with an email alert, set a little above the expected monthly cost (the goal is under $10–15/month).
- **Secrets only take effect on restart.** ECS reads Parameter Store when a task starts. After changing a secret, force a new deployment (`aws ecs update-service --cluster blue-report --service engine --force-new-deployment`).

## 2. Watch production for a week or two

The engine logs a `stats` line every minute and a line for each publish. Things to watch:

- **Lag** (`lag_ms` in `stats`): should stay around a second. Steady growth means the engine can't keep up.
- **Memory** (`heap_mb`): the task has 2 GB, and `GOMEMLIMIT` is about 1.7 GB. Note the weekly peak.
- **Startup rebuild time** (`"rebuilt state from saved events"`): should stay well under a minute once there's a full week of data.
- **Ranking time** (`top_ms`) and **publish time** (`took_ms` on `"published top links"`).
- **Warnings**, especially:
  - `"too many links waiting for the resolver"`
  - `"couldn't update feed"`
  - `"couldn't publish top links"`
  - R2 or DynamoDB errors
- **Browser Run use:** `browser_ms` on `"loaded page with browser run"`. The paid plan includes 10 browser hours a month.
- **The feed:** an entry at most every 12 hours, each with a real title.

After a full week, size the task from the numbers above (CPU, memory, and whether Fargate Spot is worth it), as planned in section 12 of [v2-plan.md](v2-plan.md).

## 3. Compare v2 with v1

v1 publishes hourly and v2 every 5 minutes, so compare snapshots taken at about the same time.

1. **Collect snapshots.** For a week or two, save both files a few times a day, for example with a small script or scheduled job that copies `data/top-links.json` and `engine/data/top-links.json` with a timestamp.
2. **Compare each list (hour, day, week):**
   - How many of v1's top 10 are in v2's top 10, and how far their ranks differ.
   - Links only one side has, and why. Expect some differences:
     - v1 undercounts the hour and day lists (see section 2 of [v2-plan.md](v2-plan.md)).
     - v2 checks popular links for redirects as they become popular. v1 checks them on a 20-minute schedule.
     - v2 skips Apple News links for now.
   - Titles, thumbnails and recommended posts.
3. **Look into specific cases**, such as the `frontofficesports.com` link that had 112 posts and 8 reposts.
4. **Agree what "close enough" means** before switching. That's the owner's call.

## 4. Switch over (owner-approved)

Do these in order, at a quiet time.

1. **Copy v1's feed entries** from `blue-report-feed` into `blue-report-feed-entries`. The items have the same shape; add `expiresAt` (each entry's `timestamp` plus 90 days, in seconds since 1970). This keeps the feed's history, so readers don't see it start over. It's about 180 items, so a one-off script is enough.
2. **Stop v1 publishing:** scale v1's aggregation service to zero, and stop its scheduled runs.
3. **Move v2 onto v1's paths:** set `PUBLISH_PREFIX` to `""` in `infra/engine_service.tf` and apply. v2 then writes `data/top-links.json` and `feeds/top-day.*`, which the website, feedgen (`feedgen.theblue.report`, which reads `data/top-links.json` from R2) and feed readers already use. The feeds' own links become the original addresses automatically.
4. **Check:**
   - theblue.report shows v2's rankings.
   - feedgen still answers.
   - Both feed files are updated by the engine.
5. **Update the website worker** (`website/`, deployed by hand with `pnpm build && pnpm run deploy`):
   - **Remove the top sites page:**
     - `src/pages/sites.astro`
     - `fetchTopSites` and its types in `src/util/Data.ts`
     - the link to it in `src/components/LinkReport.astro` and anywhere else it appears
   - **Remove the v2 special cases:**
     - `V2_HOST` and `V2_API_URL` in `src/util/Data.ts`
     - the host check in `fetchTopLinks`
     - the `robots` logic in `src/layouts/Layout.astro`
     - `src/middleware.ts`, or keep only `Vary: Host` if `v2.theblue.report` stays
   - **Decide whether `v2.theblue.report` stays.** If not, remove its route from `wrangler.jsonc` and its DNS record.

## 5. Shut v1 down

Wait a week or so after the switch, in case something needs v1. Then remove v1's resources from Terraform (`infra/`), one group at a time.

**Remove:**
- **v1's ECS services and tasks:** intake, aggregation and redirect (`service_*.tf`), and their scheduler
- **Infrastructure only v1 uses:**
  - ElastiCache / Valkey (`elasticache.tf`)
  - the SQS queue (`queue.tf`)
  - v1's ECR repository and its images
  - v1's log groups
- **v1's IAM roles and policies** (`iam.tf`), keeping anything the engine or GitHub deploys use
- **v1's Secrets Manager secrets** (`secrets.tf`). Check first that nothing still reads them; the engine uses Parameter Store.
- **DynamoDB tables:**
  - `blue-report-url-translations` and its test copy
  - `blue-report-feed` and its test copy, after step 4.1
  - the test copy of the titles table

**Keep:**
- **`blue-report-url-metadata`**, the titles table, which the engine shares
- **The `blue-report` R2 bucket**, including `thumbnails/`
- **The VPC, cluster and security group**, which the engine shares
- **`blue-report-assets`, v1's events bucket,** until the backfill is finished and checked. After that, decide whether to delete v1's event files or keep them in cold storage.

**Then tidy up the repository:**
- Delete `services/`, along with any v1 build or deploy scripts.
- Update `README.md`, `RANKING.md` and the docs to describe v2 only.
- Remove the "v2:" notes from Terraform comments once there's nothing left to tell apart.

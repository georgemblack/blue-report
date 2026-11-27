# Blue Report v2 back-end

The rebuild described in [docs/v2-plan.md](../docs/v2-plan.md). It's separate from v1 (`services/` and `infra/`) and doesn't import from it.

## Running locally

```bash
LOG_FORMAT=text go run ./cmd/engine
```

This reads live Jetstream and saves events as Parquet files under `local-data/` (git-ignored), which stands in for the S3 bucket. Every 5 minutes it publishes `top-links.json` to `local-data/published/data/`, which stands in for R2. Set `DATA_DIR` and `PUBLISH_DIR` to use other folders. In AWS, `EVENTS_BUCKET` and `PUBLISH_BUCKET` (with R2 credentials) switch to S3 and R2; see `internal/config`. Ctrl-C saves any remaining events before exiting; the next run resumes from the newest saved file.

To look at the files with [DuckDB](https://duckdb.org):

```sql
SELECT make_timestamp(time_us) AS ts, type, did, url
FROM read_parquet('local-data/events/*/*/*.parquet', hive_partitioning = true)
ORDER BY time_us DESC LIMIT 20;
```

## Packages

- `cmd/engine`: the program. Connects the reader, state actor, writer and publisher, and handles shutdown.
- `internal/config`: reads settings from environment variables.
- `internal/event`: interaction types (post, repost, like), the stored event row, and time helpers.
- `internal/eventlog`: event files: Parquet encoding, file names, finding the resume point, and the writer.
- `internal/hash`: turns strings into 64-bit keys for the in-memory maps.
- `internal/publish`: builds `top-links.json` (same shape as v1) from the rankings and uploads it on a timer.
- `internal/postcache`: maps recently active posts to the link they shared, so likes, reposts and quotes can be credited to that link.
- `internal/ranking`: who interacted with each link and when; ranks links over the last hour, day and week.
- `internal/state`: the state actor, the one goroutine that owns the post cache, ranking, translations and write buffer.
- `internal/store`: saves and reads whole files by key. A local folder, or an S3 bucket (also used for R2).
- `internal/stream`: reads Jetstream v2 using Bluesky's official client library, and turns its events into posts, quotes, reposts and likes, dropping everything we don't count.
- `internal/translations`: follows short links to where they redirect.
- `internal/urltools`: URL cleaning, the ignore list and redirect lookup, copied from v1.

## Deploying

Every push to `master` that changes `backend/` runs `.github/workflows/engine.yml`: it runs the tests, builds the image from `Dockerfile`, pushes it to ECR, and redeploys the `engine` ECS service. The infrastructure is in `infra/` (Terraform).

## Testing

```bash
go test ./...
go test -run='^$' -bench=. ./internal/ranking/
```

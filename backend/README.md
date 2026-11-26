# Blue Report v2 back-end

The rebuild described in [docs/v2-plan.md](../docs/v2-plan.md). It's separate from v1 (`services/` and `infra/`) and doesn't import from it.

## Running locally

```bash
LOG_FORMAT=text go run ./cmd/intake
```

This reads live Jetstream and saves events as Parquet files under `local-data/` (git-ignored), which stands in for the S3 bucket. Set `BLUE_REPORT_DATA_DIR` to use another folder. Ctrl-C saves any remaining events before exiting; the next run resumes from the newest saved file.

To look at the files with [DuckDB](https://duckdb.org):

```sql
SELECT make_timestamp(time_us) AS ts, type, did, url
FROM read_parquet('local-data/events/*/*/*.parquet', hive_partitioning = true)
ORDER BY time_us DESC LIMIT 20;
```

## Packages

- `cmd/intake`: the program. Connects the reader, state actor and writer, and handles shutdown.
- `internal/event`: interaction types (post, repost, like), the stored event row, and time helpers.
- `internal/eventlog`: event files: Parquet encoding, file names, finding the resume point, and the writer.
- `internal/hash`: turns strings into 64-bit keys for the in-memory maps.
- `internal/postcache`: maps recently active posts to the link they shared, so likes, reposts and quotes can be credited to that link.
- `internal/ranking`: who interacted with each link and when; ranks links over the last hour, day and week.
- `internal/state`: the state actor, the one goroutine that owns the post cache, ranking, translations and write buffer.
- `internal/store`: saves and reads whole files by key. A local folder for now; S3 comes with the infrastructure.
- `internal/stream`: reads Jetstream v2 using Bluesky's official client library, and turns its events into posts, quotes, reposts and likes, dropping everything we don't count.
- `internal/translations`: follows short links to where they redirect.
- `internal/urltools`: URL cleaning, the ignore list and redirect lookup, copied from v1.

## Testing

```bash
go test ./...
go test -run='^$' -bench=. ./internal/ranking/
```

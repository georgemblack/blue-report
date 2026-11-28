# Backfilling v1 Events into v2

v1 has been saving events since December 2024. v2's events bucket only starts when the engine first ran (2026-11-26). This document plans copying v1's history into the v2 bucket, in v2's format, so all the data is in one place and one format.

This is separate from the switch-over and from shutting v1 down ([migration.md](migration.md)). The one dependency: **v1's event files must not be deleted until this is done and checked.**

The engine doesn't need any of this to run. It only ever reads the last 7 days. The backfill is for keeping the history and for analysis.

## Source and destination

| | v1 (source) | v2 (destination) |
|---|---|---|
| Bucket | `blue-report-assets` | `blue-report-events` |
| Keys | `events/YYYY-MM-DD-HH-MM-SS.json` (when v1 started filling the file) | `events/date=YYYY-MM-DD/hour=HH/<newest time_us>.parquet` |
| Format | JSON Lines, 10,000 events per file | Parquet, zstd compression |
| Size | about 384 GB in about 154,000 files (Nov 2026). Most is in Glacier Instant Retrieval; the last 30 days are in Intelligent-Tiering. | Expect a small fraction of that, since Parquet with zstd compresses repeated URLs and DIDs well |

[event-schema.md](event-schema.md) describes both formats in full.

## Converting each event

| v1 field | v2 column | How |
|---|---|---|
| `type` | `type` | Same numbers (0 post or quote, 1 repost, 2 like) |
| `did` | `did` | Same |
| `post` | `subject_uri` | Same meaning |
| `url` | `url` | Copy as is. It was cleaned with the rules of the time; the engine cleans and filters again when it reads. |
| `timestamp` | `time_us` | Microseconds since 1970. v1's time is when it *processed* the event, usually within a second or two of when it happened. |
| none | `rkey` | Empty. v1 didn't record it, so duplicates in backfilled data can't be detected. |

## Rules the backfill must follow

- **Only events before v2 started.** Skip v1 events at or after the first event in `blue-report-events` (2026-11-26), so the overlap isn't stored twice.
- **Never move the engine's resume point.** The engine resumes from the largest file name in the last 7 days of folders. Backfilled files all hold older events, so they can't be the largest, as long as the rule above holds.
- **Use the engine's own code for writing.** Encode with `eventlog.Encode` and name files with `eventlog.Key`, so the backfilled files are exactly like live ones, including the `schema_version`, `min_time_us` and `max_time_us` metadata.
- **One v2 file per v1 file**, placed by its newest event, like live files. Converting a v1 file always gives the same v2 file, so the job can be stopped and re-run safely. If two v1 files ever end with the same newest microsecond, the second would replace the first; the job should check for that and stop rather than lose data.
- **Respect the storage class.** The events bucket moves files to Intelligent-Tiering from day 0, so nothing extra is needed.

## How to run it

- **A small Go command in `backend/`** (for example `cmd/backfill`): list v1's keys for a date range, then download, convert and upload each file. A few workers running in parallel are enough.
- **Run it inside AWS, in us-west-2,** for example as a one-off ECS task using the engine's cluster and a role that can read `blue-report-assets` and write `blue-report-events`. Downloading 384 GB to a laptop would cost about $35 in AWS data transfer. Inside the region it costs nothing.
- **Do a small range first,** such as one day, and check it (below) before running everything.
- **Expected cost:**
  - reading from Glacier Instant Retrieval: about $0.03 per GB, so about $11
  - requests: a few dollars
  - storing the result: well under $1 a month

## Checking the result

- **Per day,** the number of events in v1 and in the backfilled v2 files should match exactly, since no filtering is done.
- **Spot checks:** a few v1 files compared row by row with their v2 files.
- **A DuckDB query over the backfilled range** (see [event-schema.md](event-schema.md)) should give sensible daily top links.
- **The engine's next startup** should be unaffected: the same resume point, and the same rebuild counts.

## Open questions

- **What happens to v1's event files afterwards?** They could be deleted, saving about $1.50 a month at Glacier Instant Retrieval prices, or moved to Glacier Deep Archive as a backup. That's the owner's call.
- **Should backfilled files be marked?** Possibly with a Parquet metadata key such as `source = v1`, so analysis can tell approximate v1 timestamps from exact v2 ones.

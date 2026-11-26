# Event Storage Schemas

The Blue Report saves every Bluesky event that counts toward a link's ranking: posts that share a link, plus the reposts, likes and quote posts of those posts. This page describes how those events are stored, both in the current system (v1) and in the rebuild (v2), so anyone can read the raw data.

Both systems store the same kind of thing: **one row per interaction with a post that contains a link.** The differences are the file format, the timestamps, and a few extra fields.

---

## v1 (current system)

### Where it lives

| | |
|---|---|
| Bucket | `blue-report-assets` (`blue-report-test` for local and test runs) |
| Key format | `events/YYYY-MM-DD-HH-MM-SS.json` |
| File format | [JSON Lines](https://jsonlines.org/): one JSON object per line |
| Events per file | 10,000 |
| Storage class | Intelligent-Tiering, moved to Glacier Instant Retrieval after 30 days. Never deleted. |

The time in the file name (UTC) is **when intake started filling that file**, not the time of the first or last event in it.

### Fields

| Field | Type | Meaning |
|---|---|---|
| `type` | integer | `0` = post, `1` = repost, `2` = like. **Quote posts are also stored as `0`.** |
| `url` | string | The link, already cleaned (tracking parameters removed, etc.). Cleaned with the rules in place **at the time it was saved**, which may differ from today's rules. |
| `did` | string | The user who made the post, repost or like (e.g. `did:plc:abc123`). |
| `timestamp` | string | **When intake processed the event** (`time.Now()`), in RFC 3339 format. This is not the time the user acted on Bluesky, though it's usually within a second or two. |
| `post` | string | Address (AT URI) of the post **that contains the link**. For a post, that's the post itself. For a repost, like or quote, it's the original post being reposted, liked or quoted. |

### Examples

A post sharing a link:

```json
{"type":0,"url":"https://www.nytimes.com/2026/11/26/example.html","did":"did:plc:alice","timestamp":"2026-11-26T15:00:01.123456789Z","post":"at://did:plc:alice/app.bsky.feed.post/3kabc"}
```

Bob likes Alice's post. The `url` and `post` are Alice's, and the `did` is Bob's:

```json
{"type":2,"url":"https://www.nytimes.com/2026/11/26/example.html","did":"did:plc:bob","timestamp":"2026-11-26T15:04:12.000000000Z","post":"at://did:plc:alice/app.bsky.feed.post/3kabc"}
```

Carol quotes Alice's post. It's stored as a post (`type` 0), with Alice's link and post address:

```json
{"type":0,"url":"https://www.nytimes.com/2026/11/26/example.html","did":"did:plc:carol","timestamp":"2026-11-26T15:06:40.000000000Z","post":"at://did:plc:alice/app.bsky.feed.post/3kabc"}
```

### Things to know when reading v1 data

- **What gets saved:** only English posts (the post lists `en` as a language) that contain a link not on the ignore list. Reposts, likes and quotes are only saved if intake had already seen the original post and still had it cached, which lasts 12 hours after the post's last activity. Interactions on older or uncached posts were never recorded.
- **Quotes and posts look identical.** There's no way to tell them apart in v1 data.
- **There's no record ID**, so there's no reliable way to spot duplicate events or match them to later deletions.
- **Deletions were never recorded:** unlikes, deleted reposts and deleted posts.
- **URLs are cleaned but not "final".** Short links (e.g. `bit.ly/…`) are stored as-is. The aggregation job looks up the destination when it reads the data.

---

## v2 (rebuild)

### Where it lives

| | |
|---|---|
| Bucket | New v2 bucket (name to be decided; nothing is shared with v1) |
| Key format | `events/date=YYYY-MM-DD/hour=HH/<max time_us>.parquet` |
| File format | [Apache Parquet](https://parquet.apache.org/), zstd compression |
| Written | Every few minutes, plus once on shutdown |

- `date` and `hour` are the UTC date and hour of the events in the file.
- The file name is the **newest** `time_us` in the file. The largest file name in the bucket is therefore the point intake resumes from after a restart. There's no separate cursor file.
- If a file's events cross an hour boundary, it goes in the folder for its newest event.

### Columns

| Column | Parquet type | Meaning |
|---|---|---|
| `time_us` | INT64 | **When the event happened on Bluesky**, as Jetstream reports it: microseconds since the Unix epoch, UTC. |
| `type` | INT8 | `0` = post (including quote posts), `1` = repost, `2` = like. Same numbers as v1. |
| `did` | STRING | The user who made the post, repost or like. |
| `rkey` | STRING | The record key of **this user's own** post, repost or like. Together, `did` + `type` + `rkey` identify one Bluesky record. |
| `subject_uri` | STRING | Address (AT URI) of the post **that contains the link**. Same meaning as v1's `post`. |
| `url` | STRING | The link, cleaned (tracking parameters removed, etc.) but **not** resolved through redirects. See "Final URLs" below. |

**File-level metadata** (Parquet key-value metadata, not columns):

| Key | Meaning |
|---|---|
| `schema_version` | `1` for the layout on this page. Increase it if the columns ever change. |
| `min_time_us` | Oldest `time_us` in the file |
| `max_time_us` | Newest `time_us` in the file (matches the file name) |

### Examples

The same three events as above, in v2:

| time_us | type | did | rkey | subject_uri | url |
|---|---|---|---|---|---|
| 1795705201123456 | 0 | did:plc:alice | 3kabc | at://did:plc:alice/app.bsky.feed.post/3kabc | https://www.nytimes.com/2026/11/26/example.html |
| 1795705452000000 | 2 | did:plc:bob | 3klike1 | at://did:plc:alice/app.bsky.feed.post/3kabc | https://www.nytimes.com/2026/11/26/example.html |
| 1795705600000000 | 0 | did:plc:carol | 3kquote | at://did:plc:alice/app.bsky.feed.post/3kabc | https://www.nytimes.com/2026/11/26/example.html |

- **Alice's own post:** its `rkey` matches the end of its `subject_uri`. A quote doesn't, because the quote is Carol's record while the subject is Alice's post.
- **The full address of any row's own record** can be built from these columns: `at://{did}/{collection}/{rkey}`, where the collection is `app.bsky.feed.post` (type 0), `app.bsky.feed.repost` (type 1) or `app.bsky.feed.like` (type 2).

### Things to know when reading v2 data

- **What gets saved:** the same filters as v1. English posts with a link not on the ignore list, plus reposts, likes and quotes of posts intake had cached.
- **Duplicates are possible.** After a restart, intake resumes from the newest saved `time_us`, so a few events can be saved twice. Use `(did, type, rkey)` to remove them. The rankings aren't affected, because each user only counts once per link and type anyway.
- **Deletions are still not recorded.**
- **Final URLs.** `url` is stored before redirect lookup, so improvements to redirect handling apply to old data automatically. To get the final URL, look up `url` in the v2 URL translations table in DynamoDB, and repeat for the result in case of a chain.

### Reading it with DuckDB

```sql
-- All events for one hour, with a readable timestamp
SELECT make_timestamp(time_us) AS ts, type, did, url
FROM read_parquet('s3://<v2-bucket>/events/date=2026-11-26/hour=15/*.parquet');

-- Top links in the last day, counting each user once per link and type
SELECT url,
       count(DISTINCT did) FILTER (type = 0) AS posts,
       count(DISTINCT did) FILTER (type = 1) AS reposts,
       count(DISTINCT did) FILTER (type = 2) AS likes
FROM read_parquet('s3://<v2-bucket>/events/date=*/hour=*/*.parquet', hive_partitioning = true)
WHERE time_us > (epoch_us(now()) - 24 * 3600 * 1000000)
GROUP BY url
ORDER BY 10 * posts + 10 * reposts + likes DESC
LIMIT 10;
```

(The second query ranks raw cleaned URLs. It doesn't apply redirects, so short links and their destinations are counted separately.)

---

## v1 → v2 field mapping

| v1 field | v2 column | Notes |
|---|---|---|
| `type` | `type` | Same values. Neither version tells quotes apart from posts. |
| `url` | `url` | Same meaning: cleaned, not resolved through redirects. |
| `did` | `did` | Same. |
| `timestamp` | `time_us` | v1 is when intake **processed** the event (RFC 3339 string); v2 is when it **happened** (microseconds). To convert v1, use the timestamp's microseconds and note that it's approximate. |
| `post` | `subject_uri` | Same meaning, renamed. |
| none | `rkey` | Not in v1. Converted v1 data will have an empty `rkey`, so duplicates in converted data can't be detected. |

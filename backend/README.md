# Blue Report v2 back-end

The rebuild described in [docs/v2-plan.md](../docs/v2-plan.md). It's separate from v1 (`services/` and `infra/`) and doesn't import from it.

## Packages

- `internal/event`: interaction types (post, repost, like), the stored event row, and time helpers.
- `internal/hash`: turns strings into 64-bit keys for the in-memory maps.
- `internal/jetstream`: parses Jetstream messages into posts, quotes, reposts and likes, dropping everything we don't count.
- `internal/postcache`: maps recently active posts to the link they shared, so likes, reposts and quotes can be credited to that link.
- `internal/ranking`: who interacted with each link and when; ranks links over the last hour, day and week.
- `internal/state`: applies each interaction to the post cache, ranking and write buffer. The state actor will own this.
- `internal/translations`: follows short links to where they redirect.
- `internal/urltools`: URL cleaning, the ignore list and redirect lookup, copied from v1.

## Testing

```bash
go test ./...
go test -run='^$' -bench=. ./internal/ranking/
```

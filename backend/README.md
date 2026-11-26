# Blue Report v2 back-end

The rebuild described in [docs/v2-plan.md](../docs/v2-plan.md). It's separate from v1 (`services/` and `infra/`) and doesn't import from it.

## Packages

- `internal/event`: interaction types (post, repost, like) and time helpers shared by everything else.
- `internal/hash`: turns strings into 64-bit keys for the in-memory maps.
- `internal/postcache`: maps recently active posts to the link they shared, so likes, reposts and quotes can be credited to that link.
- `internal/ranking`: who interacted with each link and when; ranks links over the last hour, day and week.

## Testing

```bash
go test ./...
go test -run='^$' -bench=. ./internal/ranking/
```

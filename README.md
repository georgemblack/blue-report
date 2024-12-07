# Blue Report

## Running Locally

```
docker run -it -d -p 6379:6379 valkey/valkey
go run cmd/aggregate/main.go
go run cmd/intake/main.go
```
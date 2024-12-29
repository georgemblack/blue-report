# Blue Report

## Running Locally

```
podman machine start
podman run -it -d -p 6379:6379 valkey/valkey
export DEBUG=true
go run cmd/aggregate/main.go
go run cmd/intake/main.go
```
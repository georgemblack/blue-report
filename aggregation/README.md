# The Blue Report Aggregation Services

This directory contains two services used to power the site:

1. **Intake service**: Responsible for ingesting data via the Bluesky [Jetstream](https://docs.bsky.app/blog/jetstream) and storing it in S3.
2. **Generate service**: Responsible for generating the website on an hourly internal and publishing it to S3.

## Running Locally

First, ensure you have AWS credentials configured on your machine for read/write access to your S3 buckets.

Then, start a Valkey cache locally:

```
podman machine start
podman run -it -d -p 6379:6379 valkey/valkey
```

Then run one of the given services, such as the intake job:

```
DEBUG=true go run cmd/intake/main.go
```

Or the generate job:

```
DEBUG=true go run cmd/generate/main.go
```
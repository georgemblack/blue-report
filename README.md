# The Blue Report

The Blue Report is a website that shows trending links on Bluesky over the previous 24 hours. Links are scored based on the number of posts, reposts, and likes that reference them.

This repository contains two services used to power the site:

1. **Intake service**: Responsible for ingesting data via the Bluesky [Jetstream](https://docs.bsky.app/blog/jetstream) and storing it in S3.
2. **Generate service**: Responsible for generating the website on an hourly internal and publishing it to S3.

This application is hosted on AWS, using: ECS, S3, ElastiCache, and CloudFront. For more info on the infrastructure, see the following [repository](https://github.com/georgemblack/cloud-infra/tree/master/projects/blue-report).

If you have any questions, please [reach out to me on Bluesky](https://bsky.app/profile/george.black)!

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
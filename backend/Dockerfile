# The engine's container image: one static Go binary on a minimal base.

# Build on the build machine's own architecture and cross-compile for the
# target (arm64 for Fargate), so no slow emulation is needed.
FROM --platform=$BUILDPLATFORM golang:1.27.4 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

# Download dependencies first, so they're cached when only code changes.
COPY go.mod go.sum ./
RUN go mod download

# The git commit being built, logged by the engine at startup.
ARG COMMIT=unknown

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" -o /engine ./cmd/engine

# Distroless has no shell or package manager, just the CA certificates and
# time zone data a Go program needs. Runs as a non-root user.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /engine /engine
ENTRYPOINT ["/engine"]

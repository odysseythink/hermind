## Multi-stage Dockerfile for hermes-agent-go.
##
##  Stage 1 (builder): pulls the full Go toolchain, downloads modules
##   using a module cache mount, builds a statically-linked binary
##   with Version/Commit/BuildDate ldflags.
##  Stage 2 (runtime): alpine:3.20 with ca-certificates. Binary is
##   placed at /usr/local/bin/hermes and runs as non-root.

FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY hermes-agent-go/go.mod hermes-agent-go/go.sum ./hermes-agent-go/
RUN --mount=type=cache,target=/go/pkg/mod go mod download -C hermes-agent-go

COPY hermes-agent-go ./hermes-agent-go

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux \
    go build -C hermes-agent-go \
      -ldflags "-s -w \
        -X main.Version=${VERSION} \
        -X main.Commit=${COMMIT} \
        -X main.BuildDate=${BUILD_DATE}" \
      -o /out/hermes ./cmd/hermes

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S hermes && adduser -S hermes -G hermes

COPY --from=builder /out/hermes /usr/local/bin/hermes

USER hermes
WORKDIR /home/hermes

ENTRYPOINT ["/usr/local/bin/hermes"]
CMD ["--help"]

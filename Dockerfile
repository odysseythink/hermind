## Multi-stage Dockerfile for hermind.
##
##  Stage 1 (builder): pulls the full Go toolchain, downloads modules
##   using a module cache mount, builds a statically-linked binary
##   with Version/Commit/BuildDate ldflags.
##  Stage 2 (runtime): alpine:3.20 with ca-certificates. Binary is
##   placed at /usr/local/bin/hermind and runs as non-root.

FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags "-s -w \
        -X main.Version=${VERSION} \
        -X main.Commit=${COMMIT} \
        -X main.BuildDate=${BUILD_DATE}" \
      -o /out/hermind ./cmd/hermind

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S hermind && adduser -S hermind -G hermind

COPY --from=builder /out/hermind /usr/local/bin/hermind

USER hermind
WORKDIR /home/hermind

ENTRYPOINT ["/usr/local/bin/hermind"]
CMD ["--help"]

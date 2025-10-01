# syntax=docker/dockerfile:1.7-labs

FROM golang:1.24-bookworm AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOFLAGS='-trimpath' go build -mod=mod -ldflags='-s -w' -o /out/server ./cmd/server
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOFLAGS='-trimpath' go build -mod=mod -ldflags='-s -w' -o /out/worker ./cmd/worker

FROM gcr.io/distroless/base-debian12:nonroot AS server
WORKDIR /
COPY --from=builder /out/server /app-server
COPY scripts/entrypoint.sh /entrypoint.sh
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/entrypoint.sh"]

FROM gcr.io/distroless/base-debian12:nonroot AS worker
WORKDIR /
COPY --from=builder /out/worker /app-worker
COPY scripts/entrypoint.sh /entrypoint.sh
USER nonroot:nonroot
ENTRYPOINT ["/entrypoint.sh"]

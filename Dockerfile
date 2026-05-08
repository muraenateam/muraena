# syntax=docker/dockerfile:1

# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.22-bookworm AS builder

WORKDIR /src

# Cache dependency downloads separately from source compilation
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /muraena .


# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /muraena /app/muraena

# Config and certs are bind-mounted at runtime; this just documents the path.
VOLUME ["/app/config"]

# 443 = HTTPS proxy, 80 = HTTP→HTTPS redirect
EXPOSE 80 443

ENTRYPOINT ["/app/muraena"]
CMD ["-config", "/app/config/config.toml"]

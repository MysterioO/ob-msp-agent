# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache dependency downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -trimpath \
    -o /sre-mcp-server \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

# Bring in CA certs (needed for Slack HTTPS) and timezone data.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /sre-mcp-server /sre-mcp-server

# Run as non-root uid.
USER 65534:65534

ENTRYPOINT ["/sre-mcp-server"]

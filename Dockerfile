# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Enable static binary build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Copy source
COPY main.go .

# Initialize module (if not using go.mod)
RUN go mod init chatfilter-discord && \
    go mod tidy

# Build binary
RUN go build -ldflags="-s -w" -o chatfilter-discord

# ---------- Runtime Stage ----------
FROM alpine:3.19

WORKDIR /app

# Install CA certificates (required for HTTPS to Discord)
RUN apk add --no-cache ca-certificates

# Copy compiled binary
COPY --from=builder /app/chatfilter-discord /app/chatfilter-discord

# Create logs mount point
RUN mkdir -p /mnt/logs

# Run as non-root (recommended)
RUN adduser -D appuser
USER appuser

ENTRYPOINT ["/app/chatfilter-discord"]

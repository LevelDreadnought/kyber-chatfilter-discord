# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# enable static binary build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# copy source
COPY main.go .

# initialize module
RUN go mod init chatfilter-discord && \
    go mod tidy

# build binary
RUN go build -ldflags="-s -w" -o chatfilter-discord

# ---------- Runtime Stage ----------
FROM alpine:3.19

WORKDIR /app

# install CA certificates (required for HTTPS calls to Discord)
RUN apk add --no-cache ca-certificates

# copy compiled binary
COPY --from=builder /app/chatfilter-discord /app/chatfilter-discord

# create log and state mount points
RUN mkdir -p /mnt/logs /mnt/state

# create non-root user
RUN adduser -D appuser

# fix permissions
RUN chown -R appuser:appuser /mnt

USER appuser

ENTRYPOINT ["/app/chatfilter-discord"]

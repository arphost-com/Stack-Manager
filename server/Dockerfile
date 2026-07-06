ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /stack-manager-server ./cmd/server

ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}alpine:3.22
RUN apk add --no-cache ca-certificates curl docker-cli docker-cli-compose bash tar gzip git rclone
RUN apk upgrade --no-cache
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup && \
    mkdir -p /home/appuser/.docker && \
    chown -R appuser:appgroup /home/appuser
WORKDIR /app
COPY --from=builder /stack-manager-server .
USER appuser
EXPOSE 8192
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8192/health || exit 1
ENTRYPOINT ["./stack-manager-server"]

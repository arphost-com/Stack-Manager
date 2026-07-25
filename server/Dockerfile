ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}golang:1.25.12-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# APP_GIT_SHA stamps the running build's commit into the binary so the server
# can report its own version (footer/system-info and agent check-in). Wired from
# the same short SHA the web build uses (VITE_GIT_SHA) via compose build args.
ARG APP_GIT_SHA=
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'github.com/arphost-com/Stack-Manager/server/internal/version.GitSHA=${APP_GIT_SHA}'" \
    -o /stack-manager-server ./cmd/server

ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}alpine:3.22
RUN apk add --no-cache ca-certificates curl docker-cli docker-cli-compose bash tar gzip git openssh-client rclone util-linux
RUN apk upgrade --no-cache
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup && \
    mkdir -p /home/appuser/.docker && \
    chown -R appuser:appgroup /home/appuser
WORKDIR /app
COPY --from=builder /stack-manager-server .
USER appuser
EXPOSE 8192
# The controller MUST be able to reach the Docker daemon to be useful, so the
# healthcheck verifies BOTH the HTTP API and daemon connectivity. Previously it
# only checked HTTP, so a controller that had lost its docker.sock connection
# (e.g. after a docker-ce upgrade replaced the socket inode) still reported
# "healthy" while the UI showed everything as down — a silent failure. Now it
# goes unhealthy, surfacing the problem in `docker ps`, fleet view, and uptime
# monitoring. Agent modes skip the docker check (they may not expose :8192).
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD if [ "$APP_MODE" = "agent-callback" ] || [ "$APP_MODE" = "agent-cli" ]; then exit 0; fi; wget --no-verbose --tries=1 --spider http://127.0.0.1:8192/health || exit 1; docker version --format '{{.Server.Version}}' >/dev/null 2>&1 || exit 1
ENTRYPOINT ["./stack-manager-server"]

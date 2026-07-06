ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

ARG BASE_IMAGE_PREFIX=
FROM ${BASE_IMAGE_PREFIX}alpine:3.22
RUN apk add --no-cache nginx openssl && \
    apk upgrade --no-cache && \
    mkdir -p /run/nginx /var/lib/nginx/tmp/client_body /var/log/nginx /usr/share/nginx/html /etc/nginx/ssl /etc/nginx/ssl/acme-webroot && \
    chmod -R 0777 /run/nginx /var/lib/nginx /var/log/nginx /etc/nginx/ssl && \
    chmod -R 0755 /usr/share/nginx/html /etc/nginx/http.d
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/http.d/default.conf
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
# Runtime user is set by docker-compose to match SERVER_USER so bind-mounted
# /etc/nginx/ssl on the host stays owner-consistent with state-init.
EXPOSE 8080 8443
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider --no-check-certificate https://127.0.0.1:8443/ || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]

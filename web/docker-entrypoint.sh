#!/bin/sh
set -eu

# Bootstrap TLS certs for nginx. If /etc/nginx/ssl/fullchain.pem is missing,
# generate a self-signed cert with SANs covering the hostnames the operator
# is likely to hit. The Go server's SSL settings endpoints later manage the
# same files for regeneration and Let's Encrypt handoff.

SSL_DIR=/etc/nginx/ssl
CERT="${SSL_DIR}/fullchain.pem"
KEY="${SSL_DIR}/privkey.pem"
META="${SSL_DIR}/mode"

mkdir -p "${SSL_DIR}" "${SSL_DIR}/acme-webroot"

if [ ! -s "${CERT}" ] || [ ! -s "${KEY}" ]; then
    hostname_short="$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo stack-manager)"
    hostname_fqdn="$(hostname -f 2>/dev/null || echo "${hostname_short}")"
    common_name="${SSL_CN:-${hostname_fqdn}}"

    sans="DNS:${common_name}"
    if [ "${hostname_short}" != "${common_name}" ]; then
        sans="${sans},DNS:${hostname_short}"
    fi
    if [ "${hostname_fqdn}" != "${common_name}" ] && [ "${hostname_fqdn}" != "${hostname_short}" ]; then
        sans="${sans},DNS:${hostname_fqdn}"
    fi
    sans="${sans},DNS:localhost,IP:127.0.0.1"

    if [ -n "${SSL_EXTRA_SANS:-}" ]; then
        old_ifs="${IFS}"
        IFS=','
        for extra in ${SSL_EXTRA_SANS}; do
            extra="$(echo "${extra}" | tr -d ' ')"
            [ -z "${extra}" ] && continue
            case "${extra}" in
                *:*) sans="${sans},${extra}" ;;
                *[!0-9.]*) sans="${sans},DNS:${extra}" ;;
                *) sans="${sans},IP:${extra}" ;;
            esac
        done
        IFS="${old_ifs}"
    fi

    umask 077
    openssl req -x509 -nodes -newkey rsa:2048 \
        -days "${SSL_SELF_SIGNED_DAYS:-3650}" \
        -subj "/CN=${common_name}" \
        -addext "subjectAltName=${sans}" \
        -addext "keyUsage=digitalSignature,keyEncipherment" \
        -addext "extendedKeyUsage=serverAuth" \
        -keyout "${KEY}" -out "${CERT}"
    printf 'self-signed\n' > "${META}"
    echo "docker-entrypoint: generated self-signed TLS cert with SANs ${sans}"
fi

chmod 600 "${KEY}" 2>/dev/null || true
chmod 644 "${CERT}" 2>/dev/null || true

exec "$@"

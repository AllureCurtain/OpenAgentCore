# Gateway and one-time data initialization. With managed HTTPS the gateway also
# serves domain setup. oac talks to Docker through its client library, so this
# image carries no Docker CLI; Core and Web never receive Docker access.
# Caddy and oac are static binaries. oac init and oac gateway run as root so
# they can chown data directories and start Caddy as an unprivileged child.
# scripts/build-core-distribution.sh builds this from a context that also contains
# the oac binary.
FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8
RUN apk add --no-cache ca-certificates
COPY --from=caddy:2.10.2-alpine@sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d /usr/bin/caddy /usr/local/bin/caddy
COPY --chmod=0555 oac /usr/local/bin/oac
ENTRYPOINT []

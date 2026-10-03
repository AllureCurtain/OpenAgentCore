# Deployment

| Path | Contents |
| --- | --- |
| `install.sh` | Host installer published with each release |
| `compose/` | Compose template, port overlays and their tests |
| `distribution/` | Image Dockerfiles |
| `node/` | [Node installer](node/README.md), packaged as `node-install.pyz` |

## Installation

`install.sh` downloads its release's `compose.yaml` and port files, checks them against `compose-sha256sums.txt`, writes `.env`, and starts Compose. Core applies database migrations when it starts. The host needs Linux amd64 and Docker Compose 2.26 or newer. [Configuration](../docs/configuration.md) owns the installation layout and settings.

`oac` is a Go command (`services/core/cmd/oac`) in the Core image and the gateway image. The host copy implements `apply`, `domain`, `core-key` and `rotate-core-key`; `core-key --show` runs `oac-web core-key` in the Web container. Start, stop, logs and removal are `docker compose`. `apply` runs `oac-core check-config` before recreating services. The gateway image runs data initialization as `oac init`, its health check as `oac healthcheck` and, with managed HTTPS, domain setup as `oac gateway`; it contains no Python and is the only image with a Docker client.

## Managed HTTPS

Managed installs add `https.yaml`, which publishes ports 80 and 443 and runs the gateway as `oac gateway`: Caddy as an unprivileged child plus the domain API. The gateway is then the only service with the Docker socket. Web reaches it through `data/domain/api.sock`, and every request carries the Core key. External proxies and hosting platforms omit `https.yaml` and set `OAC_PUBLIC_URL` in `.env`.

## Native daemon installer

`oac-daemon install` installs the daemon and selected Harnesses on a self-hosted machine. The [credential contract](../contracts/agents-api/environment-executor-credentials.md#installation-grant) covers the grant it claims. The release catalog is in the Core image at `/opt/oac/native-installers`; Core serves it from there. Node installation is separate and stays in `node-install.pyz`.

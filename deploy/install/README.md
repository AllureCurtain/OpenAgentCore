# Installation

`deploy/install.sh` is the host installer published with each release. It downloads that release's `compose.yaml` and port files, writes `.env`, and starts Compose. The host needs Linux amd64 and Docker Compose 2.26 or newer.

`oac` is a Go command in the Core image and the gateway image. The host copy implements `apply`, `domain`, `core-key`, `rotate-core-key` and `setup-sandbox`. Start, stop, logs and removal are `docker compose`. `apply` runs `oac-core check-config` before recreating services. The gateway image also runs data initialization and the domain service; it is the only image with a Docker client.

Managed installs publish ports 80 and 443 and start the `domain` service. That service is the only one with the Docker socket. Web reaches it through `data/domain/api.sock`. External proxies omit it and set `OAC_PUBLIC_URL` in `.env`.

## Node installer

These modules stay Python and are packaged as `node-install.pyz`. They install sandbox nodes, not Core.

| Module | Role |
| --- | --- |
| `node_install.py`, `node_spec.py`, `node_generations.py` | Node installer and generation helper |
| `distribution.py` | Checksums, release metadata and Docker image identity |
| `install_display.py`, `node_output.py` | Terminal output |
| `provider_assets.py` | Node provider artifact names |
| `node_payload.py` | Publish a release's node files without replacing bytes already installed |
| `acceptance.py`, `harness_catalog.py` | Opt-in real-model acceptance. `harness_catalog.py` is generated |

## Native daemon installer

`oac-daemon install` installs the daemon and selected Harnesses on a self-hosted machine. The [credential contract](../../contracts/agents-api/environment-executor-credentials.md#installation-grant) covers the grant it claims. The release catalog is in the Core image at `/opt/oac/native-installers`; Core serves it from there. Node installation is separate and stays in `node-install.pyz`.

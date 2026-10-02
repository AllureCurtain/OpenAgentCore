---
title: "Operate your installation"
---

The installation operator owns the Core host, its storage and its availability. Node hosts run their own services; see [Nodes](./nodes.md). Settings are described in the [configuration reference](../configuration.md).

## The oac command

Each installation has its own management command in its directory. It needs neither the bundle nor root:

```sh
~/.oac/core/oac status
```

| Command | What it does |
| --- | --- |
| `oac status` | Shows Compose service status, the public URL and the domain state |
| `oac start` | Starts the services |
| `oac stop` | Stops the services. Data, nodes and sandboxes are kept |
| `oac apply` | Runs `oac-core check-config`, then `docker compose up -d --wait`. A failed check changes no service |
| `oac domain HOSTNAME` | Managed HTTPS: sets the public URL to `https://HOSTNAME`, as **Configure domain and HTTPS** in Web does |
| `oac core-key [--show]` | Prints the Core key path, or the key itself with `--show` |
| `oac rotate-core-key` | Replaces the Core key and restarts Core and Web |
| `oac backup DEST` | Stops the services, archives the installation directory, then starts them |
| `oac uninstall --yes` | Removes the installation, its containers and its images |

For a second installation, use its own command, such as `~/.oac/second/oac status`.

## Service health

Use these observations for different questions:

| Observation | What it establishes |
| --- | --- |
| `oac status`, PostgreSQL health | The database accepts its readiness check |
| Core `/healthz` | Core's process is alive |
| An authenticated API read | The caller's key works for that resource |
| Environment connection | The Runtime transport is connected |
| A finished Turn and its results | The task's recorded outcome |

Service health does not show that a harness or a model works. Use Session, Turn, Items and Usage reads for execution, and Web's **Nodes** page for node connection, readiness and placement. For local diagnosis, use the installation's own Compose file:

```sh
docker compose -f "$HOME/.oac/core/compose.yaml" ps --all
docker compose -f "$HOME/.oac/core/compose.yaml" logs --tail 200 core
```

Don't paste `docker compose config`, `docker inspect` or raw logs into public issue reports.

## Stop and restart

Let active work settle before a planned restart:

```sh
~/.oac/core/oac stop
~/.oac/core/oac start
```

Stopping Core stops no node and no sandbox. Node services, their microVMs and Docker containers keep running; stopping is not a way to reclaim compute. A Core restart does not continue an interrupted native tool call transparently. After reconnecting, query the same Session; don't create a new Session to replay uncertain work. Session event streams are live only; recover through Session, Turn and Items reads.

A Web restart, including one caused by `oac apply`, signs everyone out of the console. Web sign-ins otherwise last 12 hours.

## Core key

Each installation has one administrator credential, the Core key. The installer generates a 64-character random key in `data/secrets/web/core.key`. Read it with `oac core-key --show`; the file is owned by the container user. The Core key:

- signs in to Web. The browser gets an HttpOnly session cookie, never the key;
- authorizes Core API (`/core/v1`) requests sent as `Authorization: Bearer <Core key>`;
- never authorizes the Agents API (`/v1`). Applications use Project API keys, which in turn can't call `/core/v1`.

Keep it private. Web and the `domain` service read `data/secrets/web/core.key`. Core reads only its SHA-256 from `data/secrets/core/core-key-digests.json`. A Core key has at least 32 characters and no whitespace. Web limits failed sign-ins.

### Script the Core API

Run scripts on the Core host against Core's loopback port. This helper reads the key from its file, keeping it off the command line:

```sh
core() {  # core METHOD PATH [JSON body]
  curl -fsS -X "$1" "http://127.0.0.1:8091/core/v1$2" \
    -H @<(printf 'Authorization: Bearer %s\n' "$(~/.oac/core/oac core-key --show)") \
    -H 'Content-Type: application/json' ${3:+-d "$3"}
}
```

| Task | Command |
| --- | --- |
| List Projects | `core GET /projects` |
| Create a Project | `core POST /projects '{"name": "billing-bot"}'` |
| Issue an API key (shown once, as `key`) | `core POST /projects/$PROJECT_ID/keys '{"name": "prod"}'` |
| Revoke a key | `core DELETE /projects/$PROJECT_ID/keys/$KEY_ID` |
| Archive a Project (revokes all keys) | `core POST /projects/$PROJECT_ID/archive` |
| See harnesses and their default models | `core GET /harnesses` |
| Set Codex's default model | `core PUT /harnesses/codex/model-configuration '{"model": "your-model-id", "model_provider": {"protocol": "responses", "base_url": "https://provider.example/v1", "api_key": "sk-..."}}'` |
| Installation facts, including the API base URL | `core GET /installation` |

The [Core administration API](../../contracts/agents-api/admin-api.md) lists every route; errors use the [Core error envelope](../../contracts/agents-api/core-errors.md).

### Rotate the Core key

```sh
~/.oac/core/oac rotate-core-key
```

It writes a new key to `data/secrets/web/core.key`, regenerates `data/secrets/core/core-key-digests.json`, and restarts Core and Web. The old key stops working as soon as Core restarts, and every console session ends: sign in again and update your scripts.

## Projects and API keys

Create Projects and issue keys in Web, on **Projects and keys**, or through the [Core API](#script-the-core-api). How Projects and keys behave is in [Projects own assets](../concepts.md#projects-own-assets).

To rotate an application key:

1. Issue a new key in the same Project.
2. Update the application to use it.
3. **Revoke** the old key.

**Archive** disables every key of a Project and keeps its assets.

Core records which key made each public resource write; the retention of that history is [`core.write_audit_retention`](../configuration.md#settings).

## Back up

Back up these together; a restore needs all of them:

- the PostgreSQL volume `<project>_database`. It holds Projects, key digests, nodes, default models, encrypted credentials and all execution history, including large objects. A logical dump:

  ```sh
  docker compose -f "$HOME/.oac/core/compose.yaml" exec -T database \
    pg_dump -U agents_api agents_api > oac-backup.sql
  ```

- the installation directory, especially `data/`. `data/secrets/core/credential.key` must stay with the database, or stored credentials can't be decrypted.
- each node's state directory on its host, `/var/lib/oac-node/.oac/nodes/<installation-id>/`, with its provider storage: Docker volumes or microsandbox's store. See [when a node host fails](./nodes.md#when-a-node-host-fails) for restoring them.

`oac backup DEST` stops the services, archives the installation directory, and starts them again.

Never prune Docker volumes or delete native harness history to make a retry pass. A deleted Session does not prove that all provider resources were reclaimed.

## Uninstall

```sh
~/.oac/core/oac uninstall --yes
```

It removes the Compose project, its containers and images, and the installation directory. Without `--yes` it changes nothing.

All data goes with it: Projects and API keys, Session history, stored credentials and the Core key. To keep the data, stop the installation with `oac stop` instead, or [back it up](#back-up) first.

Uninstall stops no sandbox: node sandboxes keep running on their nodes, and E2B sandboxes keep running, and billing, at E2B. While Core is still up, archive their Sessions or [reset the deployment](./nodes.md#change-the-sandbox-configuration) and let it complete; the command shows how many sandboxes Core has in use.

Nodes on other hosts keep running. To uninstall them the usual way, remove them in Web first, as in [Remove a node](./nodes.md#remove-a-node). After `oac uninstall` their Core is gone: on each node host, run the node uninstall command with `--force`, using `node-install.pyz` from the [bundle you installed from](#installation-version-policy). `oac uninstall` prints that command with the installation ID.

## Installation version policy

An installation runs one release for its whole life. In-place version upgrades and downgrades are not supported. Nothing migrates data between releases.

To move to a new release, install it into a new, empty directory, with its own database, Core key and nodes, and add nodes from its Web. Keep the old installation, its data and its nodes until their work is finished. Nodes run the program of the console that added them and are never upgraded in place; Core accepts only nodes that speak its own node protocol.

`install.sh` refuses a directory that is not empty. If the first start fails, it deletes the directory it created, and the same command can be run again. An installation that has already started is left in place.

Mutating `oac` commands hold `.oac.lock`. If another command holds it, retry after that command finishes. Never delete `.oac.lock` to get past a busy installation.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `Core installation requires Linux amd64 with Docker access` | Use Linux amd64 and an account with Docker access; root and ordinary users are supported |
| `Installation failed: inspect prerequisites and private deployment files` | A prerequisite failed without its own message, most often Docker: check that `docker info` and `docker compose version` work for this user |
| `Docker Compose 2.26.0 or newer is required …` | Update the Docker Compose plugin |
| `Port N is already in use.` | Another program holds that port. Stop it, or choose another `--web-port`. The installer does not move to a different port |
| `Port 80 is already in use. Free it or rerun with --external-proxy.` | Managed HTTPS needs 80 and 443. Free them, or install with `--external-proxy` |
| `Installation directory is not empty` | Use an empty `--install-dir`, or [uninstall](#uninstall) the existing installation first |
| `configuration check failed; no service was changed` | `.env` has a value Core rejects. The message names the variable and not the value. Fix `.env` and run `oac apply` again |
| `Docker Compose 2.26 or newer is required` | Update the Docker Compose plugin |
| `The services did not start: …` | A new installation's first start failed, and the installer [removed what it created](./install.md#install). Compose's or Core's error is printed above it; fix the cause and run the same command again |
| `Removal did not finish. Left: …` | The installer, cleaning up a failed new installation, or `oac uninstall` could not remove everything. Run the printed commands to remove what is left, or fix the cause and run the same command again |
| `This installation did not finish installing …` | The installer stopped before reporting that the services were running. Rerun the installer command, which [removes what is left](./install.md#install) and installs again, or [uninstall](#uninstall) it |
| `… already in use on this server. Automatic HTTPS cannot run beside another program …` during domain setup | Another program holds port 80 or 443. Stop it, using the printed `ss` command to find it, and retry; automatic HTTPS cannot share [these ports](./install-options.md#ports) |
| `HTTPS verification failed …` during domain setup | DNS points elsewhere, a firewall or NAT blocks inbound ports 80 and 443, or the certificate request failed; see [Configure the domain and HTTPS](./install.md#configure-the-domain-and-https) |
| Web answers 403 `Forbidden` | Open exactly the console address `oac status` prints; a reverse proxy must pass the original Host |
| `/v1` or `/api/v1` answers 404 | Those paths reach Web; route them to Core ([reverse proxy](./install-options.md#https-and-the-reverse-proxy)) |
| Web shows that Core is unavailable (502) | Core is stopped or failing: `oac status`, then Core's log |
| Session creation returns 400 `model_provider_required` | No model provider: set a [default model](../configuration.md#default-models) for the harness, or pass one; self-hosted Sessions always pass their own |
| Add node shows no command | See [Before you add a node](./nodes.md#before-you-add-a-node) |
| A node is not ready | See [node troubleshooting](./nodes.md#troubleshooting) |

## Exposure and network policy

| Listener | Managed ingress (default) | External ingress |
| --- | --- | --- |
| Web and the API | The `gateway` publishes `OAC_WEB_PORT` (8080) on `OAC_HOST`, plus [80 and 443](./install-options.md#ports) | The gateway publishes `OAC_WEB_PORT` on `OAC_HOST`. Your proxy should use `127.0.0.1` |
| Core admin API | `127.0.0.1:8091`. The gateway routes `/v1` and `/api/v1` | `127.0.0.1:8091`. The gateway routes `/v1` and `/api/v1` |
| PostgreSQL | No published port | No published port |

Web signs administrators in with the Core key, checks the origin of every request, and forwards signed-in `/core/v1` requests to Core with the Core key, which stays on the server. It answers 404 on `/v1` and `/api/v1` whatever credential a request carries, serves only the non-secret node payload at `/node-install/`, and has no Docker or KVM access. Machine routes under `/api/v1` use their own enrollment and connection credentials. With managed ingress, the `domain` service applies domain changes through the Docker socket; Web reaches it only over a private Unix socket, and it checks the Core key on every request.

Sandboxes are the isolation boundary ([Runtime and outer isolation](../concepts.md#runtime-and-outer-isolation)). Docker sandboxes share the node's kernel, and a Docker node is [root-equivalent](./nodes.md#what-the-installer-sets-up) on its host; microsandbox gives each sandbox a microVM with an explicit [network policy](./nodes.md#what-the-installer-sets-up). Core itself has no Docker socket or KVM access.

---
title: "Install Core and Web"
---

One command installs Core, the Web console and PostgreSQL on a Linux host. Web is the administrator console: you sign in with the Core key, give the installation a domain, set a default model and issue Project API keys. Applications then call Core's API with those keys, and their Sessions run in sandboxes on nodes you add, or on E2B.

1. [Check the prerequisites](#prerequisites).
2. [Run the installer](#install).
3. [Sign in to Web](#sign-in-to-web).
4. [Configure the domain and HTTPS](#configure-the-domain-and-https).
5. [Set a default model](#set-a-default-model).
6. [Issue a Project API key](#issue-a-project-api-key).
7. [Add sandbox capacity](#add-sandbox-capacity).

This page follows the default path. Every flag, existing reverse proxies and offline hosts are in [installation options](./install-options.md).

## Prerequisites

- Linux amd64 and curl.
- Docker Engine with Docker Compose 2.26.0 or newer (`docker compose version`).
- An account that can run `docker` and write to its home directory. Ordinary users and root both work; the installer never calls sudo.
- Free port 8080 for Web, and free ports 80 and 443 for managed HTTPS. Core's admin API uses `127.0.0.1:8091`. See [ports](./install-options.md#ports). Docker must be able to publish them; the installer does not change host policy.
- A DNS hostname that points to this host, before you connect applications, nodes, E2B or self-hosted machines. You can install and sign in first.

The Core host needs no KVM; nodes that run microsandbox do.

## Install

```sh
curl -fsSL https://github.com/MiniMax-AI/OpenAgentCore/releases/latest/download/install.sh | bash
```

If DNS already points to this host, pass the address to set up HTTPS during installation instead of in step 4:

```sh
curl -fsSL https://github.com/MiniMax-AI/OpenAgentCore/releases/latest/download/install.sh | bash -s -- --public-url https://core.example
```

The script downloads that release's Compose files, checks their SHA-256, and:

1. checks Linux amd64, Docker Compose 2.26 or newer, and that the ports it will publish are free;
2. creates the [installation directory](../configuration.md#installation-directory), `~/.oac/core`, writes `.env`, and copies the `oac` command out of the Core image;
3. starts the services with Docker Compose. The gateway serves Web on all IPv4 interfaces at port 8080. Core's admin API stays on `127.0.0.1:8091`. PostgreSQL is not published;
4. selects the microsandbox sandbox backend at the Standard size. It adds no node.

With `--public-url https://HOSTNAME` on a managed install, it then runs `oac domain`. DNS and ports 80 and 443 must already reach this host.

It creates no Project or key and makes no model request. It ends by printing the console address and how to read the Core key.

If installation fails before the services become healthy, the installer removes the directory it created. Fix the reported cause and rerun the same command. Once the services have started, a later failure keeps the installation and its data. A new release is a new directory; see [version policy](./operations.md#installation-version-policy).

For insufficient space or quota, free space on the filesystem named by the error. Image-loading failures can also require space in Docker's storage, which may be on a different filesystem.

## Sign in to Web

1. Open the console address the installer printed, such as `http://SERVER_IP:8080`, or your public URL if you passed one. Behind NAT, use the IP address your browser reaches. Until a domain is set, Web accepts IP addresses only, not host names.
2. Sign in with the [Core key](./operations.md#core-key), the installation's administrator credential. Web has no user accounts.

   ```sh
   ~/.oac/core/oac core-key --show
   ```

## Configure the domain and HTTPS

Applications, nodes and sandboxes reach Core at one HTTPS address, the public URL. The HTTP address on port 8080 keeps serving Web and the API.

1. Point the hostname's A/AAAA records to this host, allow inbound ports 80 and 443 from the internet, and keep other programs off [those ports](./install-options.md#ports).
2. In Web, open **System**, choose **Configure domain and HTTPS**, enter the hostname, such as `core.example.com`, and choose **Apply**.

The installation checks DNS, requests a certificate, and checks that `https://HOSTNAME` reaches this installation before switching Core and Web to it. Then open the HTTPS address and sign in again. Certificates renew automatically. If DNS or the certificate fails, the previous address stays in use: correct the reported problem and retry the same hostname. `data/domain/status.json` shows a failed attempt and how long to wait before the next one.

The same operation from a terminal:

```sh
~/.oac/core/oac domain core.example.com
```

To change the address later, see [changing the public URL](../configuration.md#changing-the-public-url).

## Set a default model

Core-hosted Sessions without their own model provider use their harness's default model. On **System**, under **Default model configuration**, find the harness marked **Default** (Codex unless you changed `core.default_harness`) and choose **Set**. Enter the model ID, the protocol, and the provider's base URL and API key. MiniMax Code also needs the context window and max output tokens. See [default models](../configuration.md#default-models).

## Issue a Project API key

1. On **Projects and keys**, choose **Create project**, then **Issue key**. The dialog shows the key once: copy it and keep it safe. Its **How to call** card shows the API base URL and sample requests.
2. Give the key and the API base URL to the application developer. They continue with the [quickstart](./quickstart.md).

Web's **Overview** tracks these steps in a **Getting started** checklist.

## Add sandbox capacity

Sessions need somewhere to run:

- **Nodes** run the microsandbox backend the installer selected: [add a node](./nodes.md) from Web's **Nodes** page. Docker and E2B are chosen later with [Reset deployment](./nodes.md#change-the-sandbox-configuration).

Day-to-day operation, backups and upgrades are in [Operations](./operations.md).

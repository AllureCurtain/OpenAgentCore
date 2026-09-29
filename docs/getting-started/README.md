# OpenAgentCore documentation

Start with the path that matches your role. These pages are the canonical sources
for the documentation website; generated site pages are not a second manual.

## First installation and first task

1. [Install Core and Web](install.md), then sign in with the Core key.
2. [Add managed execution capacity](nodes.md) or
   [connect your own Runtime](self-hosted.md).
3. Issue a Project API key and follow the [first Session walkthrough](quickstart.md).
4. Continue with the [user guide](../user-guide.md) for capabilities, files and recovery.

## Operate a deployment

| Guide | Use it for |
| --- | --- |
| [Installation](install.md) | Host prerequisites, release bundles, HTTPS and first sign-in |
| [Configuration](../configuration.md) | Installer configuration and database-owned settings |
| [Nodes](nodes.md) | Managed capacity, readiness and removal |
| [Self-hosted execution](self-hosted.md) | Connecting a user-owned machine to a Session |
| [Native daemon](../self-hosted-native.md) | Platform support and local installation lifecycle |
| [Operations](operations.md) | Service lifecycle, backups, keys, repair and troubleshooting |
| [Administrator console](../web/README.md) | Monitoring and managing Core through Web |

## Build an application

The [user guide](../user-guide.md) connects common tasks to their detailed contracts.
The [API index](../api/README.md) identifies each caller and credential. Use the
[coverage record](../../contracts/agents-api/README.md) to check operation support
and Harness differences before relying on an optional feature.

## Extend Core

Start with the [developer guide](../development.md). It links the repository map,
required checks and extension points for Harness adapters, Runtime implementations
and Sandbox Providers. Architecture rules live in
[CONTRIBUTING.md](../../CONTRIBUTING.md); distribution and release procedures live in
[the maintainer guide](../maintainers.md).

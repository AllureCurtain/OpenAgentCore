# OpenAgentCore development

Read [CONTRIBUTING.md](CONTRIBUTING.md) before changing code. It owns repository
boundaries, architecture, workflow, required checks and independent blind review.
Use [docs/development.md](docs/development.md) for checkout and component guidance.

- Keep Core, Runtime, Harness and Sandbox Provider ownership separate through the
  shared protocols; see [decoupling](CONTRIBUTING.md#decoupling-principle).
- Identify every API route's caller and credential in [the API index](docs/api/README.md).
- Run `make sqlc-generate` after query changes and `make openapi` after handler
  changes; review and commit the generated contracts with their source changes.
- Run `make check` before reporting completion.
- Update the canonical documentation in the same branch when changing a rule,
  boundary, workflow or generated contract. Keep this file a short index.

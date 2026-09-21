# Parsar Core development

Read [CONTRIBUTING.md](CONTRIBUTING.md) before changing code. Work in an isolated
Git worktree and submit a PR. Keep product business code in the Parsar repository.

Preserve the pinned public Agent API and documented extensions. Do not add product
database dependencies or bypass Core execution ownership. Update architecture and
generated contracts with changes. Run `make check` before reporting completion.

After implementation and verification, request an independent blind review with
only requirements, acceptance criteria, boundaries, repository path and baseline.
Fix in-scope blockers and review again. Never use `codex exec` for this review.

Documentation and code comments are English; user-facing copy may be bilingual.

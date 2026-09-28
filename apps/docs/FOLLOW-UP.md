# Documentation integration status

The current repository implements durable reset, retained deployment generations,
same-team E2B online changes and Docker/microsandbox online size and Runtime targets
(design 50, including #178). Node target preparation, qualified serving generations,
retained ownership and generation GC coexist; target changes require neither an
execution drain nor node reenrollment. Core error details, configuration validators
and Session diagnostics are also present in the current source.

These are repository implementation facts, not evidence that a release was published
or deployed. Use the merged source revision and its matching bundle for an installed
release. A proposal or an unmerged implementation must stay labelled as such until
integration and the requested acceptance have completed.

| Remaining requirement | Documentation boundary |
| --- | --- |
| Unified Runtime capability preparation | Future design: keep resource allocation, Environment creation and reclamation separate from Runtime prepare/execute/recover. Closing an executor must not destroy its Environment, delete its workspace or release its allocation. Define a platform-neutral protocol with Linux as the first implementation; do not claim it exists today. |
| Agent metrics backend aggregation (BE8) | Still a proposal. Current browser fan-out is not a deployment-wide backend aggregation service; preserve coverage and missing-data limits. |
| Release qualification and publication | Reconcile the final merged source, generated references and reading notes, and record the requested checks. Source implementation alone does not establish published-release or live-deployment readiness. |

English guides are rendered from repository sources. Chinese pages are explicitly
labelled reading notes, not complete translations. Review them when the canonical
procedures change. API tag pages use the default-language fallback. Remote deployment
and preview replacement are outside this documentation reconciliation.

The source Web guide still carries a pre-rename screenshot. This site copies the
approved current Web onboarding Monitor image instead, with its own source hash;
refresh that mapping when the canonical guide image is updated.

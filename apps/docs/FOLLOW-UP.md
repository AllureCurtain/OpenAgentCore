# Feature integration follow-up

This candidate documents durable reset, retained Core generations and verified
same-team E2B online changes at source `b9b13b186089b317db271ab83b10f86b48168f66`.
Node changes still require zero held resources until the node protocol is complete.
The Core error-envelope foundation is included; later configuration validators and
diagnostic endpoints are not. Do not replace current procedures with planned behavior
before those features land.

| Feature dependency | Site pages to reconcile | Source and verification work |
| --- | --- | --- |
| Final Web generation controls (design 50 PR-W) | configure, hosted-providers, troubleshooting, execution-model | Reconcile the actual Web workflow after node online changes are qualified; keep target preparation separate from serving readiness. |
| Node update protocol (design 50 PR-N) | hosted-providers, troubleshooting, API reference/machine | Explain the implemented update command, old-node handling and ownership preservation. Do not claim an update command now. |
| Later configuration/error changes (design 40 and remaining installer work) | install, configure, troubleshooting, quickstart | Regenerate current configuration fields and revise diagnostics against merged code; keep secrets out of examples. |
| Final feature assembly | every Chinese reading note, execution-model diagram, all generated references | Review translations against regenerated English, verify source provenance and links, run contract/route/copy/fact checks, typecheck/build, local browser review and the repository gate. |

English guides are rendered from repository sources. Chinese pages are explicitly
labelled reading notes, not complete translations; expand them only against the final
source procedures. API tag pages use the default-language fallback. Release publication,
remote deployment and preview replacement are outside this reconciliation task.

The source Web guide still carries a pre-rename screenshot. This site copies the
approved current Web onboarding Monitor image instead, with its own source hash;
refresh that mapping when the canonical guide image is updated.

# Maintainers and advanced deployments

This page is for people who build and publish OpenAgentCore, or run Core without the
installer. To install Core and Web, use the
[installation guide](getting-started/install.md) instead.

## Build a distribution

Release builders need the repository's full Linux toolchain and Docker. Build from
clean, committed source, with the pinned Codex platform package and a matching MiniMax
companion prepared through the existing Runtime build instructions:

```sh
export AGENTS_RUNTIME_CODEX_PACKAGE=/absolute/path/to/codex-linux-package
export MCODE_HARNESS_BUILD_DIR=/absolute/path/to/mcode-harness-artifact
export CORE_DISTRIBUTION_RELEASE_BASE_URL=https://downloads.example/releases/COMMIT
make build-core-distribution
```

The builder reuses the existing Core, Runtime, SDK and Web build scripts. It records
the commit, immutable image identities, microsandbox binary hashes and the actual
Runtime OCI manifest digest. Output goes to `~/.oac/build/core-distribution/`; it is
not published anywhere automatically. Qualify the exact bundle before distributing it.
See the [contributor guide](../CONTRIBUTING.md).

The release base must serve the generated asset file names over HTTPS. Set
`CORE_DISTRIBUTION_OFFLINE=1` to also produce the offline bundle; a build without a
release base must select offline mode. Nodes always download their files from the
console that generated their command, whatever release base the build recorded.

A bundle carries a fixed set of docs (the build lists them). Links between them stay
relative; every other relative link is rewritten to the same file on GitHub at the
bundle's commit, and the build fails if a link or anchor does not resolve.

## Produce and qualify a release

The `core-release` GitHub Actions workflow builds production assets from a full
committed source SHA with the pinned Runtime builders. Acceptance credentials and
private test certificate authorities must never enter its inputs. Run it from the
repository's Actions page, or:

```sh
revision=$(git rev-parse HEAD)
gh workflow run core-release --repo MiniMax-AI/parsar-core --ref main \
  -f ref="$revision" -f offline=true -f draft_release=true
```

The workflow uploads the matched files as an Actions artifact and, with
`draft_release`, creates a draft Release tagged `build-<full SHA>`. The manifest records
the same tag in every asset URL. Don't mix files across releases or resolve components
through `latest`.

Download the draft assets with repository access, verify their checksums, and qualify a
fresh installation plus the node and self-hosted connection paths before publishing the
draft. A workflow build alone is not live acceptance. Publish exactly the tested assets;
never rebuild or replace files under the same release identity. Publishing a Release
does not change the repository's visibility.

The workflow defaults to offline output, including tag-triggered builds. It remains
a candidate builder; it does not publish merely because a build succeeded.

For the current installation batch, the promotion controller runs on the existing
operator host with gh authentication and SSH access to the qualification host:

```sh
python3 scripts/promote-qualified-release.py \
  --source FULL_CANDIDATE_SOURCE_SHA \
  --assets /absolute/path/to/flat-candidate-assets \
  --qualification-package /absolute/path/to/reviewed-private-package \
  --qualification-manifest-sha256 FULL_REVIEWED_MANIFEST_SHA256 \
  --merge-wait-seconds 86400 \
  --state /absolute/path/to/new-promotion-evidence \
  --host zju_a100_2 --remote-root /absolute/path/to/existing-isolated-acceptance-root \
  --promotion-commit FULL_REVIEWED_TOOLING_COMMIT_SHA
```

This command is an operational publication command, not a dry run. It does not
build, merge PRs or create host accounts. Its directly supervised qualification
scripts perform the reviewed fresh installation and real execution actions. Before
starting, independently review the private package and record its manifest hash.
The package is separate from the candidate. `ready=true` describes the generic
adapter protocol only; it does not establish package review, host readiness or
successful live qualification.

Stage only the build's flat files, including thin/offline archives, both checksum
files and every versioned Runtime asset; exclude the extracted bundle directory.
Before staging files from the builder, compare their full inventory byte total plus
64 MiB of working reserve with actual local free space. The controller uses only
that existing single asset set and hashes subsequent GitHub downloads as streams;
it checks the additional reserve and records both byte counts. It never removes
user files to make room.
The remote parent directory must already exist within the batch's authorized scope.
The controller creates one new run directory, copies verified draft assets there,
and invokes the reviewed adapter over SSH. For this batch, zju coordinates managed stages and reaches mx2 through existing
SSH for the fresh Core/node stages; no new service or credentials are required.
GitHub credentials stay on the operator host; nodes continue to download from their console.

The adapter protocol is documented in `scripts/qualify-core-release.py`: one JSON
request as the first stdin line followed by `ping` heartbeat lines, one bound JSON
result on stdout, redacted diagnostics on stderr,
and a nonzero exit for any failed or skipped required check. The six current-batch
checks cover fresh installation, current lifecycle, managed native execution,
current generations, node Runtime and diagnostics/observations. Their actual
commands must operate on freshly extracted supplied assets and respect the agreed
resource ownership. The controller verifies remote asset hashes before and after
execution; it has no pass-file option. Execution commands come only from the
maintainer-pinned package, never from candidate metadata or a stage result.

The package root contains `manifest.json` and exactly its enumerated regular files;
symlinks, extra files and changed bytes are rejected. Manifest version 1 has `files`
(relative path to SHA256/size), `configuration` (reviewed resource bounds and private
file paths, no credential values), and six ordered `stages`. Each stage has `name`,
absolute `python` interpreter path, package-relative `.py` `script`, structured
string `args`, and `timeout_seconds` (1–14400). No shell command or candidate-driven
substitution is used. The explicit SHA256 covers the exact manifest bytes. Preserve
the reviewed package together with all evidence.

The supervisor gives each child the unchanged controller identity and inventory,
`qualification_package`, `qualification_manifest_sha256`, `package_configuration`,
`owned_resources`, and `previous_stage_result`. The latter two come from the actual
preceding child, initially empty/null. Private wrappers adapt host-specific harness
interfaces and must verify their detailed subchecks before returning
`status: passed`, exact `checks: {stage-name: passed}`, the five identity fields
(source/tree/run_id/inventory_sha256/adapter_sha256), and `owned_resources`.
The run ID is a canonical UUID string. `qualification_control.py` is part of the
reviewed tooling bytes and the pinned private package. The sender holds the same
SSH stdin open, sends heartbeats every five seconds and never reloads a pass file.
The receiver stops later work on EOF or a thirty-second heartbeat timeout. Signal
handlers enter the same cleanup path. Each stage or remote worker has a foreground
process group with a waiting owner outside it. The owner cleans the group on every
exit, including success and nonzero exit, so an inner timeout or SIGKILL cannot
leave same-group foreground descendants running. Foreground commands inherit the
group; only recorded background resources may create separate sessions and remain
running. Nested SSH workers use this protocol too; closing a local SSH process alone is insufficient. A write already
sent may still have an unknown result; retain its intent and resources without
replay or a rollback claim. Child stdout/stderr remain private files;
nonzero exit, timeout, changed bytes or mismatched identity stops the sequence.

After all six stages pass, the same controller waits up to `--merge-wait-seconds`
for main to reach the exact reviewed promotion tree. While main is an ancestor of
that reviewed commit it continues waiting; divergent changes stop publication.
Do not restart merely because the merge is pending. Cancellation or timeout retains
resources and evidence but cannot resume from a recorded pass. The command never
merges the batch itself.

This finite command publishes automatically when all checks pass and the batch is
landed. Candidate source and tag remain
`48ed8158e134207d15cdd14ae0a30e10f070eb5c` / `build-48ed8158e134207d15cdd14ae0a30e10f070eb5c`.
The reviewed tooling commit can add only the enumerated release files, the
node-generation protocol wording correction and test registration; main must have that commit's tree and include the candidate source.
Later release documentation is not retroactively inserted into the tested bundle.
Any product change blocks publication of this candidate instead of silently
publishing an obsolete product or relabeling old bytes.

On failure, retain the new evidence and resources and reconcile the Release state.
Never overwrite assets or use saved check results to resume publication. A release
already published is refused before any new acceptance; a failed final verification
needs investigation, not automatic deletion or replacement. Run only one controller
for this batch. This path does not require unattended cloud scheduling for future
pushes, public repository visibility or historical-installation upgrades.

## Run Core without the installer

These paths give you Core alone, without Web, the `oac` command or `config.json`.
They are for development, testing and operators who manage Core's process themselves.
They are not an installation path for new users.

- [Standalone Core archive](../services/agents-api/RELEASE.md)
  (`make build-agents-api-release`): Core, its migrator and operator commands for your
  own PostgreSQL.
- [Standalone container](../services/agents-api/CONTAINER.md)
  (`make docker-build-agents-api`): the same in a Linux container image.
- [Service guide](../services/agents-api/README.md): building and running Core from
  source.

Core reads only its environment; the
[configuration appendix](configuration.md#appendix-core-environment-without-the-installer)
lists the variables. The Docker-hosted variant of the standalone archive is retired: it
could not describe a complete Runtime release by itself. Docker-hosted deployments use
the Core distribution and its installer, whose manifest carries the complete release.

Immediately after the final asset download, the controller repeats the exact
main/tree, tag, draft and Release ID checks. Publication uses the verified Release
ID through the existing authenticated API, so a replacement tag lookup cannot
select another release. Published bytes and final release/tag identity are checked
again afterward.

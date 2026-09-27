# Node generation protocol

This is the internal, authenticated Core-to-node provider protocol. It does not
change the pinned public Agent API. Version 1 retains its existing envelope;
version 2 carries exact deployment-generation routing and sparse local provider
observations. A version 1 connection never receives version 2 fields.

## Bounded control

A version 2 hello or heartbeat contains at most eight generation observations.
Each names a positive signed-64-bit generation, its lowercase SHA-256 specification
digest, a `ready`, `preparing` or `failed` state, and an optional fixed diagnostic.
The target and serving generation take priority; other records rotate fairly.
Eight bounds one message, not the number of generations a node may retain.
Omitted observations never authorize deletion or imply absence.

Welcome and heartbeat acknowledgements carry the Core-owned target generation,
its specification digest, and an explicitly nullable durable serving generation.
Target preparation is independent of the readiness of a retained serving provider.
A provider request carries its exact immutable `deployment_generation` separately
from the allocation's compute generation.

Retention exchanges use an independent bounded control path. A request identifies
at most eight local `(generation, specification_digest)` references, a UUID request
ID, a monotonically increasing sequence, the current connection UUID and owner
epoch. Its acknowledgement must match the complete pending request, including
entry order and identity, and must explicitly supply a boolean `keep` for every
entry. Only one pending exchange exists per connection. A disconnect discards it;
an unsolicited, replayed, stale, partial or mixed acknowledgement deletes nothing.

Control envelopes are at most 32 KiB. Their member names are exact and unique;
unknown members, case aliases, duplicate members and unexpected nulls are rejected.
The nullable serving pin and host measurements preserve unknown values. Provider
request/response frames retain the existing global size bound; control traffic
does not increase that bound or consume the provider-operation queue.

## Local retention and helper lifetime

A Core drop grant is necessary but insufficient for collection. Queued and running
provider calls, preparation, the local target and serving pin retain references.
Collection rechecks those references and refuses an already canceled connection.
A canceled provider caller does not establish that its native helper has stopped:
the parent counts that helper through its actual `Wait` completion.

Every generation also owns a permanent private lease file at
`state/node/generations/<generation>.lease`. Before starting a native helper, the
node acquires its shared flock and checks the generation's dropped tombstone under
that lock. The helper inherits the descriptor. The node closes its own descriptor
only after `Wait`; it never explicitly unlocks the shared open-file description.
Thus caller cancellation or node exit does not release a live helper's reference.
New native helpers set this descriptor close-on-exec before calling the SDK so VM
and daemon descendants do not inherit a helper reference.

Collection acquires the exclusive nonblocking flock before inspecting references,
removing shared images or release files, or publishing the dropped tombstone. It
keeps the lock through those changes. Lock files belong to stable node state and
are never removed or atomically replaced during collection. Symlinks, multiply
linked files, foreign ownership, unsafe permissions and replaced lock paths are
refused. A dropped generation cannot be prepared or used again; a future rollback
would require a new generation and a separate policy.

An immutable older native helper may pass its inherited descriptor to descendants.
That conservatively retains its generation's bytes until those descriptors close;
collection must not kill historical VMs or a host daemon merely to reclaim disk.
A helper's exit is local file-lifetime evidence, not proof that a remote mutation
or an uncertain provider receipt has been released. Core's durable allocation and
placement retention requirements remain independent.

## Matched fresh installation

The host program release and Core's selected Runtime release are independent.
A fresh node gets its executable and private preparer from the current console
release. It reads the exact Runtime source, image identities and native runtime /
firmware digests from the authenticated Core configuration. If that Runtime is
older, the console must still serve its immutable `releases/<source>/` manifest,
checksums and allowlisted artifacts. Runtime helper, firmware, seccomp and image
bytes come from that selected release; the enrolled specification records it.
Artifact URLs are pinned to their verified manifest source even if the console's
current release changes during download.

A missing retained release refuses installation rather than substituting the
current Runtime. A local bundle that contains only a different Runtime also
refuses with guidance to use the console origin retaining the selected release.
These refusals occur before writing the installation identity, importing the
Runtime, registering the node or starting its service.

## Restart recovery

Missing retained Runtime bytes do not switch a pinned placement to the current
Runtime. The node retains the original generation and specification digest as
unready state, and asks Core's authenticated configuration endpoint for that exact
kept generation before recovery. Missing seccomp bytes may leave an unready
provider placeholder; missing image or native artifacts discovered by a provider
probe queue repair without advertising readiness.

Preparation and repair are serialized. Target and serving generations take
priority, with bounded progress over other retained generations. Each attempt has
a 30-minute deadline; failures back off for 1, 2, 5, 10 and then at most 30 minutes.
No connection-established deployment facts means no preparation starts. Repair
preserves existing configurations and paths, verifies the selected release and all
existing sibling checksums, and downloads only absent immutable files. Conflicting
bytes or a different retained specification refuse repair. A missing complete
provider configuration remains a refusal rather than a guessed reconstruction.

Repair takes the same exclusive generation lease and installation lock used by
collection. A live helper or concurrent collector therefore retains ownership;
repair retries later without replacing in-use files. Once bytes are restored, the
node still runs the actual provider readiness probe. File presence and executable
capability alone never establish serving readiness.

## Interrupted local collection

Before native or release deletion, the node persists a private collection journal
bound to its installation, generation and specification digest. Restart loads an
unfinished journal only as a retention-exchange candidate: it cannot prepare,
probe, acquire or advertise that generation. A fresh correlated Core drop grant
is required to resume; the journal itself never authorizes deletion.

A successful complete native image inventory distinguishes absence from a CLI or
daemon failure. Failed queries, malformed inventories, native in-use refusals and
unknown ownership retain the local bytes. Native completion is persisted before
release-file cleanup, so a retry can finish a partly removed release without
executing an already removed helper. Shared images/releases remain until their
last local reference. The final digest-bound dropped marker follows durable file
cleanup and permanently prevents re-adoption. The small immutable configuration
and ownership journals remain as local identity records.

New preparation records its exact identity before downloads and records import
start before invoking the native importer. An interrupted download can repair only
missing bytes at the original paths. If collection precedes any import attempt,
the preparation journal proves that this generation has no imported native image.
An older or interrupted generation whose native executable is missing and whose
import may have started remains conservatively retained; missing files do not
prove native absence. Receipt/store history and a legacy-unfenced generation are
never erased using an empty native inventory.

## Identity-preserving program updates

Run the checksum-verified `node-install.pyz --update` as the original installation
owner, with the same installation ID and Core URL and the selected release's
`--source-url` or `--bundle`. Do not supply a new enrollment token. The installer
first authenticates the retained node through a read-only configuration request.
It verifies and stages the new node executable and matching private preparer,
checks the executable's `protocol-version`, and persists a resume journal before
stopping the fixed service unit. This capability check is not Runtime readiness.

The journal pins the staged file hashes. Rerunning an interrupted update resumes
those files even if the console has since published another release. Atomic file
replacement changes the node program and preparer; the enrolled provider,
credential, Runtime identity, resource files and retained generation state remain.
The previous executable is preserved separately from Runtime releases. Core must
confirm connection and provider readiness before the journal becomes complete.
An unavailable Core or failed restart leaves recoverable files and journal; rerun
the same command after resolving the reported error.

The existing units use `KillMode=process`: update stops and restarts their main
node process without terminating native VMs or helper processes. System-mode
updates reuse the existing recorded service account; all user-owned files and
programs are handled after dropping privileges. Root controls only the fixed
root-owned service unit and host installation lock.

A v1-to-v2 update cannot retrospectively add a file lock to an already-running v1
helper. Before stopping v1, it durably marks precisely the original enrolled
generation `legacy-unfenced`. Automatic collection retains that generation's local
payload and store across interruptions and node restarts. Unknown original
generation identity refuses the update. Neither an empty allocation list nor PID
absence clears this marker. It is a single-generation disk-retention limitation,
not a Core resource, serving-readiness or admission claim. Fresh v2 installations
and subsequent generations use normal flock-based collection. No historical VM
or daemon is killed to clear the marker.

## Qualification boundary

Protocol and process tests do not qualify Runtime readiness, VM coexistence,
native image/store locking, or identity-preserving updates. Those require the
separate exact-artifact KVM and Docker acceptance matrix. In particular, immutable
legacy helper descendant FD behavior must be recorded against its actual artifact;
absence of local test coverage is not evidence of successful garbage collection.

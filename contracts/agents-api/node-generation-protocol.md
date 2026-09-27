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

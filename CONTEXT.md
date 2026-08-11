# Secret Management

Shared language for centrally managing sensitive configuration across one administrative boundary.

## Language

**Organization**:
The single administrative boundary that owns managed infrastructure and its Secrets.
_Avoid_: Tenant, workspace, account

**Secret**:
A named sensitive value required by software, such as an API token, password, certificate, or private key.
_Avoid_: Key, credential, config key

**Desired State**:
The authoritative declaration of which Secrets managed infrastructure is expected to receive.
_Avoid_: Current config, local truth

**Managed Node**:
A host enrolled in an Organization and expected to converge to its assigned Desired State.
_Avoid_: Server, machine, target

**Agent**:
The software identity of a Managed Node that applies Desired State and reports the observed result.
_Avoid_: Probe, daemon, client

**Application**:
A software workload whose Secret requirements are managed as one unit.
_Avoid_: Project, service, repository

**Environment**:
An isolated deployment context of an Application, such as development, staging, or production, that owns its own Secret values and versions.
_Avoid_: Stage, namespace, profile

**Secret Bundle**:
The collection of Secrets belonging to one Application and Environment and assigned together.
_Avoid_: Key set, config file, secret group

**Secret Version**:
An immutable sequence of opaque bytes for a Secret created by one change or rotation.
_Avoid_: Candidate, current value, backup value

**Rotation**:
The replacement of a Secret's value by creating a new Secret Version and publishing a Bundle Revision; it does not create or revoke credentials at an external provider.
_Avoid_: Candidate cycling, automatic provider rotation, in-place overwrite

**Bundle Revision**:
An immutable snapshot that selects one Secret Version for every Secret in a Secret Bundle.
_Avoid_: Release, config version, latest bundle

**Draft**:
A mutable proposed change to a Secret Bundle that has no effect on Managed Nodes.
_Avoid_: Unpublished revision, pending config, working copy

**Publish**:
The Administrator action that freezes a Draft into a Bundle Revision and makes it the Desired State for assigned Node Groups.
_Avoid_: Save, deploy, sync

**Materialized Bundle**:
The node-local file representation of one Bundle Revision made available to an Application.
_Avoid_: Secret dump, config folder, generated files

**File Binding**:
The declaration that maps a Secret to a normalized relative path and constrained ownership and access mode within a Materialized Bundle.
_Avoid_: Secret name, absolute target path, file template

**Activation**:
The atomic switch that makes a Materialized Bundle current on a Managed Node, optionally followed by an allowed service action.
_Avoid_: Install, deploy, write files

**Last Known Good Revision**:
The most recent Bundle Revision successfully activated on a Managed Node.
_Avoid_: Cache, local truth, fallback copy

**Convergence**:
The independent process by which each Managed Node attempts to activate its assigned Desired State while retaining its Last Known Good Revision on failure.
_Avoid_: Global deployment, synchronization, all-or-nothing rollout

**Drift**:
Any node-local change that makes a Materialized Bundle differ from its assigned Bundle Revision.
_Avoid_: Local override, manual fix, pending sync

**Rollback**:
The explicit Administrator action that restores a previous Bundle Revision as Desired State.
_Avoid_: Undo, automatic recovery, downgrade

**Decommissioning**:
The controlled removal of a Managed Node in which its Agent clears Materialized Bundles before its identity is revoked.
_Avoid_: Delete node, uninstall, disconnect

**Emergency Revocation**:
The immediate invalidation of a Managed Node's identity when local cleanup cannot be trusted or confirmed.
_Avoid_: Decommissioning, force delete, remote wipe

**Node Group**:
An explicitly maintained set of Managed Nodes that receive the same Secret Bundle assignments.
_Avoid_: Dynamic selector, cluster, fleet

**Assignment**:
The relationship that makes one Secret Bundle the Desired State for a Node Group without creating multiple sources for the same Application and Environment on any Managed Node.
_Avoid_: Deployment, binding, group override

**Unassignment**:
The explicit removal of a Secret Bundle from a Node Group, requiring affected Applications to stop before their Materialized Bundles are removed.
_Avoid_: Delete files, detach, remove group

**Enrollment Token**:
A short-lived, single-use proof authorizing one Agent to join an Organization and establish its own identity.
_Avoid_: Agent key, shared token, install password

**Reveal**:
An audited action that temporarily exposes a Secret value to a re-authenticated Administrator.
_Avoid_: View, show, unmask

**Step-up Authentication**:
A recent password or TOTP proof required before an Administrator performs a high-risk action.
_Avoid_: Login, confirmation dialog, role check

**Audit Event**:
An immutable record of a security-relevant human, Core, or Agent action and its outcome that never contains Secret values.
_Avoid_: Application log, activity message, history row

**Alert**:
An actionable current condition requiring Administrator attention, delivered in the Web event center and optionally through a signed webhook.
_Avoid_: Audit Event, log entry, notification history

**Retired Secret**:
A Secret that cannot be referenced by new Drafts but remains available to existing Bundle Revisions and audit history.
_Avoid_: Deleted Secret, disabled value, archived key

**Purge**:
The separately authorized destruction of an unreferenced Retired Secret and its Secret Versions while preserving a value-free Audit Event.
_Avoid_: Delete, retire, clear

**Recovery Bundle**:
An offline-encrypted package containing the database state and key material required to restore Core.
_Avoid_: Database dump, volume snapshot, key backup

**Administrator**:
An Organization member allowed to change Desired State and manage Managed Nodes.
_Avoid_: Owner, operator, superuser

**Viewer**:
An Organization member allowed to inspect metadata, status, and audit history without changing Desired State or revealing Secret values.
_Avoid_: Read-only administrator, guest, auditor

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

**Protected Environment**:
An Environment whose Secret and Assignment changes require elevated authorization because they can affect sensitive or production infrastructure.
_Avoid_: Production Environment, secure Environment, locked Environment

**Standard Environment**:
An Environment whose changes follow normal authenticated authorization because it is not classified as requiring elevated protection.
_Avoid_: Unprotected Environment, development Environment, low-security Environment

**Unclassified Environment**:
An existing Environment whose protection level has not yet been confirmed; it is treated as a Protected Environment until an Administrator classifies it.
_Avoid_: Legacy Environment, unknown Environment, standard by default

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

**Bootstrap Code**:
A short-lived, one-time code emitted by Core on first boot that authorizes creation of the first Administrator.
_Avoid_: setup password, admin seed, install code

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

**Activation Policy**:
The Environment-level declaration of an ordered set of systemd units and one allowed service action applied after Activation and used to stop the Application during Unassignment.
_Avoid_: Hook, shell command, deployment script

**Last Known Good Revision**:
The most recent Bundle Revision successfully activated on a Managed Node.
_Avoid_: Cache, local truth, fallback copy

**Convergence**:
The independent process by which each Managed Node attempts to activate its assigned Desired State while retaining its Last Known Good Revision on failure.
_Avoid_: Global deployment, synchronization, all-or-nothing rollout

**Poll Interval**:
The per-node frequency at which the Agent polls Core for Desired State; Core advertises it in every heartbeat/desired response and the Agent adopts it on its next pass (5 seconds to 24 hours).
_Avoid_: Sync frequency, update rate, refresh timer

**Assignment Convergence**:
The independently tracked progress of one Managed Node toward activating the Bundle Revision selected by one Assignment.
_Avoid_: Node status, sync status, deployment status

**Drift**:
Any node-local change that makes a Materialized Bundle differ from its assigned Bundle Revision.
_Avoid_: Local override, manual fix, pending sync

**Rollback**:
The explicit Administrator action that creates a new Bundle Revision from an earlier snapshot and makes the new revision Desired State.
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

**Unassignment Task**:
The persistent, per-node progress record for stopping an Application, removing its Materialized Bundle, and completing an Unassignment.
_Avoid_: Delete job, cleanup request, Assignment deletion

**Abandon Cleanup Confirmation**:
The emergency Administrator action that stops waiting for unreachable Managed Nodes during Unassignment while explicitly retaining their cleanup as unconfirmed.
_Avoid_: Force cleanup, successful removal, skip node

**Enrollment Token**:
A short-lived, single-use proof authorizing one Agent to join an Organization and establish its own identity.
_Avoid_: Agent key, shared token, install password

**Install Command**:
The Administrator-visible command that installs the Agent on a Managed Node; it carries the server URL, a single Enrollment Token, and the Core signing key used to verify the downloaded artifact.
_Avoid_: curl line, setup script, agent installer

**Reveal**:
An audited action that temporarily exposes a Secret value to a re-authenticated Administrator.
_Avoid_: View, show, unmask

**Step-up Authentication**:
A recent password or TOTP proof required before an Administrator performs a high-risk action.
_Avoid_: Login, confirmation dialog, role check

**TOTP**:
A six-digit time-based one-time password generated by the Administrator's authenticator and used as an additional authentication proof.
_Avoid_: OTOP, OTP, dynamic code

**TOTP Login Policy**:
The Organization-wide rule that determines whether local authentication requires TOTP in addition to the Administrator's password; it is disabled by default for new Organizations.
_Avoid_: MFA preference, remember device, optional code

**Password Login Policy**:
The Organization-wide rule that determines whether username-and-password can start a new Session. It may be disabled only while at least one External Identity Binding is usable for login; if no External Identity Provider can log the Administrator in, password login remains available.
_Avoid_: disable local auth, SSO-only mode, passwordless

**External Identity Provider**:
An OpenID Connect provider or OAuth 2.0 authorization server that authenticates the Administrator outside AutoSecrets.
_Avoid_: social login, SSO server

**External Identity Binding**:
The explicit association between the Administrator and one stable issuer-and-subject identity from a configured External Identity Provider. OIDC and OAuth bindings are independent.
_Avoid_: Email match, automatic account linking, OAuth account

**OAuth Authorization Server**:
An OAuth 2.0 authorization server that identifies the Administrator through a userinfo subject rather than an ID Token.
_Avoid_: OpenID Connect provider, social login

**Audit Event**:
An immutable record of a security-relevant human, Core, or Agent action and its outcome that never contains Secret values.
_Avoid_: Application log, activity message, history row

**Operation Reason**:
The Administrator-provided category and explanation for initiating a high-risk action, optionally linked to an external change or incident record.
_Avoid_: Comment, note, commit message

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
The single human identity allowed to access an Organization, change Desired State, and manage Managed Nodes.
_Avoid_: Organization Member, Viewer, user, account, owner, operator, superuser

**Username**:
The Administrator's unique local login name; it is not the internal identity id and is not a display name.
_Avoid_: user id, account name, login id, display name

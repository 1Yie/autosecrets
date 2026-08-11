# AutoSecrets Web Control Plane and Managed Node Agent

Status: ready-for-agent

## Problem Statement

The existing AutoSecrets CLI is a lightweight personal tool that downloads an age-encrypted TOML document and writes decrypted values into local files. It has no Web management experience, no central Desired State, no multi-node identity, no role model, no audit trail, and no safe rollout or rollback protocol. Its current deployment path executes a floating remote script, deletes the target directory before a replacement is ready, allows Secret names to influence paths, and provides no transactional recovery when download, decryption, parsing, or writing fails.

The user needs to manage Secrets for multiple Linux Managed Nodes from a Web application backed by a central Core. A quickly installed Python Agent must continuously converge each Managed Node to assigned Secret Bundle revisions without exposing unrelated Secrets, depending on inbound node connectivity, or destroying a working local configuration when Core or the network is unavailable.

The resulting product must be practical for a self-hosted single Organization while treating Secret confidentiality, node identity, rollback, auditability, supply-chain integrity, and disaster recovery as first-release requirements rather than later add-ons.

## Solution

Build a single-Organization AutoSecrets control plane composed of a React and TypeScript Web application, one Go Core process, PostgreSQL, and one self-contained Python Agent per Linux Managed Node. Core is the only source of Desired State. Administrators organize Secrets by Application and Environment, edit a Draft, and explicitly Publish immutable Bundle Revisions to explicit Node Groups. Viewers may inspect metadata, status, Alerts, and Audit Events but may never Reveal Secret values.

Core encrypts every Secret Version before PostgreSQL persistence, keeps its master key outside the database, and may only hold plaintext briefly in memory. Agents enroll with a short-lived single-use Enrollment Token, establish independent mTLS and envelope-encryption identities, poll a versioned Agent Interface over outbound HTTPS, verify a Core signature, decrypt only their assigned payload, and atomically activate safe relative File Bindings. Failed Activation restores the Last Known Good Revision; network failure leaves current files untouched; Drift is reported and corrected.

The first release is delivered as a versioned Docker Compose deployment with PostgreSQL and optional Caddy, signed self-contained Agent artifacts, append-only Audit Events, signed Webhook Alerts, two-phase Decommissioning, age-encrypted Recovery Bundles, and a guided one-time import plus read-only export path for the legacy age/TOML format.

## User Stories

1. As an Administrator, I want to install Core and PostgreSQL from one supported Compose bundle, so that I have one tested self-hosted deployment path.
2. As an Administrator, I want Web and Agent traffic to use separate HTTPS hostnames on port 443, so that browser sessions and node mTLS have independent trust surfaces without requiring extra outbound ports.
3. As an Administrator, I want a fresh Core deployment to have no default credentials, so that exposing the service cannot expose a known account.
4. As an Administrator, I want to create the first Administrator with a short-lived one-time bootstrap code, so that initial ownership is explicit and cannot be repeated.
5. As an Administrator, I want to configure TOTP during account setup, so that a stolen password alone cannot control Secrets.
6. As an Administrator, I want single-use recovery codes, so that I can recover from loss of my TOTP device without weakening normal authentication.
7. As an Administrator, I want secure cookie-based sessions with predictable expiry and logout, so that browser credentials are not stored as reusable application tokens.
8. As an Administrator, I want Step-up Authentication before high-risk actions, so that an old or hijacked session cannot immediately Reveal or distribute Secrets.
9. As a Viewer, I want to sign in and inspect metadata, node state, Alerts, and Audit Events, so that I can monitor the Organization without receiving mutation or Reveal authority.
10. As a Viewer, I want every control I cannot use to be absent or clearly disabled, so that the Web experience reflects my real authorization.
11. As an Administrator with lost credentials, I want a host-local recovery command, so that recovery does not depend on SMTP or direct database edits.
12. As an Administrator, I want to create an Application, so that one workload's Secret requirements have a clear ownership unit.
13. As an Administrator, I want to create development, staging, and production Environments for an Application, so that deployment contexts remain explicit.
14. As an Administrator, I want each Environment to own independent Secret values and versions, so that a lower-trust Environment cannot inherit production Secrets.
15. As an Administrator, I want to create a named Secret with non-sensitive metadata, so that I can identify and manage its purpose without exposing its value.
16. As an Administrator, I want to enter Secret bytes as text or upload a file, so that API tokens, certificates, private keys, and binary material use one opaque value model.
17. As an Administrator, I want each Secret to have an explicit normalized relative File Binding, so that its domain name is not implicitly interpreted as a filesystem path.
18. As an Administrator, I want File Bindings to reject absolute paths, parent traversal, duplicate targets, reserved names, and symlinks, so that publication cannot escape the managed root.
19. As an Administrator, I want to declare constrained uid, gid, and safe file modes for each File Binding, so that the intended systemd application can read its Secrets without making them world-readable.
20. As an Administrator, I want Secret edits to remain in a Draft, so that saving an edit does not change production nodes.
21. As an Administrator, I want Drafts to use optimistic concurrency, so that another Administrator cannot silently overwrite my changes.
22. As an Administrator, I want a metadata-only conflict diff when my Draft is stale, so that I can reconcile edits without revealing Secret values unnecessarily.
23. As an Administrator, I want to review a Draft diff before Publish, so that I understand which Secret Versions and File Bindings will change.
24. As an Administrator, I want Publish to freeze an immutable Bundle Revision, so that the exact Desired State remains auditable and reproducible.
25. As an Administrator, I want production Publish and Rollback to require recent Step-up Authentication, so that fleet-wide changes have an additional authorization gate.
26. As an Administrator, I want to browse immutable Bundle Revision history, so that I can understand what changed and when.
27. As an Administrator, I want to Rollback by selecting a previous Bundle Revision as Desired State, so that recovery follows the same observable Convergence path as a forward change.
28. As an Administrator, I want Rotation to create a new Secret Version rather than overwrite a value, so that history and rollback remain valid.
29. As an Administrator, I want Core to version and distribute externally issued credentials without invoking their providers, so that provider master credentials and arbitrary rotation code remain outside Core.
30. As an Administrator, I want an optional expiry time on a Secret Version, so that time-limited certificates and tokens can generate advance Alerts.
31. As an Administrator, I want expiry to alert without automatically deleting files or stopping applications, so that a missed rotation does not silently create an outage.
32. As an Administrator, I want Reveal to require recent password or TOTP proof and produce an Audit Event, so that plaintext access is exceptional and attributable.
33. As an Administrator, I want Reveal responses to be short-lived and non-cacheable, so that plaintext does not remain in query caches or browser storage.
34. As a Viewer, I want Reveal to remain impossible even if I know a Secret identifier, so that authorization is enforced by Core rather than only by the Web interface.
35. As an Administrator, I want to Retire a Secret before destruction, so that new Drafts stop referencing it while existing Bundle Revisions and history remain coherent.
36. As an Administrator, I want Purge to require Step-up Authentication and reject any referenced Secret, so that destruction cannot silently break Rollback.
37. As a Viewer, I want a value-free tombstone Audit Event after Purge, so that historical actions remain understandable without retaining the Secret bytes.
38. As an Administrator, I want to create an explicit Node Group, so that I can assign the same Secret Bundle to a known set of Managed Nodes.
39. As an Administrator, I want to manage explicit Node Group membership, so that sensitive distribution does not depend on dynamic or accidental labels.
40. As an Administrator, I want Core to reject overlapping group changes that create two sources for the same Application and Environment on a node, so that Desired State is deterministic.
41. As an Administrator, I want to assign one Secret Bundle to one or more Node Groups, so that all members converge to the same published revision.
42. As an Administrator, I want Unassignment to be an explicit high-risk action, so that removing an application's Secrets cannot happen as a side effect of editing group metadata.
43. As an Administrator, I want Unassignment to stop the configured systemd unit before removing its Materialized Bundle, so that a running process is not left with disappearing configuration.
44. As an Administrator, I want to generate a ten-minute single-use Enrollment Token after Step-up Authentication, so that a copied install command has tightly bounded authority.
45. As an Administrator, I want one generated command to install a pinned signed Agent release and enroll a node, so that adding a Linux Managed Node is fast and repeatable.
46. As an Administrator, I want one Agent per Managed Node, so that multiple Applications share one managed identity, update path, and status reporter.
47. As an Agent, I want to generate certificate and envelope-encryption private keys locally, so that Core never receives my private identity material.
48. As an Agent, I want to exchange an Enrollment Token for my own short-lived certificate, so that compromise of one node does not expose a shared Organization credential.
49. As an Agent, I want to renew my certificate before expiry, so that normal operation does not require repeated manual enrollment.
50. As an Administrator, I want to revoke one Managed Node independently, so that a compromised identity loses access without disrupting healthy nodes.
51. As an Agent, I want to poll Desired State with an ETag and jitter, so that I discover changes promptly without maintaining fragile long-lived connections or synchronizing load spikes.
52. As an Agent, I want a no-change response when my ETag is current, so that steady-state polling remains inexpensive.
53. As an Agent, I want Core to authorize every request against my certificate-bound node identity, so that I cannot request another node's payload.
54. As an Agent, I want each complete Bundle envelope encrypted to my public key and signed by Core, so that proxies, caches, altered payloads, and wrong-node delivery do not expose or forge Desired State.
55. As an Agent, I want to reject unknown protocol versions, expired envelopes, bad signatures, and wrong-key ciphertext, so that delivery failures fail closed.
56. As an Agent, I want to validate a complete revision before changing `current`, so that partial downloads or invalid bindings never become active.
57. As an Agent, I want to stage files without following symlinks and atomically switch the active revision, so that local races cannot redirect Secret writes or expose a partial tree.
58. As an Agent, I want to retain only the current and previous successful plaintext revisions, so that local rollback remains possible without accumulating historical exposure.
59. As an Administrator, I want an Application to configure only `none`, `reload`, or `restart` for one allowed systemd unit, so that Activation cannot become arbitrary remote code execution.
60. As an Agent, I want to verify the configured unit is active after Activation, so that a successful file write is not mistaken for a healthy application.
61. As an Agent, I want to restore the Last Known Good Revision when file Activation or the systemd action fails, so that one bad publication does not leave the node in a partial state.
62. As an Administrator, I want Activation reports to identify each stage and stable failure code, so that I can diagnose validation, write, switch, service, and rollback failures without seeing Secret values.
63. As an Administrator, I want each node to converge independently, so that one failing node does not force healthy nodes back or create false global atomicity.
64. As an Agent, I want to keep the Last Known Good Revision when Core or the network is unavailable, so that control-plane downtime does not become an application outage.
65. As an Administrator, I want offline nodes to remain visibly behind Desired State, so that availability is not confused with Convergence.
66. As an Agent, I want to detect content Drift in bound files, so that local edits cannot silently replace the Core-owned Desired State.
67. As an Agent, I want to restore the assigned revision after Drift, so that the node returns to the authoritative state automatically.
68. As an Administrator, I want repeated Drift to open a high-priority Alert, so that an application or local operator fighting the Agent becomes visible.
69. As an Administrator, I want an overview of unresolved Alerts, offline nodes, Drift, failed Activation, expiring Secret Versions, and recent publications, so that the first screen supports operations rather than marketing.
70. As an Administrator, I want to approve a specific signed Agent version, so that nodes do not silently follow the latest upstream release.
71. As an Agent, I want to verify an update signature and architecture before replacement, so that a modified or incompatible artifact is never executed.
72. As an Agent, I want an update to replace the executable atomically and restore the previous executable on failed startup, so that Agent maintenance is recoverable.
73. As an Administrator, I want normal Decommissioning to wait for Agent cleanup acknowledgement before revoking identity, so that managed plaintext is removed when the node remains reachable.
74. As an Administrator, I want Emergency Revocation to invalidate identity immediately and mark cleanup unconfirmed, so that urgent containment does not make a false cleanup claim.
75. As a Viewer, I want immutable Audit Events for successful and failed security-relevant human, Core, and Agent actions, so that I can reconstruct responsibility and outcomes.
76. As a Viewer, I want to filter Audit Events by actor, action, resource, result, and time, so that incident review is practical without revealing Secret bytes.
77. As an Administrator, I want in-application Alerts to deduplicate and resolve as conditions change, so that the event center represents current action rather than raw log volume.
78. As an Administrator, I want optional signed Webhook delivery with bounded retries and delivery history, so that failures are visible when nobody has the Web application open.
79. As a Webhook receiver, I want replay-resistant signatures and no Secret values in payloads, so that I can trust event origin without becoming another Secret store.
80. As an Administrator, I want an age-encrypted Recovery Bundle containing PostgreSQL state and required key material, so that a database-only backup cannot create an unrecoverable false sense of safety.
81. As an Administrator, I want Recovery Bundle creation to avoid plaintext intermediate archives and include integrity metadata, so that backup creation does not create a new unencrypted Secret repository.
82. As an Administrator, I want an automated restore verification into an empty deployment, so that a backup is not considered successful until it can actually restore Core.
83. As an Administrator, I want a guided one-time import of legacy `secrets.toml.age`, so that existing values can move into the central model without manual re-entry.
84. As an Administrator, I want every legacy rotation candidate imported as a historical Secret Version and to choose the initial current version explicitly, so that migration never guesses which value is authoritative.
85. As an Administrator, I want legacy paths normalized and reviewed with explicit ownership and mode before import, so that old path behavior cannot recreate traversal or permission risks.
86. As an Administrator, I want a read-only legacy export for emergency use, so that I retain a fallback without creating a second write authority.
87. As an Administrator, I want Web to be the only remote Desired State writer in the first release, so that Service Account and automation-token authorization can remain out of scope.
88. As an Administrator, I want all API reads and writes represented by TanStack Query Hooks, so that loading, error, success, retry, and cache behavior are consistent across screens.
89. As an Administrator, I want forms validated with React Hook Form and Zod, so that client and server validation failures are clear before high-risk mutations.
90. As a Web user, I want Watermelon UI components with shadcn primitives only where needed, so that the console has a coherent visual and interaction language.
91. As a keyboard or assistive-technology user, I want every critical workflow to be operable and correctly labeled, so that Secret management is accessible without sacrificing security.
92. As a mobile Web user, I want text, controls, tables, dialogs, and Secret actions to remain non-overlapping and understandable, so that urgent operational work remains possible on a small viewport.
93. As an Administrator, I want the system tested at about 100 Managed Nodes, 100 Applications, and 10,000 active Secrets, so that the stated first-release capacity is evidence-based.
94. As an Administrator, I want healthy Agents to discover new Desired State within 30 seconds, so that publication latency is predictable.
95. As an Administrator, I want a single Core failure to be recoverable through Last Known Good node state and a verified Recovery Bundle, so that the first release can avoid pretending to provide multi-Core high availability.

## Implementation Decisions

- The first release supports one self-hosted Organization, Linux Managed Nodes, and systemd-managed Applications. It does not implement multi-tenancy.
- The deployable system consists of a React and TypeScript Web application, one Go Core process, PostgreSQL, optional Caddy, and one self-contained Python Agent per Managed Node.
- Core is the only source of Desired State. Node-local edits are Drift and are never synchronized back into Core.
- Core uses a database-external master key and per-Secret-Version envelope encryption. Plaintext is limited to controlled in-memory operations and is excluded from persistence, logs, errors, URLs, Audit Events, Webhooks, browser storage, and query keys.
- Data encryption, Agent CA, and Core Bundle-signing keys are separate identities with separate rotation and recovery concerns.
- The Identity Module owns bootstrap, password and TOTP authentication, recovery codes, sessions, role authorization, and Step-up Authentication. Other Modules request authorization decisions instead of reading identity storage directly.
- The Secret Control Module owns Applications, Environments, Secrets, immutable Secret Versions, File Bindings, Drafts, Publish, immutable Bundle Revisions, Rotation, Rollback, Retire, and Purge.
- The Fleet Control Module owns Managed Nodes, Node Groups, explicit memberships, Assignments, Enrollment Tokens, certificate lifecycle, approved Agent versions, Unassignment, Decommissioning, and Emergency Revocation.
- The Delivery Module exposes one high-level Interface that accepts node identity and the prior ETag and returns no change or one complete signed and node-encrypted Desired State envelope.
- The Convergence Module stores desired and observed state separately, records heartbeat and Activation reports, derives offline or behind state, detects sustained Drift, and opens or resolves Alerts.
- The Audit Module appends immutable Audit Events in the same transaction as security-relevant state changes. The application exposes no mutation or deletion Interface for Audit Events.
- The Alerting Module owns in-application Alert lifecycle and signed Webhook delivery. HTTP Webhook delivery is a true external seam with a production Adapter and deterministic test Adapter.
- The Recovery Module streams an age-encrypted Recovery Bundle, restores only into an empty deployment, and performs non-destructive restore verification.
- PostgreSQL is used directly inside owning Core Modules and tested with real PostgreSQL. The implementation must not add one shallow repository Interface per table.
- The conceptual schema includes identity records; Application and Environment records; Secret, Secret Version, File Binding, Draft, Bundle Revision, and revision-entry records; Managed Node, node key, node certificate, Enrollment Token, Node Group, membership, Assignment, Agent release, and update records; Convergence, Alert, Webhook delivery, decommission task, and append-only Audit Event records.
- A Secret Version belongs to one Environment and is immutable. An Environment cannot reference a Secret Version owned by another Environment.
- A published Bundle Revision is immutable and selects one Secret Version for each Secret in a Secret Bundle.
- Draft writes use optimistic concurrency through an ETag or equivalent version value. Stale writes fail with a metadata-only conflict representation.
- Assignment writes are rejected transactionally when overlapping Node Groups would create multiple sources for one Application and Environment on a Managed Node.
- File Bindings use normalized relative POSIX paths and constrained uid, gid, and mode declarations. Absolute paths, parent traversal, duplicate targets, symlink following, unknown owners, and world-readable modes are rejected.
- Management and Agent traffic use separate public hostnames on port 443. The management hostname uses browser sessions; the Agent hostname uses token enrollment followed by short-lived mTLS certificates.
- If a reverse proxy terminates mTLS, Core trusts certificate identity headers only from an explicitly configured private proxy connection. Direct access to the internal listener is not public.
- The management Interface is a versioned REST contract for browser and administrative workflows. OpenAPI is the transport contract source for generated TypeScript types.
- The Agent Interface is a separate versioned REST contract for enrollment, certificate renewal, ETag polling, heartbeat, Activation reports, update metadata, cleanup acknowledgement, and decommission status.
- Enrollment Tokens are stored only as hashes, expire after ten minutes by default, are single-use, and are issued only after Step-up Authentication.
- Agent certificate and envelope-encryption private keys are generated and retained locally. Core stores the public keys, certificate serial and status, and historical revocation evidence.
- Agent payloads are encrypted to the destination Agent and signed by Core in addition to mTLS. The cross-language envelope is versioned and uses a reviewed age or HPKE-compatible implementation with checked-in Go and Python known-answer vectors; custom cryptography is prohibited.
- Agent polling uses HTTPS, ETags, jitter, and bounded exponential backoff. Core does not initiate inbound connections to nodes.
- The Materializer exposes the high-level `Activate(envelope) -> ActivationResult` Interface. It validates a complete revision, stages files without following symlinks, atomically switches `current`, performs an allowed systemd action, checks unit health, and restores the Last Known Good Revision on failure.
- Agent systemd actions are limited to `none`, `reload`, or `restart` for an explicitly allowed unit. Arbitrary shell hooks are not supported.
- Managed Nodes retain only current and previous successful plaintext materializations. Older revisions are retrieved again from Core when explicitly required.
- Each node converges independently. A failed node retains its Last Known Good Revision while healthy peers remain on the new revision; Core never claims a distributed all-or-nothing transaction.
- Unassignment is explicit and stops the configured unit before deleting its Materialized Bundle. Normal Decommissioning cleans and acknowledges before revocation; Emergency Revocation invalidates identity immediately and records cleanup as unconfirmed.
- Agent updates require Administrator approval, signed artifacts, architecture checks, atomic executable replacement, startup health verification, and binary rollback.
- Core deployment is a versioned Docker Compose bundle with PostgreSQL and optional Caddy. It supports one active Core instance in the first release and relies on Last Known Good node state plus verified Recovery Bundles for outage recovery.
- The Web application uses function components and Hooks, kebab-case filenames, PascalCase component identifiers, camelCase Hook identifiers, explicit props interfaces, and no component-level `any`.
- Watermelon UI is the primary component registry. shadcn/ui supplies missing foundational primitives. Copied registry source is treated as project-owned code subject to accessibility, security, typing, and testing rules.
- TanStack Query owns server state, React Hook Form and Zod own forms, local `useState` owns transient local UI, and Zustand owns only shared client state. React Context and Redux are not application state stores.
- The Web information architecture begins with an operational Overview and includes Applications, Nodes, Audit, and Settings areas. It is not a marketing landing page.
- Legacy age/TOML is a one-time import and read-only export format. It is not a Core storage format or second source of truth.
- Legacy rotation candidates become historical Secret Versions. The Administrator must explicitly select the initial current value before producing the first Bundle Revision.
- The first-release performance baseline is about 100 Managed Nodes, 100 Applications, 10,000 active Secrets, and discovery of new Desired State within 30 seconds under healthy conditions.

## Testing Decisions

- Good tests assert externally observable behavior through the highest stable Interface. They do not assert private method calls, table-access implementation details, React component internals, or incidental log wording.
- The primary test seam is the complete running system: browser or management REST input enters a real Go Core and real PostgreSQL, a test Agent consumes the Agent Interface, and assertions observe Web-visible state plus the Agent's Activation report.
- The only additional fault-injection seam is the Agent Materializer Interface, `Activate(envelope) -> ActivationResult`, backed by a temporary filesystem and fake systemd Adapter. This seam exists because deterministic disk, symlink, permission, and systemd failures cannot be exercised reliably through a production host.
- Core Module tests invoke the Identity, Secret Control, Fleet Control, Delivery, Convergence, Audit, Alerting, and Recovery Interfaces rather than mocking internal functions.
- PostgreSQL integration tests use a real PostgreSQL instance for migrations, constraints, transaction races, encryption round trips, assignment ambiguity, optimistic Draft conflicts, and append-only audit behavior.
- Identity tests cover one-time bootstrap, password hashing, TOTP setup and replay, recovery-code consumption, session rotation, logout, CSRF, role denial, expired Step-up grants, and host-local recovery evidence.
- Secret Control tests cover opaque values, Environment isolation, immutable Secret Versions and Bundle Revisions, Draft conflict behavior, File Binding validation, Publish, Rollback, Rotation, expiry metadata, Reveal authorization, Retire, reference-protected Purge, and tombstone Audit Events.
- Fleet Control tests cover hashed Token storage, Token expiry and reuse, key-possession proof, certificate renewal and revocation, explicit membership, assignment conflicts, approved updates, Unassignment, Decommissioning, and Emergency Revocation.
- Protocol tests use checked-in Go and Python known-answer vectors. They cover valid delivery, wrong node, wrong key, modified ciphertext, modified manifest, modified signature, expired envelope, unknown protocol version, and replay behavior.
- Materializer tests cover absolute paths, parent traversal, duplicate paths, symlink races, unknown uid/gid, unsafe modes, disk full, interrupted writes, failed rename, failed reload, failed restart, inactive unit, rollback failure reporting, and plaintext revision retention.
- Convergence tests cover healthy Activation, one-node partial failure, Core outage, resumed polling, stale ETags, offline status, Drift detection, automatic restoration, repeated-Drift Alert deduplication, and explicit Rollback.
- Agent update tests cover valid signed replacement, corrupt artifact, invalid signature, wrong architecture, interrupted replacement, failed startup health, and restoration of the previous executable.
- Alerting tests cover signed Webhook payloads, replay protection, redaction, timeouts, retry backoff, deduplication, disabled endpoints, exhausted delivery, and resolution.
- Recovery tests create a real age-encrypted Recovery Bundle and restore an empty deployment. Success requires decryptable Secret Versions, valid user and TOTP state, usable Agent identity history, intact Audit Events, valid signing capability, and a passing automated verification.
- Legacy migration tests use representative age/TOML fixtures and cover malformed TOML, invalid identity, traversal paths, duplicate targets, candidate mapping, explicit current selection, all-or-nothing import, and legacy-compatible read-only export.
- Web unit tests use Vitest for pure utilities and Hooks. Component behavior uses React Testing Library with MSW-backed API behavior.
- Web tests verify loading, empty, error, retry, stale, conflict, success, denied, and redacted states for every asynchronous workflow.
- Playwright covers bootstrap, login, TOTP, Viewer denial, Secret creation, Draft conflict, Publish, Reveal, Rollback, Agent install command, Convergence, failed Activation, Drift, update, Unassignment, Decommissioning, Emergency Revocation, Alert acknowledgement, migration, and recovery-status workflows.
- Browser security tests prove Reveal data uses a non-cacheable mutation and is absent from local storage, session storage, IndexedDB, query caches, URLs, analytics, screenshots, and captured console output after dismissal.
- Accessibility tests cover keyboard navigation, focus return, labels, validation messaging, dialogs, tables, status changes, and high-risk confirmation workflows.
- Responsive browser tests use desktop and mobile viewports and check that text, actions, tables, dialogs, Secret controls, and status indicators do not overlap or shift incoherently.
- System capacity tests run at least 100 jittered Agents and 10,000 active Secrets and must meet the 30-second Desired State discovery target with bounded PostgreSQL and Core resource use.
- Security verification includes secret scanning, dependency and container scanning, fuzzing of path normalization and envelope parsing, proxy-trust tests, Webhook SSRF review, artifact signature verification, and an SBOM for Core and Agent releases.
- The current Web skeleton and legacy CLI contain no applicable test prior art. This specification establishes the initial testing patterns; later tests must reuse these highest seams rather than proliferating new mock layers.

## Out of Scope

- Multi-Organization tenancy and SaaS operation.
- Windows, macOS, Kubernetes, container-native Secret injection, and non-systemd process management.
- Environment-variable injection, arbitrary absolute paths, configuration templates, arbitrary shell hooks, and user-defined health-check scripts.
- OIDC, Passkeys, SMTP password recovery, Service Accounts, remote management CLI writes, CI/CD write tokens, and fine-grained per-Environment RBAC.
- Provider-specific automatic credential creation, Rotation, or revocation.
- Dynamic Node Group selectors, group priority, inherited Environments, shared Secret Versions, merged Secret Bundles, and bidirectional local synchronization.
- Canary and batch rollout orchestration, first-failure rollout pausing, and automatic fleet-wide rollback.
- Multiple active Core instances, PostgreSQL high-availability orchestration, horizontal partitioning, and a multi-region control plane.
- External KMS or Vault as a first-release requirement.
- SIEM-specific, email, or chat-platform-specific notification integrations beyond signed generic Webhooks.
- A second database implementation or local SQLite production mode.
- Live synchronization with legacy `secrets.toml.age` or retention of the legacy Candidate domain type.
- Guaranteed secure erasure from SSD, copy-on-write, snapshotting, or compromised root filesystems.
- Protection of Secret plaintext from a root-compromised Managed Node or from an attacker holding both PostgreSQL data and the Core master key.

## Further Notes

- The domain glossary and architecture decisions are authoritative for naming and durable trade-offs. Proposed implementation changes that conflict with them require explicit domain-modeling and a superseding decision.
- The current repository contains a minimal React and TypeScript Web skeleton plus design and architecture documentation. Core and Agent implementation do not yet exist.
- The legacy CLI remains a factual migration reference, not a base architecture. Its pure rotation concepts may inform migration parsing, but its global URL, subprocess, filesystem, command-line, and delete-before-write behavior must not be carried into Core or Agent Modules.
- The implementation plan intentionally begins with two risk gates: trusted mTLS identity propagation through the proxy and reviewed Go/Python envelope interoperability. Feature implementation must not bypass either gate.
- The exact cross-language envelope library and Python self-contained packaging tool remain implementation-level selections. They are acceptable only after known-answer, compatibility, signing, and supported-Linux evidence passes.
- `ready-for-agent` means the product decisions and test seams are settled. The implementation should still be delivered in the phased vertical slices defined by the project implementation plan rather than as one unreviewable change.

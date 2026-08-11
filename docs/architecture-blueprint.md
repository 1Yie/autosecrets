# AutoSecrets Architecture Blueprint

## Status

This blueprint consolidates the confirmed product, security, deployment, and engineering decisions for the first release. Domain language is defined in [CONTEXT.md](../CONTEXT.md), durable trade-offs are recorded in [docs/adr](./adr/), and Web implementation rules are defined in [frontend-guidelines.md](./frontend-guidelines.md).

## Goals

- Give one self-hosted Organization a central, auditable Desired State for Secrets across Linux Managed Nodes.
- Let an Administrator install one Python Agent per node with a short-lived, single-use command.
- Materialize versioned Secret files atomically and keep applications running through Core or network outages.
- Make every high-risk action attributable, recoverable where possible, and explicit when it is destructive.
- Support about 100 Managed Nodes, 100 Applications, and 10,000 active Secrets, with new Desired State discovered within 30 seconds under healthy conditions.

## Non-goals for the first release

- Multi-tenant SaaS, Windows, macOS, Kubernetes, or container-native Secret injection.
- Provider-specific credential creation or revocation.
- Arbitrary file paths, configuration templates, shell hooks, or bidirectional local edits.
- Service accounts, remote write-capable CLI clients, or public automation tokens.
- Multi-Core high availability or multiple database implementations.
- Live synchronization with the legacy `secrets.toml.age` format.

## System context

```text
Browser
  | HTTPS, session + TOTP
  v
secrets.example.com:443
  |                     Core Compose network
  +--> Caddy ----------> Go Core ----------> PostgreSQL
                           |  |                  ciphertext + metadata
                           |  +--------------> external master-key file
                           +-----------------> Agent CA + signing keys

Python Agent
  | HTTPS, short-lived mTLS certificate
  v
agents.example.com:443
  +--> Caddy ----------> /agent/v1
                            |
                            +--> node-encrypted, Core-signed Bundle envelope

Python Agent ----------> managed revision directories + systemd
```

The two public hostnames may resolve to one host and one Caddy instance. The Web hostname never requests a node certificate. The Agent hostname permits token-authenticated enrollment and requires a verified node certificate for all post-enrollment routes.

If Caddy terminates Agent mTLS, Core accepts forwarded certificate identity only from the private proxy network and only through a configured trusted-proxy mode. Direct public access to the internal Core listener is forbidden.

## Trust model

### Core may do

- Decrypt a Secret Version briefly in memory for Reveal, publication, export, or node delivery.
- Issue and revoke node certificates through a dedicated Agent CA.
- Sign Bundle envelopes through a key separate from the data-encryption master key and Agent CA.

### Core must not do

- Persist plaintext Secret values, log them, place them in URLs, include them in errors, or expose them to Viewers.
- Accept a node identity solely from an untrusted proxy header.
- execute arbitrary commands on Managed Nodes.
- Treat local Agent state or a legacy file as a second source of truth.

### Explicitly outside the protection boundary

- A root-compromised Managed Node can read Secrets materialized for applications on that node.
- An attacker holding both the PostgreSQL data and the Core master key can decrypt stored Secret Versions.
- A malicious Administrator who passes Step-up Authentication can reveal or publish values; Audit Events provide attribution, not prevention.

## Core Modules

Each Module has one external Interface. Internal seams exist only where a production Adapter and a deterministic test Adapter are both useful.

### Identity Module

**Interface:** bootstrap the first Administrator, authenticate password and TOTP, create and revoke sessions, authorize Administrator or Viewer actions, and require recent Step-up Authentication.

It hides password hashing, TOTP secrets, recovery codes, secure-cookie rotation, CSRF protection, session expiry, and host-local break-glass recovery. Components outside the Module ask for an authorization decision rather than reading roles or session tables directly.

### Secret Control Module

**Interface:** manage Applications and Environments; create Secrets and immutable Secret Versions; edit Drafts with optimistic concurrency; Publish and Rollback Bundle Revisions; and Retire or Purge Secrets.

It owns these invariants:

- A Secret Version belongs to exactly one Environment and is never updated in place.
- A published Bundle Revision is immutable.
- A Draft save must present the current version or ETag.
- Environment values cannot be shared by reference.
- Purge requires Step-up Authentication, no live references, and a value-free tombstone Audit Event.

### Fleet Control Module

**Interface:** enroll and describe Managed Nodes; manage explicit Node Groups and Assignments; resolve assignment conflicts; approve Agent versions; renew or revoke node identity; Unassign, Decommission, or Emergency Revoke a node.

It rejects any change that would give a Managed Node more than one source for the same Application and Environment. Enrollment Tokens are stored only as hashes and are single-use with a ten-minute default lifetime.

### Delivery Module

**Interface:** `GetDesiredEnvelope(nodeIdentity, ifNoneMatch)` returns no change or one authenticated envelope representing the complete Desired State for that node.

The Module resolves Assignments, creates a canonical manifest, seals Secret bytes to the Agent encryption key, signs the envelope, and produces its ETag. Callers never assemble partial node payloads themselves.

The cross-language envelope format is versioned and includes at least:

- protocol version, node ID, Bundle Revision ID, creation time, expiry, and manifest hash;
- normalized File Bindings and constrained uid, gid, and mode declarations;
- encrypted payload bytes and encryption-suite identifier;
- Core signing-key ID and signature.

The implementation must use a reviewed age or HPKE-compatible library rather than custom cryptography. Go and Python test vectors are checked into the protocol package before the first Agent delivery is accepted.

### Convergence Module

**Interface:** record Agent heartbeat and Activation reports, derive current node status, detect sustained Drift or offline state, and open or resolve Alerts.

It models desired and observed state separately. A failed node remains visibly on its Last Known Good Revision while successful peers continue to converge. It never invents global atomicity across nodes.

### Audit Module

**Interface:** append one structured Audit Event in the caller's transaction and query immutable events with redacted metadata filters.

Security-relevant state changes fail if their Audit Event cannot be committed. No application Interface updates or deletes audit rows. Partitioning or archival may be added internally without changing this Interface.

### Alerting Module

**Interface:** open, deduplicate, resolve, and list Alerts; enqueue signed webhook deliveries with retry state.

HTTP delivery is a true external seam. Production uses an HTTP Adapter; tests use an in-memory Adapter. Payloads contain identifiers and status metadata, never Secret values or Bundle bytes.

### Recovery Module

**Interface:** stream an age-encrypted Recovery Bundle to an explicit recipient, restore into an empty deployment, and run a non-destructive restore verification.

The implementation coordinates PostgreSQL logical backup with the external data master key, Agent CA, and Core signing material without writing a plaintext archive to disk.

## Agent Modules

### Enrollment Module

Consumes the Core URL and Enrollment Token, creates node certificate and encryption keys locally, proves key possession, receives the short-lived certificate, and stores private material root-only. It never sends private keys to Core.

### Sync Module

Polls `/agent/v1` over mTLS with an ETag, applies bounded jitter and exponential backoff, verifies the Core signature, decrypts the node envelope, and hands a complete revision to the Materializer. Network failure leaves current files untouched.

### Materializer Module

**Interface:** `Activate(envelope) -> ActivationResult`.

This is the Agent's deepest Module. It validates every File Binding, stages a complete revision, writes through file descriptors without following symlinks, applies constrained ownership and modes, atomically switches `current`, performs the allowed systemd action, verifies the unit is active, and restores the Last Known Good Revision on failure.

Only the current and previous successful revision remain in plaintext on disk. Older materializations are removed; secure erasure on SSD or copy-on-write storage is not promised.

### Drift Module

Hashes bound files without following symlinks, reports mismatches, and asks the Materializer to restore the current Desired State. Repeated Drift opens a high-priority Alert.

### Lifecycle Module

Verifies Administrator-approved signed Agent releases, replaces the executable atomically, and restores the previous executable if startup health fails. It also performs the cleanup side of Unassignment and two-phase Decommissioning before certificate revocation.

Filesystem and systemd are local-substitutable dependencies. Production uses Linux Adapters; tests exercise the same Agent Interfaces through temporary filesystems and a deterministic fake systemd Adapter.

## Conceptual data model

The schema should preserve domain constraints rather than mirror HTTP resources one-for-one.

| Area | Records |
| --- | --- |
| Identity | organization, user, password credential, TOTP credential, recovery code, session, step-up grant |
| Secret control | application, environment, secret, secret version, file binding, draft, draft entry, bundle revision, revision entry |
| Fleet | managed node, node key, node certificate, enrollment token, node group, group membership, assignment, agent release, approved update |
| Operations | convergence report, decommission task, alert, webhook endpoint, webhook delivery |
| Evidence | append-only audit event |

Important storage rules:

- Secret plaintext is encrypted before a database statement is built.
- Each Secret Version uses a random data-encryption key and versioned AEAD metadata. The data key is wrapped by the external master key; changing the master key rewraps data keys without rewriting plaintext values.
- AEAD associated data binds Organization, Environment, Secret, and Secret Version identifiers.
- Draft and Assignment changes use database constraints plus serializable or explicitly locked transactions where ambiguity could otherwise be committed.
- Certificate serials, public keys, status, and revocation state are retained after node removal for Audit Event interpretation.
- Secret names, paths, application names, and other metadata are not treated as plaintext Secret values, but logs still minimize them.

Use PostgreSQL directly inside the owning Core Module and test with real PostgreSQL. Do not create one shallow repository Interface per table. Add an internal seam only when a second Adapter is real and useful.

## Primary flows

### First boot and Administrator bootstrap

1. Compose starts PostgreSQL, Core, and Caddy with mounted key volumes.
2. Core initializes or validates the data master key, Agent CA, and signing key.
3. Core emits one short-lived bootstrap code without a default password.
4. The browser creates the first Administrator, configures TOTP, downloads recovery codes, and consumes the bootstrap code.
5. Core appends the bootstrap Audit Event.

### Secret creation and publication

1. The Administrator creates an Application, Environment, Secret Bundle, and File Bindings.
2. Secret bytes enter through a text field or file upload and are encrypted immediately.
3. Edits remain in a versioned Draft. A stale ETag returns a conflict and a metadata-only diff.
4. Publish requires recent Step-up Authentication for production and freezes one Bundle Revision.
5. Assignments make that revision Desired State for explicit Node Groups.

### Agent enrollment

1. Web creates a ten-minute, single-use Enrollment Token after Step-up Authentication.
2. The generated command installs a pinned, signed Agent artifact and presents the token once.
3. Agent creates its mTLS and envelope-encryption private keys locally and proves possession.
4. Core consumes the token, registers the node and public keys, and issues a short-lived certificate.
5. Agent starts its systemd unit and begins ETag polling.

### Convergence and Activation

1. Agent polls with node identity and the previous ETag.
2. Core returns `304` or a node-encrypted, signed complete envelope.
3. Agent verifies, decrypts, validates, stages, and activates the revision.
4. Agent performs `none`, `reload`, or `restart`, checks the configured unit, and reports each stage.
5. Core updates observed state, appends an Audit Event, and opens or resolves Alerts.

### Rollback and Drift

- Rollback explicitly restores a previous Bundle Revision as Desired State; nodes converge through the normal flow.
- Local file changes are Drift, not a new source of truth. Agent reports and restores the assigned revision.

### Unassignment and Decommissioning

- Unassignment stops the configured unit before deleting that Application's Materialized Bundle.
- Normal Decommissioning waits for Agent cleanup acknowledgement before revoking the certificate.
- Emergency Revocation invalidates identity immediately and records local cleanup as unconfirmed.

## HTTP Interfaces

### Management Interface: `/api/v1`

- Same-origin secure-cookie sessions, CSRF protection, role authorization, and Step-up grants.
- Versioned REST resources for identity, Applications, Environments, Secrets, Drafts, Bundle Revisions, Node Groups, Assignments, Managed Nodes, Alerts, Webhooks, and Audit Events.
- Reveal is a non-cacheable `POST` action, never a queryable `GET`; responses use `Cache-Control: no-store` and are not retained in TanStack Query cache.
- Mutations that may be retried use idempotency keys. Draft mutation uses `If-Match` or an equivalent version field.
- OpenAPI is the contract source for generated TypeScript transport types. React components only call feature Hooks that wrap TanStack Query.

### Agent Interface: `/agent/v1`

- Token-authenticated enrollment is the only pre-certificate route.
- Certificate renewal, desired envelope polling, Activation reporting, heartbeat, update metadata, cleanup acknowledgement, and decommission status require mTLS.
- Every request is authorized against the node ID bound to the verified certificate.
- Error bodies contain stable codes and correlation IDs, never Secret values.

## Web information architecture

The first screen after login is the operational console, not a landing page.

- **Overview:** unresolved Alerts, offline or drifting nodes, pending convergence, expiring Secret Versions, and recent publications.
- **Applications:** Environment switcher, Secret metadata, File Bindings, Draft editor, revision diff, Publish, Rollback, Retire, and Purge.
- **Nodes:** Managed Nodes, Node Groups, Assignments, install command, convergence detail, approved updates, Unassignment, and Decommissioning.
- **Audit:** immutable event search and actor/resource/result filters.
- **Settings:** users and roles, Webhooks, key and certificate health, backup verification, and local recovery guidance.

Feature code follows [frontend-guidelines.md](./frontend-guidelines.md). Watermelon UI registry source is preferred, shadcn/ui supplies missing primitives, and all imported source is reviewed as project code. TanStack Query owns server state, React Hook Form and Zod own forms, Zustand owns only shared client state, and local UI state stays local.

## Deployment layout

### Core host

```text
deploy/
  compose.yaml
  caddy/Caddyfile
  env/core.env
volumes/
  postgres/
  keys/master.key
  keys/agent-ca.*
  keys/core-signing.*
```

Production documentation must require two DNS names, trusted HTTPS certificates, root-only key permissions, database and key volume ownership, and a tested Recovery Bundle before the instance is considered ready.

### Managed Node

```text
/opt/autosecrets-agent/                 signed executable and previous executable
/etc/autosecrets-agent/config.toml      Core URL and non-secret settings
/var/lib/autosecrets-agent/identity/    node private keys and certificates
/var/lib/autosecrets-agent/bundles/
  <application-id>/<environment-id>/
    revisions/<revision-id>/
    current -> revisions/<revision-id>/
```

The installer creates the dedicated directories, validates architecture and systemd, verifies the release signature before execution, enrolls once, writes the unit, and removes the Enrollment Token from temporary state.

## Observability and operational targets

- New Desired State discovered within 30 seconds under healthy conditions at the target scale.
- Agent polling uses jitter so nodes do not synchronize requests.
- A node becomes offline after a configurable number of missed heartbeats; the default should be longer than transient network jitter.
- Health endpoints distinguish process liveness, PostgreSQL readiness, key readiness, and migration readiness.
- Metrics cover request latency, polling outcomes, Activation stages, Drift, certificate expiry, webhook retries, and backup verification. Labels never contain Secret names, paths, or values.
- Structured logs use correlation IDs and stable error codes; Audit Events remain a separate immutable record.

## Legacy migration

The importer accepts an age-encrypted legacy TOML file and an explicit age identity. The Administrator selects the target Application and Environment, reviews normalized relative paths, chooses uid/gid/mode, imports every rotation candidate as a historical Secret Version, and explicitly selects the initial current version before the first Bundle Revision can be published.

Legacy export is read-only and produces the old format for emergency use. It cannot write back into Core, infer current values from old node directories, or reintroduce Candidate as a Core domain type.

## Verification strategy

- **Core Module tests:** pure state and authorization tests through each Module Interface.
- **PostgreSQL integration:** migrations, constraints, transaction races, append-only audit behavior, and encrypted record round trips against real PostgreSQL.
- **Protocol contract:** OpenAPI validation plus checked-in Go/Python envelope test vectors, malformed input, wrong-node ciphertext, expired envelope, and signature failure cases.
- **Agent tests:** temporary filesystem and fake systemd Adapters for atomic activation, rollback, traversal, symlink, ownership, Drift, update, and decommission paths.
- **Web tests:** Vitest, React Testing Library, MSW, and Playwright as specified in the frontend guidelines.
- **System tests:** Compose Core plus multiple isolated Agent fixtures covering enrollment, Publish, partial failure, Rollback, offline recovery, certificate renewal, signed update, and Recovery Bundle restore.
- **Capacity tests:** at least 100 jittered Agents and 10,000 active Secrets without missing the 30-second discovery target.

## Decision index

The durable choices behind this blueprint are recorded in [ADR 0001 through ADR 0012](./adr/). Changes to those choices require superseding ADRs rather than silent edits to this document.

# First Vertical Slice: Bootstrap to Secret Landing

Status: ready-for-agent

## Problem Statement

The full product shape is specified in `.scratch/autosecrets-control-plane/spec.md`. That
spec spans seven implementation phases, but nothing end-to-end exists yet: the Core is a
skeleton, the Agent has no enrollment or sync, and the Web panel is an empty scaffold. The
operator's core promise — "deploy the panel, curl-install an Agent on a server, and its
Secrets stay in sync" — is unproven until one complete vertical path works: first boot,
authoring, Publish, Install Command, Enrollment, polling, and Secret files landing on a
Managed Node with the declared ownership and mode.

This spec fixes the boundary of the first vertical slice and the decisions confirmed during
the grilling session that produced it. It deliberately trims identity hardening, service
actions, and fleet rules to their documented minimum so the experience can be proven and
measured before the remaining phases are built on top.

## Solution

Deploy the existing Compose bundle and open the panel. First boot prints a one-time
Bootstrap Code to Core logs; the Administrator consumes it to create the first account and
logs in with a password. From the panel the Administrator creates an Application and an
Environment, uploads opaque Secret bytes (each getting an editable default File Binding
whose filename equals the Secret name), edits in a Draft, and Publishes an immutable Bundle
Revision. A minimal Node Group receives the Assignment.

The Nodes screen offers a copyable Install Command of the form
`curl -fsSL https://<agent-host>/install.sh | sudo bash -s -- --server <url> --token <token>`.
The script embeds the Core signing public key, downloads the signed Agent artifact and its
signature from the same origin, and verifies before executing (ADR-0013). The ten-minute,
single-use Enrollment Token is shown once. The Agent generates local mTLS and
envelope-encryption keys, proves possession, enrolls, and begins ETag polling. Within the
30-second target it verifies and decrypts its node-specific envelope, stages the
Materialized Bundle, and switches it atomically into `current/`. Activation performs no
service actions in this slice; on failure the previous revision remains active.

## User Stories

1. As an Administrator, I want to deploy the panel from one supported Compose bundle, so that a self-hosted Secrets control plane exists without manual assembly.
2. As an Administrator, I want a fresh deployment to have no default credential, so that exposing the service cannot expose a known account.
3. As an Administrator, I want first boot to print a one-time Bootstrap Code to Core logs, so that I can prove initial ownership before anything is stored.
4. As an Administrator, I want to consume the Bootstrap Code to create my account with a password, so that the panel is secured before any Secret is created.
5. As an Administrator, I want to log in with my password and log out, so that only I can manage Secrets from my browser.
6. As an Administrator, I want to create an Application, so that the Secret requirements of one workload are managed as a unit.
7. As an Administrator, I want to create an Environment such as production for an Application, so that values are isolated per deployment context.
8. As an Administrator, I want to create a Secret by typing or uploading opaque bytes, so that values are never persisted or transmitted in plaintext.
9. As an Administrator, I want every new Secret to receive a default File Binding whose filename equals the Secret name, so that the common path needs no extra configuration.
10. As an Administrator, I want to edit or remove a default File Binding and declare ownership and mode, so that least-privilege layout stays under my control.
11. As an Administrator, I want unsafe File Binding declarations rejected before Publish, so that absolute paths, `..`, duplicate targets, and unsafe modes can never reach a node.
12. As an Administrator, I want Draft edits to fail with an explicit conflict instead of overwriting, so that concurrent changes are never silently lost.
13. As an Administrator, I want to Publish a Draft into an immutable Bundle Revision, so that Managed Nodes converge to a known Desired State.
14. As an Administrator, I want a Node Group with explicit members, so that one Assignment reaches exactly the nodes I choose.
15. As an Administrator, I want to Assign a Bundle Revision to a Node Group, so that its Managed Nodes are expected to converge.
16. As an Administrator, I want a copyable Install Command for a node containing a ten-minute, single-use Enrollment Token, so that enrolling a Managed Node is one command.
17. As an Administrator, I want the Install Command to verify the downloaded Agent artifact against the Core signing key, so that a compromised or cached download cannot execute on my node.
18. As an Administrator, I want the Token displayed exactly once and expired quickly, so that a leaked command cannot enroll a second node.
19. As an Administrator, I want the enrolled node to appear with its status, so that I can confirm enrollment and observe convergence.
20. As an Administrator, I want a published revision to reach an enrolled node within the 30-second target, so that rotation is fast enough to be trusted.
21. As an Administrator, I want Secret files to land under the bundle directory with the declared relative path, ownership, and mode, so that applications read them with least privilege.
22. As an Administrator, I want a failed Activation to leave the previous revision active, so that a bad Publish never breaks a working node.
23. As an Administrator, I want Secret values to never appear in logs, errors, Audit Events, or the browser's storage after dismissal, so that redaction holds everywhere.
24. As an Administrator, I want every security-relevant action recorded as an Audit Event in the same transaction as the change, so that I can answer who did what.
25. As an Administrator, I want a rejected, expired, or reused Enrollment Token to fail closed, so that identity cannot be forged through replay.
26. As an Administrator, I want the Agent to reject a wrong-key, modified, or expired envelope, so that forged Desired State is never activated.
27. As an Administrator, I want a Core outage to leave node files untouched and polling to resume afterward, so that operations survive control-plane downtime.
28. As the Agent, I want to enroll once with a Token, generate my own mTLS and encryption keys, and prove possession, so that Core never sees my private key material.
29. As the Agent, I want to poll with my node identity and previous ETag, so that unchanged Desired State costs nothing.
30. As the Agent, I want to verify, decrypt, validate, stage, and atomically switch the Materialized Bundle, so that no partial tree is ever current.

## Implementation Decisions

- **Slice boundary** (from the grilling session): bootstrap → password login → authoring
  path → minimal Node Group and Assignment → Install Command → Enrollment → ETag polling →
  files land. Everything not listed here is deferred, not silently assumed.
- **Identity (slice-1 subset)**: Core emits a short-lived, one-time Bootstrap Code to its
  logs on first boot; consuming it creates the first Administrator. No default credential.
  Password login with secure-cookie sessions and CSRF protection. Administrator is the only
  role in this slice. TOTP, recovery codes, and Step-up Authentication are deferred; until
  they land, Publish and Install Command generation do not require Step-up — a documented
  temporary weakening of the control-plane spec's stories 5-8.
- **Secret Control (slice-1 subset)**: Application, Environment, Secret (opaque bytes),
  immutable Secret Version, File Binding, Draft, Bundle Revision, Publish. Each Secret
  Version is encrypted before persistence with a versioned AEAD record and a wrapped
  per-version data key; the master key stays outside PostgreSQL (ADR-0003). Draft edits use
  optimistic concurrency (ETag / If-Match) and return metadata-only conflict diffs.
  File Binding validation covers relative POSIX paths, duplicate targets, and the safe mode
  allowlist. Default binding: filename equals the Secret name, path inside the bundle
  directory, editable or removable. Rollback is deferred.
- **Fleet Control (slice-1 subset)**: Node Group with explicit membership and Assignment of
  one Bundle Revision to one Node Group. Overlapping-group ambiguity rejection is deferred;
  a duplicate Assignment fails with a plain error.
- **Enrollment**: hashed, single-use, ten-minute Enrollment Tokens. The Agent generates
  independent mTLS and envelope-encryption key pairs locally and proves possession; Core
  registers the node and public keys and issues a short-lived certificate from the internal
  CA (ADR-0006). Tokens are consumed exactly once and cannot be exchanged for a second node.
- **Install Command** (ADR-0013): the script is served by Core over TLS from the Agent
  hostname; it embeds the Core signing public key and receives `--server <url>` and
  `--token <token>` as arguments. The signed Agent artifact and its signature are
  downloaded from the same origin and verified before execution. The Token is never
  embedded in the script. Only trusted public HTTPS is supported for first release; the
  Agent hostname must be reachable outbound from Managed Nodes.
- **Delivery**: `/agent/v1` polling with ETag; complete Desired State envelopes encrypted
  to the node's public key and signed by Core (ADR-0011); jitter and bounded backoff toward
  the 30-second discovery target. Envelope protocol v1 is extended only with compatible
  fields, and cross-language vectors are regenerated for any change.
- **Agent (slice-1 subset)**: enrollment, ETag polling, envelope verification and
  decryption, validation, staging, and an atomic switch of the `current/` Materialized
  Bundle under the blueprint's node layout. Activation performs `none` only; `reload` and
  `restart` actions and the systemd adapter are deferred. On failure the previous revision
  stays active. Drift detection is deferred.
- **Web (slice-1 subset)**: login and bootstrap screens, Applications/Environments/Secrets/
  Draft/File Binding screens, minimal Node Groups screen, and a Nodes screen with the
  one-time Install Command presentation. TanStack Query owns server state; React Hook Form
  and Zod own forms; Zustand owns only shared client state. The deferred Web test
  dependencies (Vitest, React Testing Library, MSW, Playwright, Zustand, React Hook Form,
  Zod) land with this slice per the implementation plan.
- **Audit**: every security-relevant state change appends an Audit Event in the same
  transaction (ADR-0008); Audit Events never contain Secret values or key material.
- **Trust surfaces**: management `/api/v1` (cookie sessions) and agent `/agent/v1` (mTLS)
  stay separate (ADR-0010); the trusted proxy forwards the client-certificate serial only
  from configured CIDRs, reusing the existing skeleton and its fail-closed middleware.
- **API contract**: OpenAPI is the contract source for the slice's management endpoints;
  generated TypeScript transport types and contract tests are updated together.

## Testing Decisions

- **What makes a good test**: assert observable external behavior only — HTTP status and
  bodies, rendered screens, files on disk, process outcomes — never internals. Negative
  paths (rejected Token, wrong key, modified envelope, conflict, redaction) are as
  important as happy paths. Secret plaintext must appear in no test fixture, snapshot, log,
  or trace.
- **E2E seam (new, the single behavioral proof)**: a Compose-stack acceptance suite driven
  by Playwright plus a Managed Node fixture (a container in CI, a temporary directory
  locally) that executes the real Install Command. The full story runs: bootstrap → login →
  create Application/Environment/Secret → Publish → Assign → generate Install Command →
  enroll node fixture → poll → assert Secret files land in `current/` with declared
  ownership and mode. This seam proves the user-visible promise end to end.
- **Core seam (prior art: `core/internal/server/server_test.go`, `interop_test.go`)**: Go
  integration tests through the management and Agent HTTP interfaces against real
  PostgreSQL, per the plan's "test through Module Interfaces" rule. Covers authorization,
  Bootstrap Code single-use, Token expiry/reuse, ETag conflicts, envelope verification
  failure, and the audit-redaction negative test.
- **Agent seam (prior art: `agent/tests/test_vectors.py`, `interop.py`)**: pytest enrolling
  against a test Core instance, polling, and materializing into a temporary bundle
  directory. Covers wrong-key, modified-ciphertext, modified-manifest, expired-envelope,
  and revoked-certificate fail-closed cases.
- **Web seam (new infrastructure, mandated by the plan)**: Vitest + React Testing Library +
  MSW for feature hooks and components (form validation, conflict display, Install Command
  shown once, no Secret value in browser storage after dismissal); Playwright covers
  bootstrap, login, logout, and the install-command flow against the running stack.
- **Cross-language envelope vectors**: regenerated and checked in for any protocol v1
  extension, keeping the existing Go/Python round-trip tests green.
- The plan's common definition of done applies: lint, formatting, type checking, unit,
  integration, and production builds pass in CI; user-visible async paths handle loading,
  empty, error, retry, success, and stale-data states; migrations run from empty and from
  the prior phase.

## Out of Scope

- TOTP, recovery codes, and Step-up Authentication; Viewer role.
- Rollback, Drift detection and restoration, Retire/Purge, Reveal.
- `reload`/`restart` Activation actions and the systemd adapter; Last Known Good
  restoration beyond keeping the previous revision on failure.
- Node Group overlap/ambiguity rules; Unassignment, Decommissioning, Emergency Revocation.
- Agent update approval and signed updates (ADR-0007 mechanics apply to the install path
  but update management is a later phase); Alerts and Webhooks.
- Recovery Bundles, legacy import/export, multi-Organization, environment-variable
  injection, templates, Windows/macOS/container targets, and Overview/Audit search screens.
- Any UI polish or information architecture beyond the screens listed in this spec.

## Further Notes

- The three temporary gaps recorded in the implementation plan ("Confirmed first vertical
  slice") are normative here: Step-up deferred, Activation `none` only, ambiguity rules
  deferred. Each has an explicit landing phase.
- Convergence is pull-only; Managed Nodes expose no inbound ports (ADR-0010).
- The Install Command is presented once with a copy affordance and is never re-displayed;
  regenerating it issues a fresh Token and invalidates the old one.
- Development fixtures use synthetic Secret values; a secret scan guards the repo and CI.
- The control-plane spec remains the product-level contract; where it conflicts with this
  spec on timing, this spec's slice boundary governs until the deferred items land.

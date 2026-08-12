# AutoSecrets Implementation Plan

## Objective

Deliver the confirmed first-release architecture as evidence-backed vertical slices. A phase is complete only when its exit evidence exists; feature count alone is not completion.

## Delivery principles

- Build end-to-end slices through Web, Core, PostgreSQL, and Agent instead of completing one tier in isolation.
- Keep plaintext Secret fixtures out of source control, snapshots, logs, screenshots, traces, and CI artifacts.
- Treat OpenAPI and the versioned Agent envelope as contracts with compatibility tests.
- Test through Module Interfaces. Do not create shallow repository layers solely for mocking.
- Use real PostgreSQL in integration tests and local-substitutable filesystem/systemd Adapters for Agent tests.
- Require an Audit Event in the same transaction as every security-relevant state change.
- Add no deferred first-release capability until the confirmed path is working and measured.

## Target repository layout

```text
/
  api/
    openapi.yaml
    agent-envelope/              protocol schema and cross-language vectors
  core/
    cmd/autosecrets-core/
    internal/
      identity/
      secretcontrol/
      fleetcontrol/
      delivery/
      convergence/
      audit/
      alerting/
      recovery/
    migrations/
  agent/
    src/autosecrets_agent/
      enrollment/
      sync/
      materializer/
      drift/
      lifecycle/
    tests/
    packaging/
  web/
    src/
      components/ui/             Watermelon UI and shadcn registry source
      components/<feature>/
      hooks/<feature>/use-*.ts
      lib/api/                    generated transport only
      lib/constants/
      lib/env/
      lib/utils/
      routes/
      stores/
  deploy/
    compose.yaml
    caddy/
    installer/
  docs/
```

Existing Web code moves toward this layout incrementally. Do not rewrite generated registry components or global tokens until a real screen requires them.

## Common definition of done

Every phase must satisfy the applicable items below:

- Lint, formatting, type checking, unit, integration, and production build commands pass in CI.
- New Interfaces have success, authorization failure, validation failure, concurrency, and I/O failure tests.
- API changes update OpenAPI, generated TypeScript transport types, and contract tests together.
- Security-relevant operations append redacted Audit Events and have a negative test proving Secret bytes do not enter logs or errors.
- User-visible async paths handle loading, empty, error, retry, success, and stale-data states.
- Migrations run from an empty database and from the prior phase's schema.
- Documentation records operational prerequisites and recovery behavior.
- `git diff --check`, secret scanning, dependency scanning, and the phase-specific acceptance suite pass.

## Phase 0: Foundations and risk spikes

### Deliverables

- Establish the root Go module, Python Agent package, existing Bun Web workspace, embedded SQL migrations, and Compose development environment.
- Add CI jobs for Go, Python, Web, OpenAPI, Compose smoke tests, secret scanning, and artifact checks.
- Add the missing Web dependencies and scripts required by [frontend-guidelines.md](./frontend-guidelines.md): Zustand, React Hook Form, Zod, Vitest, React Testing Library, MSW, and Playwright.
- Add a minimal Core process with separate internal management and Agent routes, PostgreSQL readiness, and version reporting.
- Prove two-hostname Caddy routing on port 443, including optional certificate enrollment and mandatory post-enrollment mTLS identity forwarding from a trusted proxy only.
- Select a reviewed interoperable node-envelope library after a focused Go/Python spike. Check in protocol version 1 schema, known-answer vectors, wrong-key cases, signature failures, and expiry cases.
- Prove the chosen Python packaging tool can create signed self-contained x86_64 and arm64 artifacts that start on the oldest supported Linux baseline.

### Exit evidence

- One browser request reaches the management health route through its hostname.
- One test Agent reaches the Agent route through mTLS, while a browser session and an untrusted proxy header are rejected.
- Go encrypts and signs an envelope that Python decrypts and verifies, and the reverse test also passes where required by the library.
- CI builds reproducible Agent artifacts and verifies their signatures before execution.
- No product CRUD is started until these proofs pass.

## Phase 1: Secure Core bootstrap and identity

### Deliverables

- Load and validate PostgreSQL, trusted-proxy, hostname, cookie, and key-file configuration.
- Generate or mount separate data master, Agent CA, and Core signing keys with strict startup permission checks.
- Implement the Identity Module: one-time bootstrap code, first Administrator creation, password hashing, TOTP setup, recovery codes, secure sessions, logout, Viewer role, and Step-up Authentication grants.
- Implement the Audit Module and transaction integration before adding privileged mutations.
- Add host-local Core commands for Administrator password/TOTP recovery, with Audit Events.
- Build the Watermelon-based login, TOTP enrollment, recovery-code, and operational shell screens.

### Exit evidence

- A fresh Compose deployment has no default credential and can bootstrap exactly once.
- Administrator and Viewer permissions are tested across management routes.
- Session fixation, CSRF, expired TOTP, reused recovery code, and expired Step-up grant tests pass.
- Playwright covers bootstrap, login, TOTP challenge, logout, and Viewer denial.
- Audit rows contain actor, action, resource, result, correlation ID, and timestamp without passwords, TOTP seeds, or recovery codes.

## Phase 2: Secret authoring and Desired State

### Deliverables

- Implement Application, Environment, Secret, opaque Secret Version, File Binding, Draft, Bundle Revision, and Rollback behavior in the Secret Control Module.
- Encrypt each Secret Version before persistence using a versioned AEAD record and wrapped per-version data key.
- Validate relative POSIX paths, duplicates, reserved names, uid/gid declarations, and the safe mode allowlist.
- Implement optimistic Draft concurrency with ETags and metadata-only conflict diffs.
- Implement Node Groups, explicit membership, Assignments, and ambiguity rejection in the Fleet Control Module.
- Add Retire and Step-up-protected Purge with reference checks and tombstone Audit Events.
- Build Applications, Environments, Secret editor/file upload, File Bindings, Draft diff, Publish, Rollback, Node Groups, and Assignment screens.

### Exit evidence

- Database inspection proves Secret plaintext does not appear in rows, indexes, migrations, logs, traces, or Audit Events.
- Concurrent Draft edits produce one success and one explicit conflict, never a silent overwrite.
- Environment isolation and overlapping-group assignment constraints are enforced in PostgreSQL-backed race tests.
- Reveal uses a non-cacheable mutation, requires Step-up Authentication, is unavailable to Viewers, and leaves no browser-storage or query-cache copy after dismissal.
- Playwright covers create Application/Environment, add Secret bytes, publish, inspect revision history, conflict, rollback, retire, and purge denial while referenced.

## Phase 3: Agent enrollment and secure delivery

### Deliverables

- Implement hashed, single-use, ten-minute Enrollment Tokens and Step-up-protected install-command generation.
- Implement Agent local key generation, key-possession proof, node registration, short-lived certificate issuance, renewal, and revocation.
- Implement the signed installer: pinned release URL, signature verification, filesystem setup, systemd unit installation, enrollment, and temporary Token cleanup.
- Implement the Delivery Module and `/agent/v1` polling with ETag, complete Desired State envelopes, per-node encryption, and Core signatures.
- Implement the Agent Enrollment and Sync Modules with jitter, bounded backoff, certificate renewal, signature verification, decryption, and protocol-version rejection.
- Use a fake Materializer in this phase so trust and delivery failures are isolated from filesystem behavior.

### Exit evidence

- A Token cannot be reused, used after expiry, exchanged for a second node, or recovered from Core storage.
- A node certificate cannot read another node's envelope or report as another node.
- Proxy, wrong-key, modified-ciphertext, modified-manifest, expired-envelope, and revoked-certificate tests fail closed.
- An enrolled test Agent observes a newly published ETag within the 30-second target without exposing Bundle plaintext at Caddy or in Core logs.
- Playwright covers install-command creation and the node becoming enrolled without displaying the Token again.

## Phase 4: Atomic Activation and Convergence

### Deliverables

- Implement the Materializer with staging directories, descriptor-based no-follow writes, ownership/mode checks, atomic `current` switch, allowed systemd actions, active-unit verification, and Last Known Good restoration.
- Retain only current and previous successful plaintext revisions.
- Implement Activation stage reporting, heartbeat, offline status, desired-versus-observed state, and independent node Convergence.
- Implement Drift hashing, automatic restoration, repeated-Drift detection, and Alerts.
- Build Overview, node detail, convergence timeline, Activation error, offline, Drift, and explicit Rollback experiences.

### Exit evidence

- Tests cover absolute path, `..`, symlink race, duplicate binding, unknown uid/gid, unsafe mode, disk full, interrupted write, failed rename, reload failure, restart failure, and inactive unit.
- Every failure leaves either the new verified revision active or the Last Known Good Revision active; it never leaves a partial tree current.
- A Core outage and restart do not alter local files, and the Agent later resumes from its ETag.
- Two nodes may report different observed revisions during partial failure without Core claiming global success.
- End-to-end tests cover successful Publish, one-node failure, explicit Rollback, local Drift, and offline recovery.

## Phase 5: Managed operations

### Deliverables

- Implement Administrator-approved Agent release targeting, signed download verification, atomic executable replacement, startup health, and binary rollback.
- Implement Unassignment with configured unit stop and Materialized Bundle removal.
- Implement two-phase Decommissioning and immediate Emergency Revocation with unconfirmed-cleanup state.
- Implement optional Secret Version expiry, advance Alert windows, and no automatic destructive action at expiry.
- Implement in-app Alert lifecycle and signed Webhook delivery with retry, backoff, deduplication, rotation of signing secrets, and delivery history.
- Build Agent update, Unassignment, Decommissioning, Emergency Revocation, expiring Secret, Alert center, and Webhook settings workflows.

### Exit evidence

- A corrupted, unsigned, wrong-architecture, or unhealthy Agent update restores the previous executable.
- Normal Decommissioning cannot report success before cleanup acknowledgement; Emergency Revocation cannot claim cleanup occurred.
- Unassignment failure remains visible and does not silently remove the Assignment record.
- Webhook signatures, replay protection, timeout, retry exhaustion, redaction, and endpoint-disable behavior are tested.
- Playwright covers approved update, failed update, Unassignment confirmation, normal Decommissioning, Emergency Revocation, and Alert acknowledgement.

## Phase 6: Migration, deletion, and disaster recovery

### Deliverables

- Implement guided legacy age/TOML import with explicit identity, Application/Environment target, path review, File Binding ownership, candidate-to-Version mapping, and manual current-Version selection.
- Implement read-only legacy export without a remote write path.
- Complete Retire/Purge lifecycle and background encrypted-blob cleanup while preserving value-free tombstones.
- Implement streaming age Recovery Bundle creation, integrity manifest, restore into an empty deployment, certificate/key restoration, and automated verification.
- Add documented quarterly recovery-drill procedure and failure reporting.

### Exit evidence

- Legacy traversal paths, malformed TOML, invalid age identity, duplicate paths, and ambiguous candidates are rejected without partial import.
- Importing all candidates and selecting one produces the expected first Bundle Revision; export can be consumed by the legacy CLI fixture.
- A Recovery Bundle restores users, TOTP state, metadata, decryptable Secret Versions, Agent identity history, Audit Events, and signing capability into an empty environment.
- A database-only backup and a wrong age identity both fail with explicit recovery diagnostics.
- The restore verification runs automatically and does not count archive creation alone as a successful backup.

## Phase 7: Hardening and first release

### Deliverables

- Perform a threat-model review covering browser leakage, Core memory, PostgreSQL compromise, proxy trust, Agent root compromise, enrollment replay, update supply chain, Webhook SSRF, and backup loss.
- Fuzz path normalization, envelope parsing, TOML migration, HTTP request decoding, and Agent report decoding.
- Run dependency, container, secret, license, and static security scans; produce an SBOM for Core and Agent artifacts.
- Run the full browser and system suites against release Compose images and signed Agent artifacts.
- Load-test 100 jittered Agents, 100 Applications, and 10,000 active Secrets; demonstrate the 30-second discovery target and bounded PostgreSQL growth.
- Verify accessibility, responsive layout, keyboard operation, Secret redaction, and non-overlap across supported desktop and mobile viewports.
- Publish install, upgrade, rollback, backup, restore, key-loss, certificate-rotation, incident, and decommission runbooks.

### Exit evidence

- No unresolved critical or high security finding remains; accepted lower risks have owners and documented rationale.
- All common and phase-specific quality gates pass from a clean checkout.
- Core images and Agent artifacts are versioned, signed, reproducible where supported, and pinned by deployment manifests.
- A clean operator walkthrough completes bootstrap, Publish, enrollment, Activation, Drift recovery, Rollback, update, Decommissioning, backup, and restore using only published documentation.

## Release acceptance scenarios

The first release is not complete until all scenarios pass end to end:

1. Bootstrap a new single-Organization Core with no default credential.
2. Create an Application and production Environment, upload opaque Secret bytes, define safe File Bindings, and Publish a revision after Step-up Authentication.
3. Generate one install command, enroll one Linux node, and activate files with expected ownership and mode.
4. Publish a second revision to two nodes where one systemd action fails; preserve its Last Known Good Revision while the other converges.
5. Roll back explicitly and observe both nodes converge through the normal delivery path.
6. Modify a managed file locally and observe Drift detection, restoration, Audit Event, and Alert/Webhook delivery.
7. Approve a signed Agent update, then reject a modified artifact and retain the old executable.
8. Unassign one Application, Decommission one healthy node, and Emergency Revoke one unreachable node without overstating cleanup.
9. Import a legacy age file, export a compatible read-only file, create a Recovery Bundle, destroy the test deployment, and restore it successfully.
10. Repeat the polling path with 100 Agents while meeting the 30-second discovery target and redaction checks.

## Deferred after first release

- OIDC, Passkeys, Service Accounts, CI/CD write tokens, and fine-grained per-Environment RBAC.
- Provider-specific automatic rotation and revocation.
- Canary or batch rollout orchestration.
- Kubernetes, Windows, macOS, containers, environment-variable injection, templates, and arbitrary hooks.
- Multiple active Core instances, PostgreSQL HA orchestration, external KMS/Vault providers, and SIEM-specific integrations.
- Dynamic node selectors, inherited Environments, shared Secret Versions, and cross-Organization tenancy.

Any deferred item that changes a confirmed trust boundary or domain invariant requires a new grilling decision and, when appropriate, a superseding ADR.

## Phase 0 implementation status

Recorded at the end of the Phase 0 implementation session so remaining work is
explicit rather than implicit.

### Completed

- Root Go module (`core/`) with `internal/envelope`, `internal/config`,
  `internal/server`, `cmd/autosecrets-core`, and a multi-stage Dockerfile.
- Python Agent package (`agent/`) with the envelope module, interop helper, a
  minimal `verify-envelope` CLI, pytest suite, and an editable-install
  `pyproject.toml` (age 0.5.1 pinned).
- Agent envelope protocol v1: schema doc, Go reference implementation, Python
  implementation, checked-in known-answer vectors produced by both languages,
  and a live cross-language round-trip test in both directions.
- Core HTTP skeleton: separate management and Agent bases, health and version
  endpoints, trusted-proxy middleware that fails closed, and full route tests.
- Canonical JSON contract matching Go's `encoding/json` string escaping, with
  cross-language literal tests on both sides.
- Compose bundle (PostgreSQL, Core, Caddy), dual-hostname Caddyfile with
  required client certificate verification and serial forwarding, root
  `.gitignore`, and a CI workflow covering Go, Python, Web, Compose config
  validation, secret scanning, and Go vulnerability scanning.
- Two Web scaffold fixes: the `button.tsx` variant typo and the two
  react-refresh lint suppressions on shadcn registry exports.

### Remaining (explicit deferrals)

- Web phase-0 dependencies (Zustand, React Hook Form, Zod, Vitest, React
  Testing Library, MSW, Playwright) are not installed yet. They land with the
  first Web test slice in Phase 1 rather than as speculative dependencies.
- PostgreSQL readiness/migration wiring inside Core is deferred to Phase 1;
  Compose currently orders Core after a Postgres healthcheck.
- Real Caddy mTLS end-to-end verification (browser rejected, Agent accepted,
  untrusted proxy header rejected over actual TLS) requires a live Compose
  environment and is not yet automated.
- Signed, self-contained Python Agent artifacts for x86_64/arm64 and the
  corresponding CI artifact/verify job are not built yet.
- OpenAPI contract and artifact-check CI jobs are not yet present because
  their inputs (management API surface, release artifacts) do not exist.
- The Compose CI job validates configuration only; a full stack smoke test is
  deferred until the Core image and Caddy mTLS path are exercised together.

## Confirmed first vertical slice (grilled decisions)

Recorded after the grilling session that confirmed the first end-to-end
vertical slice; it supersedes any phase ordering that would build one tier in
isolation.

### Slice boundary

1. First boot: Core prints a one-time Bootstrap Code to its logs; the browser
   consumes it to create the first Administrator (no default credential).
2. Password login and session; TOTP, recovery codes, and Step-up Authentication
   are deferred (see gaps below).
3. Authoring path: create Application and Environment, upload opaque Secret
   bytes, auto-create a default File Binding (filename equals the Secret name,
   path inside the bundle directory, editable or removable), edit in a Draft,
   then Publish.
4. Minimal Node Group (name plus explicit members) and Assignment; overlap
   conflict rules are deferred.
5. Install Command:
   `curl -fsSL https://<agent-host>/install.sh | sudo bash -s -- --server <url> --token <token>`
   The script embeds the Core signing public key, downloads the signed Agent
   artifact and signature from the same origin, and verifies before executing.
   The Agent binary is hosted by Core. See ADR-0013.
6. Enrollment consumes the ten-minute, single-use Token; the Agent begins ETag
   polling toward the 30-second discovery target.
7. Activation writes files only: the Materialized Bundle is staged and switched
   atomically to `current/`; no service actions in this slice.

### Temporary gaps recorded (not silent deferrals)

- TOTP, recovery codes, and Step-up Authentication are deferred. Until they
  land, Publish and install-command generation do not require Step-up
  Authentication; this is a documented temporary weakening of Phase 1/3 exit
  evidence.
- Activation performs `none` only. `reload`/`restart` actions and the systemd
  adapter are deferred to the Materializer slice (Phase 4).
- Node Group overlap conflict rejection is deferred; a duplicate Assignment
  fails with a plain error until the ambiguity rules land.

### Confirmed unchanged

- Agent convergence is pull-only over outbound connections; Managed Nodes
  expose no inbound ports (ADR-0010).
- Envelope encryption per node and Core signatures on delivery (ADR-0011).
- Managed Node layout and Materialized Bundle location per the blueprint.

## First vertical slice implementation status

Recorded at the end of the implementation session for
`.scratch/first-vertical-slice/spec.md`.

### Completed

- Core: PostgreSQL schema and embedded migrations; master key, internal Agent
  CA, and Ed25519 signing key management outside PostgreSQL (ADR-0003/0006);
  argon2id password hashing; bootstrap code lifecycle; secure-cookie sessions
  with double-submit CSRF; append-only Audit Events in the same transaction as
  security-relevant changes (ADR-0008).
- Core Secret Control: Applications, Environments, Secrets with encrypted
  immutable Secret Versions (wrapped per-version data keys), default File
  Binding (filename equals Secret name, 0400) with path/mode validation,
  Drafts with optimistic concurrency (ETag/If-Match), Publish, immutable
  Bundle Revisions.
- Core Fleet: Node Groups with explicit membership, Assignments with
  duplicate rejection, node status reporting.
- Core Enrollment and Delivery: hashed single-use ten-minute Enrollment
  Tokens, CSR-based certificate issuance, per-node age-encrypted and Core-
  signed envelopes (ADR-0011) served by ETag polling, activation reports and
  heartbeats, the signed installer artifact service and install script with
  embedded Core signing key (ADR-0013), public CA endpoint.
- Agent: enroll/sync/serve CLI, node-local age + mTLS identity, files-only
  Materializer with staging and atomic current switch, Last Known Good
  previous revision on failure, manifest hash verification, bounded backoff.
- Web: bootstrap, login, Applications/Environments/Secrets editor with binding
  editing, rotate, Draft and Publish, Revisions list, Nodes screen with
  one-time Install Command, Node Groups and Assignments.
- Seams: Go integration tests through both HTTP surfaces against real
  PostgreSQL (8 tests); pytest integration against a real Core behind the
  devproxy that mirrors the Caddy mTLS contract (35 tests); Vitest/RTL/MSW
  component tests (4 tests); the Playwright E2E proves bootstrap → author →
  publish → assign → real install.sh on a node fixture → secret files land
  with declared mode, token never re-displayed.

### How to run

- Go integration tests: start the test container
  `docker run -d --name autosecrets-test-pg -e POSTGRES_DB=autosecrets -e POSTGRES_USER=autosecrets -e POSTGRES_PASSWORD=test -p 55433:5432 postgres:17-alpine`,
  then `go test ./...` in `core/` (skips when PostgreSQL is unreachable).
- Agent tests: same container, then `.venv/bin/python -m pytest tests/` in
  `agent/` (integration suite skips without the container).
- E2E: `scripts/run-e2e.sh` (starts Core, devproxy, vite, and Playwright).


## Frontend guidelines refactor (grilled decisions)

Recorded after the grilling session that confirmed the frontend refactor
against [frontend-guidelines.md](./frontend-guidelines.md).

- Full refactor of the Web app: kebab-case files, feature-folder Hooks
  (`src/hooks/<feature>/use-*.ts`), all forms on React Hook Form + Zod,
  Zustand session store for the CSRF token, route-level plus feature-level
  Error Boundaries, centralized `src/router/index.ts` data router with
  per-route `React.lazy`, constants under `src/lib/constants/`, and minimal
  component splits.
- Watermelon UI adoption: the correct registry host is
  `https://registry.watermelon.sh` (the `ui.watermelon.sh` host serves SPA
  HTML and was the source of the earlier false negative). Button, Input,
  Select, Table, Badge, Alert, Card, Label, and Textarea were installed from
  `registry.watermelon.sh/r/*.json` (number-variant manifests pull the full
  implementations), the shadcn fallbacks were replaced, and the pages now use
  the components instead of plain HTML. The readiness gate in
  `scripts/watermelon_registry_check.py` points at the correct host and
  reports READY.
- The readiness condition is now operationalized: `scripts/watermelon_registry_check.py`
  (with unit tests) passes only when the registry serves a JSON manifest,
  and `scripts/run-e2e.sh`-independent CI job reports it as a non-blocking
  check. As of the adoption spec implementation, the gate reports NOT READY,
  so the migration has not started and the fallback remains in force.
- Acceptance: lint, typecheck, build, Vitest, Agent integration, and the
  Playwright E2E stay green throughout; UI behavior is unchanged.

## Test unification and API contract session (grilled decisions)

Recorded after the grilling session that confirmed the backend test
unification and API contract work; it supersedes no earlier phase ordering.

### Scope

- This session covers test infrastructure unification and the management API
  contract only. Module layout (identity/secretcontrol/...), TOTP, recovery
  codes, Step-up Authentication, Reveal, Retire/Purge, Alerts/Webhooks, and
  explicit Rollback remain deferred to their grilling sessions.

### Decisions

- Go test infrastructure: shared `internal/testutil` harness; every test
  creates a uniquely named PostgreSQL database on the shared test server
  (parallel-safe across packages), fails (never skips) when PostgreSQL is
  unreachable, and drops the database on cleanup. testcontainers-go startup
  is deferred until the module proxy is reachable; the hook is documented in
  `testutil.Connect`.
- Test organization: `app_test.go` split by domain (auth, secrets, fleet,
  enroll, delivery, smoke) sharing one harness; store-level direct tests
  added only where the invariant is inherently racy (Draft optimistic-lock
  conflict, single-use Enrollment Token, expired Token, empty Draft Publish)
  or for CRUD lifecycle coverage.
- Single entry point: `scripts/test-all.sh` starts/reuses the shared
  `autosecrets-test-pg` container, exports `TEST_DATABASE_URL`, and runs
  Core (vet + tests + coverage gate), Agent pytest, and Web (lint + vitest +
  build). CI runs the same commands with the same environment.
- Coverage gate: Core product packages (excluding `cmd/autosecrets-core` and
  `internal/testutil`) must reach 70% statement coverage; enforced by
  `scripts/coverage_gate.py` in CI.
- API contract: `api/openapi.yaml` documents the management surface v1 with
  the error model from ADR-0014 (`{error, code}` with a stable enum).
  redocly lint runs in CI; every test response is validated against the spec
  with kin-openapi (contract tests); `openapi-typescript` generates
  `web/src/lib/api/generated.ts`, committed and freshness-checked in CI.
- Pagination: list endpoints stay unpaginated for v1 (documented in the
  spec); `audit-events` keeps its `limit` parameter because Audit Events grow
  without bound.

### Implementation status

Completed:

- `core/internal/testutil` shared harness (unique per-test database, key
  material, truncate).
- `app_test.go` split into auth/secrets/fleet/enroll/delivery/smoke test
  files sharing the harness.
- Store concurrency and lifecycle tests; two latent store bugs found and
  fixed (`CreateAdmin` missing unique-violation mapping, `SessionByID`
  missing no-rows mapping).
- Unified error envelope with `writeError` and stable codes across all
  management handlers (ADR-0014).
- `api/openapi.yaml` (26 operations, response schemas, error model, audit
  `limit`), redocly lint clean, kin-openapi contract validation wired into
  the test harness, `web/src/lib/api/generated.ts` generated and committed.
- `scripts/test-all.sh` + `scripts/coverage_gate.py`; CI updated (PostgreSQL
  service container, coverage gate, vitest, redocly, type freshness).

Deferred (explicit):

- testcontainers-go automatic container startup (module proxy unreachable
  during this session; hook documented in `testutil`).
- Web migration of hand-written API types onto `generated.ts`: blocked on
  the in-flight frontend refactor; the generated file and freshness check
  are already in place.
- Web lint/test/build in the unified script currently fail on the
  uncommitted in-flight frontend work (Chinese component names); CI runs on
  clean checkouts and is unaffected.

## Rotatable Secrets (grilled decisions, old-project parity)

Recorded after the session that aligned the Web UI's Secret behavior with
the legacy personal tool (git.kmou424.moe:8443/kmou424/autosecrets):

- Landing path parity: Materialized Bundles default to `~/.autosecrets` in
  the invoking user's home, exactly like the legacy tool (the `--bundle-dir`
  argument / `bundle_dir` request field overrides; "~" expands to the sudo
  caller's home). Agent identity stays in `/var/lib/autosecrets-agent/`.
- Core-driven rotation: a Secret's versions are its candidate list. Nodes
  keep the value they already activated across ordinary Publish cycles
  (never disturbed while still a candidate). An explicit rotate
  (`POST /secrets/{id}/rotate`, "下一候选" in the UI) marks the next
  candidate as the pending target; nodes are forced onto it on the next
  poll, then keep-old-value resumes. Rotation targets persist in
  `secret_rotations` (migration 0002).
- Web UI: per-Secret "下一候选" button (visible once a Secret has multiple
  versions) and an optional deployment-path input on the Add Server card.

### Implementation status

Completed: store queries (`RevisionVersionMap`, `SecretVersionSeqs`,
`SecretAppEnv`, `MarkRotation`, `PendingRotation`), delivery keep-old-value
and forced-rotation logic, rotate endpoint with audit, migration 0002,
OpenAPI contract + generated types, Web UI controls, Go tests
(keep/force lifecycle, validation) and contract tests all green.

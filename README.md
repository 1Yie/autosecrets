# AutoSecrets

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Self-hosted secret management for one Organization and one Administrator.

Core holds Desired State. Agents on Managed Nodes pull assigned Secret Bundles over mTLS, write them to disk, and report what actually landed. A Draft does nothing until you Publish it into a Bundle Revision.

Repository: [github.com/1Yie/autosecrets](https://github.com/1Yie/autosecrets)

## How it fits together

```text
 Administrator
      |
      |  browser
      v
     Web  ---- /api/v1 ---->  Core (Go)
                                |
                     PostgreSQL + keys volume
                                ^
                                |  mTLS
                                |
                          Agent on each
                          Managed Node
```

| Piece | Role |
| --- | --- |
| **Web** | React console. Talks only to the Management API. |
| **Core** | One Go process: identity, secrets, fleet, audit, Agent API, built Web assets. |
| **PostgreSQL** | The only database. |
| **Agent** | Python process on the node. Enrolls once, then converges independently. |

Master key material stays on disk outside PostgreSQL (`CORE_KEYS_DIR`).

## What it does

- One Administrator per Organization. Local username and password stay available.
- Optional local TOTP. Enabling it is a full enrollment; disabling it deletes the TOTP credential and recovery codes.
- Independent OAuth and OpenID Connect bindings. Each is configured separately, bound separately, and can be used to log in only after an explicit bind. Unconfigured providers stay off the login page.
- Draft → Publish → Bundle Revision. Rollback writes a new revision from an earlier snapshot; it does not rewrite history.
- Node Groups, Assignments, and two-phase Unassignment (stop the Application, then remove the Materialized Bundle).
- Per-node poll interval and per-assignment convergence. A failed node keeps its Last Known Good Revision.
- Append-only Audit Events with no Secret values in the payload.

## Quick start (development)

Requires Docker, Go 1.24+, and [Bun](https://bun.sh).

```bash
git clone https://github.com/1Yie/autosecrets.git
cd autosecrets
./scripts/dev-up.sh
```

This starts:

- PostgreSQL on `localhost:55434`
- Core on `http://127.0.0.1:18080`
- Web on `http://127.0.0.1:5199`

First boot prints a one-time Bootstrap Code in `.dev/core.log`. Open the Web URL, redeem the code, and create the Administrator.

```bash
./scripts/dev-down.sh
```

`./scripts/dev-up.sh --lan` also exposes the Agent API on the LAN (`https://<lan-ip>:18443`) so other machines can install an Agent.

## Deploy with Compose

The supported install is the Compose bundle in `deploy/`: Core, PostgreSQL, and Web.

```bash
cd deploy
export AUTOSECRETS_DB_PASSWORD='a-long-random-password'
export CORE_PUBLIC_URL='https://autosecrets.example'
export CORE_PUBLIC_AGENT_URL='https://agent.autosecrets.example'
docker compose up -d --build
```

Set `SOURCE_COMMIT` if you want the About dialog to show a specific revision. Core writes keys into the `keys` volume; back that volume up separately from the database.

### Optional identity providers

Neither provider is required. Incomplete configuration disables only that provider; local login still works.

OpenID Connect (discovery + ID Token):

| Variable | Purpose |
| --- | --- |
| `CORE_OIDC_ISSUER_URL` | Issuer used for discovery |
| `CORE_OIDC_CLIENT_ID` | Client id |
| `CORE_OIDC_CLIENT_SECRET` | Client secret |
| `CORE_OIDC_SCOPES` | Defaults to `openid profile` |

OAuth 2.0 (authorization code + PKCE + userinfo, no ID Token):

| Variable | Purpose |
| --- | --- |
| `CORE_OAUTH_AUTHORIZATION_URL` | Authorization endpoint |
| `CORE_OAUTH_TOKEN_URL` | Token endpoint |
| `CORE_OAUTH_USERINFO_URL` | Userinfo endpoint (`sub` or `id`) |
| `CORE_OAUTH_CLIENT_ID` | Client id |
| `CORE_OAUTH_CLIENT_SECRET` | Client secret |
| `CORE_OAUTH_SCOPES` | Defaults to `profile` |

`CORE_PUBLIC_URL` must be a canonical origin (`https://…`, or `http://` on localhost). Binding still requires a local password (and TOTP when that policy is on).

## Repository layout

```text
api/openapi.yaml     Management API contract
core/                Go service, SQL, migrations
agent/               Python Agent
web/                 React console
deploy/              Compose bundle
docs/adr/            Architecture decisions
CONTEXT.md           Shared domain language
```

## Development

```bash
# Core (integration tests need the test Postgres from scripts/test-all.sh)
cd core
go test ./...

# Web
cd web
bun install
bun run test
bun run lint
```

Management API changes go through `api/openapi.yaml`, then `bun run gen:api` in `web/`.

Domain words (`Secret`, `Draft`, `Publish`, `Assignment`, `Convergence`, …) are defined in [`CONTEXT.md`](CONTEXT.md). Decision records live in [`docs/adr/`](docs/adr/).

## License

[MIT](LICENSE)

Based on [kmou424/autosecrets](https://git.kmou424.moe/kmou424/autosecrets).

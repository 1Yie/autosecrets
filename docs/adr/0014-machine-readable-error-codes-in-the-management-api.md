# ADR-0014: Machine-readable error codes in the management API

- Status: accepted
- Date: 2026-08-12
- Context: [0010-separate-web-and-agent-trust-surfaces](./0010-separate-web-and-agent-trust-surfaces.md)

## Decision

Every error response of the management API (`/api/v1`) carries the envelope

```json
{ "error": "human-readable message", "code": "machine-readable code" }
```

The `code` field is a stable, closed enum defined in `api/openapi.yaml`:
`bad_request`, `unauthorized`, `forbidden`, `not_found`, `conflict`,
`duplicate`, `internal`, `unavailable`. Clients switch on `code` and never
parse `error`. The contract is enforced by kin-openapi response validation
in the Go test harness and by the generated TypeScript types consumed by the
Web client.

## Consequences

Positive:

- The Web client can distinguish a stale-Draft `409 conflict` from a
  duplicate-name `409 duplicate` and react differently, which message
  sniffing could not do reliably.
- Generated clients (openapi-typescript, future SDKs) get a typed discriminator
  instead of a free-form string.
- The enum is reviewed in one place (`components.schemas.Error`) and can be
  extended deliberately; adding a code requires touching the spec, the
  handler, and the contract tests together.

Negative:

- Two fields must be kept consistent per error site; the `writeError` helper
  and contract tests keep drift from surviving CI.
- RFC 7807 `application/problem+json` was rejected as heavier than the
  problem warrants at this scale (one client, one language, no aggregator
  integration); the plain envelope costs nothing to migrate later if a
  problem-details consumer appears.

## Alternatives considered

- RFC 7807 problem+json: standard, but adds a content type, a `type` URI
  resolution story, and frontend transport changes for no current consumer.
- Plain `{"error": string}` (status quo): no machine-readable signal;
  clients were already beginning to sniff messages.
- No `code` on 5xx: rejected; the audit and operator tooling benefits from a
  stable discriminator even for `internal`.

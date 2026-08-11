# Agent Envelope Protocol v1

Status: phase-0 risk gate

The Agent envelope carries a complete Desired State payload from Core to one
Managed Node. It is JSON, encrypted to the destination Agent's age X25519
public key, and signed by a Core Ed25519 key.

## Envelope JSON

| Field | Type | Meaning |
| --- | --- | --- |
| `protocol` | string | always `autosecrets-envelope` |
| `version` | int | always `1` |
| `node_id` | string | destination Managed Node identity |
| `revision_id` | string | Bundle Revision identity |
| `created_at` | string | RFC 3339 UTC creation time |
| `expires_at` | string | RFC 3339 UTC expiry; empty means no expiry |
| `manifest_sha256` | string | lowercase hex SHA-256 of the canonical manifest JSON |
| `suite` | string | encryption suite; currently only `age-x25519` |
| `ciphertext` | string | base64 age v1 ciphertext of the Secret bundle plaintext |
| `signing_key_id` | string | identifier of the Core signing key |
| `signature` | string | base64 Ed25519 signature |

## Canonical signature payload

The signature covers the JSON object below with **alphabetically sorted keys,
no whitespace, all values strings**, and no `signature` field:

```json
{"ciphertext":"...","created_at":"...","expires_at":"...","manifest_sha256":"...","node_id":"...","protocol":"autosecrets-envelope","revision_id":"...","signing_key_id":"...","suite":"age-x25519","version":"1"}
```

Go produces this with `encoding/json` map marshaling (sorted keys) and Python
with `json.dumps(..., sort_keys=True, separators=(",", ":"),
ensure_ascii=False)` followed by Go-compatible escaping. String escaping must
match Go exactly: `<`, `>`, `&` become `\u003c`, `\u003e`, `\u0026`, and
U+2028 / U+2029 become `\u2028` / `\u2029`; other non-ASCII bytes stay raw.
The `expires_at` value, when present, uses exactly `YYYY-MM-DDTHH:MM:SSZ`
(UTC with a `Z` suffix; other offsets are rejected by both implementations).

## Canonical manifest

The manifest describes the files a Bundle Revision materializes:

```json
{"files":[{"gid":"0","mode":"0400","path":"app/token","sha256":"abc123","uid":"1000"}],"protocol":"autosecrets-manifest","version":"1"}
```

Rules:

- Top-level keys sorted; every file entry has exactly `gid`, `mode`, `path`,
  `sha256`, `uid`, all strings.
- `files` is sorted by `path`.
- `manifest_sha256` in the envelope is the SHA-256 of these exact bytes.

## Verification rules

- Reject unknown `protocol`, `version`, or `suite` before touching the payload.
- Reject an envelope whose `expires_at` is in the past.
- Verify the Ed25519 signature over the canonical payload.
- Decrypt `ciphertext` with the destination Agent's age identity.

## Test vectors

`testdata/envelope-v1-vectors.json` contains known-answer vectors produced by
both the Go reference implementation and the Python Agent implementation, plus
tampered and expired vectors that must fail. Both language test suites consume
the same file.

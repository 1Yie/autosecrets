---
status: accepted
---

# Use one Administrator, optional local TOTP, and OIDC

AutoSecrets permanently supports one Administrator rather than multiple Organization Members. New Organizations allow local username-and-password authentication without TOTP by default; an Organization-wide TOTP Login Policy can strengthen local login, Session renewal, Step-up Authentication, password changes, and credential-management actions without applying an additional AutoSecrets TOTP challenge to OIDC. This keeps a recoverable local login for a personal self-hosted system while allowing an external identity provider to own the authentication strength of its own login path. This decision supersedes ADR-0023 and ADR-0024.

OIDC uses deployment-provided discovery configuration and an explicit public Core URL. The Administrator must first authenticate locally and re-authenticate according to the current TOTP Login Policy before explicitly binding one validated issuer-and-subject identity. OIDC uses Authorization Code with PKCE and standard issuer, audience, signature, time, nonce, and state validation; it stores no provider tokens and issues the same bounded AutoSecrets Session as local authentication. Invalid OIDC configuration disables only OIDC so local recovery remains available.

Local TOTP enrollment is optional and reversible but never dormant: enabling the policy requires a complete new enrollment, while disabling it requires the current password and TOTP, deletes the TOTP credential and Recovery Codes, and revokes other Sessions. Existing confirmed TOTP installations migrate with the policy enabled; an incomplete pending enrollment migrates to an active password-only Administrator with the policy disabled. A database containing more than one human identity cannot be migrated by guessing which identity to retain and must fail authentication startup with an actionable host-local diagnostic.

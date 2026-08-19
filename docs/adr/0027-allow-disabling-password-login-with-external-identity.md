---
status: accepted
---

# Allow disabling password login when an External Identity Binding can log in

ADR-0026 keeps a recoverable local login for a personal self-hosted Organization. Administrators who have already bound a usable OAuth or OpenID Connect identity can now disable the Password Login Policy so username-and-password cannot start a new Session. The password remains the proof for Step-up Authentication and credential-management actions.

This is reversible and fail-open: the policy can be disabled only while at least one External Identity Binding is currently usable for login; unbinding the last usable External Identity Binding is refused until password login is re-enabled; and if every External Identity Provider becomes unavailable, password login is accepted again so a broken provider configuration cannot lock the Administrator out.

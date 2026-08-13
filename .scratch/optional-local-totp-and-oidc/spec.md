# Optional Local TOTP and OIDC for a Single Administrator

Status: ready-for-agent

## Problem Statement

AutoSecrets is a personal, self-hosted system with one Administrator, but its current authentication flow is built around a multi-member model and mandatory TOTP. Bootstrap leaves the first Administrator pending until TOTP enrollment is verified and Recovery Codes are confirmed, and every local login requires a password plus TOTP or a Recovery Code. This makes TOTP mandatory even when the Administrator does not want that operational burden and makes an interrupted enrollment capable of blocking access entirely.

The system also has no external identity login. An Administrator who already operates a trusted OpenID Connect provider cannot use it to authenticate to AutoSecrets, cannot explicitly bind that identity to the local Administrator, and cannot retain the local password as an independent recovery path if the provider is unavailable.

From the Administrator's perspective, local TOTP should be an explicit security choice rather than a prerequisite for using the product. New installations should work with a username and password by default, existing installations must not silently lose an already enabled TOTP requirement, and OIDC should add a second login path without weakening local recovery or pretending that AutoSecrets controls the authentication strength of the external provider.

## Solution

Make the single Administrator the only supported human identity and introduce an Organization-wide **TOTP Login Policy** that is disabled by default for new Organizations. When disabled, local authentication uses the Administrator's username and password. When enabled, local login becomes a two-stage flow: the password is verified first and a short-lived challenge is issued, then the Administrator supplies TOTP or one unused Recovery Code. Session renewal, Step-up Authentication, password changes, and security-setting changes apply the exact proof rules defined below.

Add a configurable OpenID Connect login path using Authorization Code with PKCE. Provider configuration comes from the Core deployment environment and uses an explicit public Core URL. The Administrator must first log in locally, re-authenticate, and explicitly create an **External Identity Binding** to one validated issuer-and-subject identity. Once bound, the login screen exposes OIDC. A successful callback creates the same bounded AutoSecrets Session used by local login; provider tokens are not retained, and signing out ends only the AutoSecrets Session.

Migrate existing identity state conservatively. A confirmed TOTP enrollment keeps the TOTP Login Policy enabled. An incomplete pending enrollment is cleared and the sole Administrator becomes active with the policy disabled. If more than one human identity exists, Core refuses startup with an actionable diagnostic rather than guessing which identity should survive.

## User Stories

1. As a new Administrator, I want Bootstrap to activate my identity after validating the Bootstrap Code, Organization name, username, and password, so that I can start using a personal installation without first configuring TOTP.
2. As a new Administrator, I want the TOTP Login Policy disabled by default, so that the default authentication flow matches the product's personal-use model.
3. As an Administrator, I want to keep a username and password credential, so that I always have a local login path independent of an External Identity Provider.
4. As an Administrator, I want to log in with only my username and password while the TOTP Login Policy is disabled, so that TOTP is not requested on every login by default.
5. As an Administrator, I want the login screen to omit TOTP and Recovery Code controls until they are needed, so that a disabled policy does not look partially configured or broken.
6. As an Administrator, I want to enable the TOTP Login Policy from security settings, so that I can strengthen local authentication when I choose.
7. As an Administrator, I want enabling the policy to require my current password, so that a stolen Session alone cannot add or replace my second factor.
8. As an Administrator, I want enabling the policy to generate a fresh authenticator enrollment, so that no stale or previously abandoned seed becomes active.
9. As an Administrator, I want to scan a standards-compatible TOTP URI, so that I can use a normal authenticator application.
10. As an Administrator, I want to prove the new TOTP seed with one valid code, so that an untested enrollment cannot become mandatory.
11. As an Administrator, I want a fresh set of one-use Recovery Codes when enrollment succeeds, so that I have an offline recovery proof if my authenticator is unavailable.
12. As an Administrator, I want to explicitly confirm that I saved the Recovery Codes, so that the policy is not enabled before the recovery material has been presented once.
13. As an Administrator, I want an interrupted TOTP enrollment to leave the policy disabled, so that an incomplete setup cannot lock me out.
14. As an Administrator, I want a password-only first login step while the policy is enabled, so that the server can validate my primary credential before asking for a second factor.
15. As an Administrator, I want a successful first step to advance to a dedicated TOTP challenge, so that the login state is clear and focused.
16. As an Administrator, I want to use either TOTP or one unused Recovery Code at the second login step, so that loss of the authenticator does not immediately require host access.
17. As an Administrator, I want an invalid or expired login challenge to return me to the password step, so that I never remain in an unusable intermediate state.
18. As an Administrator, I want a consumed Recovery Code rejected on every later attempt, so that each code is truly single use.
19. As an Administrator, I want a successfully used TOTP counter rejected if replayed, so that captured codes cannot be reused within the acceptance window.
20. As an Administrator, I want failed password and second-factor attempts rate limited without permanent lockout, so that brute-force attempts are slowed without making denial of service permanent.
21. As an Administrator, I want login failures to avoid revealing whether the username, password, TOTP, Recovery Code, or External Identity Binding was wrong, so that anonymous users cannot enumerate identity state.
22. As an Administrator, I want to disable the TOTP Login Policy from security settings, so that I can return local authentication to password-only mode.
23. As an Administrator, I want disabling the policy to require my current password and a current TOTP code, so that a stolen Session or Recovery Code alone cannot downgrade login security.
24. As an Administrator, I want disabling the policy to delete the TOTP seed and all Recovery Codes, so that disabled credentials do not remain dormant indefinitely.
25. As an Administrator, I want re-enabling TOTP after disabling it to require a completely new enrollment, so that an old authenticator cannot silently regain authority.
26. As an Administrator, I want enabling or disabling TOTP to revoke every other Session, so that authentication-policy changes take effect across browsers and devices.
27. As an Administrator, I want Session renewal to require my password while the policy is disabled, so that renewal still proves knowledge of the local credential.
28. As an Administrator, I want Session renewal to require my password plus TOTP or a Recovery Code while the policy is enabled, so that renewal preserves the local login assurance level.
29. As an Administrator signed in through OIDC, I want Session renewal to remain possible with my local credential, so that provider availability is not required to preserve or recover local access.
30. As an Administrator, I want Step-up Authentication to require my password while the policy is disabled, so that high-risk operations remain protected from a stolen Session.
31. As an Administrator, I want Step-up Authentication to require my password plus TOTP or a Recovery Code while the policy is enabled, so that high-risk operations reflect my chosen local security level.
32. As an Administrator, I want changing my password to require my current password while the policy is disabled, so that a stolen Session cannot replace my recovery credential.
33. As an Administrator, I want changing my password to require my current password and TOTP while the policy is enabled, so that a Recovery Code cannot be used to replace my primary credential.
34. As an Administrator, I want a password change to revoke prior Sessions and issue one current Session, so that old credentials and browser sessions stop working together.
35. As a deployment operator, I want to configure an OIDC issuer, client ID, optional client secret, scopes, and public Core URL through deployment configuration, so that provider credentials are not managed through an ordinary Web settings API.
36. As a deployment operator, I want Core to use OIDC discovery, so that authorization, token, signing-key, and provider metadata come from the configured issuer.
37. As a deployment operator, I want the redirect URI derived only from an explicit `CORE_PUBLIC_URL`, so that untrusted Host or forwarding headers cannot influence the callback address.
38. As a deployment operator, I want invalid or unavailable OIDC configuration to disable only OIDC, so that Core and local login remain available for diagnosis and recovery.
39. As an Administrator, I want security settings to show whether OIDC is available, unavailable, bound, or unbound, so that I can distinguish deployment configuration from identity binding state.
40. As an Administrator, I want the UI to show an actionable OIDC configuration error only after local authentication, so that operational detail is available without leaking unnecessary anonymous diagnostics.
41. As an Administrator, I want to start External Identity Binding only after local re-authentication, so that a stolen Session cannot install a permanent external login identity.
42. As an Administrator, I want binding to require my current password and, when enabled, my current TOTP, so that a Recovery Code alone cannot create a permanent login path.
43. As an Administrator, I want the binding callback to accept only the issuer and subject returned by the validated OIDC transaction I initiated, so that email matching or a forged callback cannot bind another identity.
44. As an Administrator, I want an External Identity Binding identified by issuer and subject rather than email, so that mutable or recycled email addresses cannot take over my account.
45. As an Administrator, I want binding to revoke every other Session, so that a security-boundary change is reflected across all active browsers.
46. As an Administrator, I want the OIDC login option hidden until a valid provider configuration and an External Identity Binding both exist, so that an unusable external login path is never advertised.
47. As an Administrator, I want OIDC login to use Authorization Code with PKCE, state, and nonce, so that authorization responses cannot be injected, swapped, or replayed across browser transactions.
48. As an Administrator, I want OIDC tokens validated for issuer, audience, signature, lifetime, nonce, and subject, so that only the configured provider and client can authenticate the bound identity.
49. As an Administrator, I want an OIDC callback with an unbound subject rejected generically, so that another valid user at the provider cannot enter my Organization.
50. As an Administrator, I want OIDC login not to request an additional AutoSecrets TOTP, so that the External Identity Provider owns the strength and interaction of its authentication path.
51. As an Administrator, I want the security UI to call the setting “local login requires TOTP,” so that I do not mistake it for a guarantee covering OIDC.
52. As an Administrator, I want a successful OIDC login to issue the same bounded AutoSecrets Session as local login, so that authorization, CSRF protection, idle expiry, and absolute expiry behave consistently.
53. As an Administrator, I want Audit Events and Session records to distinguish local and OIDC authentication, so that I can investigate how access was obtained without changing authorization behavior.
54. As an Administrator, I want AutoSecrets not to persist OIDC access tokens, refresh tokens, or ID tokens, so that it does not retain external credentials it never uses.
55. As an Administrator, I want OIDC scopes limited to those required for authentication, so that AutoSecrets does not request unrelated provider access.
56. As an Administrator, I want signing out to end only my AutoSecrets Session, so that it does not unexpectedly sign me out of unrelated applications at the provider.
57. As an Administrator, I want to unlink an External Identity Binding only after local re-authentication, so that a stolen Session cannot remove my external login path.
58. As an Administrator, I want unlinking to require my current password and, when enabled, my current TOTP, so that the binding has the same protection as its creation.
59. As an Administrator, I want unlinking to revoke every other Session, so that OIDC Sessions created through the removed binding stop working.
60. As an Administrator, I want replacing an External Identity Binding to require unlinking and then binding again, so that identity replacement remains an explicit auditable sequence.
61. As an Administrator upgrading from a confirmed TOTP installation, I want the TOTP Login Policy to remain enabled, so that an upgrade never silently lowers my existing login assurance.
62. As an Administrator upgrading with an incomplete pending TOTP enrollment, I want the abandoned enrollment cleared and my identity activated with the policy disabled, so that the previous mandatory flow no longer leaves me locked out.
63. As an Administrator upgrading from an active password-only identity, I want the policy to remain disabled, so that existing usable credentials continue to work.
64. As a deployment operator, I want Core to refuse startup when more than one human identity exists, so that migration never guesses which identity should control the Organization.
65. As a deployment operator, I want the multi-identity startup failure to state the record count and required host-local remediation without exposing credential data, so that I can resolve the conflict deliberately.
66. As an Administrator, I want all public identity behavior to expose only one Administrator, so that Viewer, invitation, role-management, and member-lifecycle concepts do not remain reachable after the product model changes.
67. As an Administrator, I want existing compatibility tables and columns to remain harmless during this delivery, so that authentication can change without coupling it to a destructive schema cleanup.
68. As an Administrator, I want Bootstrap, local login, OIDC login, renewal, Step-up, policy changes, binding changes, password changes, failures, and migration outcomes represented as value-free Audit Events, so that security changes are investigable without recording secrets.
69. As an Administrator, I want every authentication intermediate cookie to be HttpOnly, Secure on HTTPS, SameSite protected, narrowly scoped, short lived, and cleared after success or failure, so that browser state cannot become a reusable bearer credential.
70. As an Administrator, I want local login and OIDC callbacks to preserve only validated same-origin return destinations, so that successful authentication cannot become an open redirect.
71. As a keyboard user, I want Bootstrap, both local login stages, TOTP enrollment, Recovery Code confirmation, OIDC binding, and policy controls operable without a pointer, so that the security workflow remains accessible.
72. As an Administrator, I want authentication errors announced without moving focus unpredictably, so that I can recover from failed credentials or expired challenges efficiently.

## Implementation Decisions

- **Identity model**: AutoSecrets supports exactly one human identity, the Administrator. Public APIs and Web behavior must not expose Viewer roles, Member Invitations, member lists, role changes, or multi-member lifecycle operations. Existing compatibility storage may remain until a separate cleanup migration.
- **Organization policy**: Persist one Organization-wide boolean TOTP Login Policy. New Organizations default to disabled. The UI label must explicitly describe local login, because OIDC is not challenged by AutoSecrets TOTP.
- **Bootstrap state machine**: Bootstrap validates the one-time Bootstrap Code and creates the active Administrator directly. It no longer starts mandatory MFA enrollment. A Session may be issued immediately after successful Bootstrap.
- **Local login API**: The password endpoint accepts username and password only. With the policy disabled it issues a Session. With the policy enabled it returns a machine-readable “second factor required” result and sets a challenge cookie; it does not issue a partial Session.
- **Second-factor API**: A dedicated endpoint accepts TOTP or a Recovery Code against the challenge cookie. Success consumes the challenge and, for recovery, the Recovery Code, then issues a normal Session. Error responses remain generic.
- **Login challenge**: The challenge expires after five minutes, is single use, and is represented in the browser by an opaque HttpOnly cookie whose value is hash-stored server-side. It is bound to the Administrator and the login transaction's source context. Success, cancellation, terminal failure, and expiry clear it. Attempts are rate limited.
- **TOTP enablement**: Enabling starts a fresh enrollment only after current-password verification. The existing verify-then-confirm Recovery Code flow remains the activation boundary. Until confirmation, the Organization policy remains disabled and the prior usable login state is unchanged.
- **TOTP disablement**: Disabling requires current password and current TOTP, not a Recovery Code. It atomically disables the policy, deletes enrollment material and Recovery Codes, revokes Step-up Grants, revokes all other Sessions, and records an Audit Event. Re-enablement always creates a new seed and Recovery Codes.
- **Proof matrix**: Login and Session renewal require password only when disabled and password plus TOTP or Recovery Code when enabled. Generic Step-up Authentication follows the same matrix. Password change, TOTP disablement, External Identity Binding, and unbinding require current password and additionally current TOTP when enabled; these credential-management actions do not accept Recovery Codes.
- **Session behavior**: Local and OIDC authentication issue the same secure-cookie Session, CSRF token, 12-hour absolute expiry, and 30-minute idle expiry. Session records and Audit Events include an authentication method of `local` or `oidc`. OIDC-created Sessions can be renewed using the local proof matrix.
- **OIDC deployment configuration**: Add explicit configuration for `CORE_PUBLIC_URL`, issuer URL, client ID, optional client secret, and scopes. `CORE_PUBLIC_URL` is a canonical origin used to construct fixed callback URLs and must not be inferred from request headers. Production public URLs require HTTPS; explicit localhost HTTP is allowed for development and tests.
- **OIDC availability**: Core performs discovery and validates provider metadata without making the whole application dependent on provider health. Missing, malformed, or unavailable OIDC configuration produces an unavailable status, disables OIDC routes that initiate authentication, preserves local login, and emits an actionable operational diagnostic without exposing secrets anonymously.
- **OIDC protocol**: Use Authorization Code with PKCE. Every transaction uses cryptographically random, purpose-bound, short-lived state and nonce. Callback validation includes issuer, audience, authorized party where applicable, signature and key selection, expiry, issued-at constraints, nonce, state, code verifier, and exact redirect URI.
- **OIDC identity**: Persist at most one External Identity Binding containing the normalized issuer and subject plus optional non-authoritative display metadata. Email, username, and groups are never identity keys and never cause automatic linking or provisioning.
- **Binding flow**: Binding can start only from an authenticated local Session after fresh re-authentication according to the credential-management proof matrix. The OIDC transaction is marked for binding rather than login. A validated callback stores its issuer and subject atomically, revokes other Sessions, and records an Audit Event.
- **OIDC login flow**: Anonymous OIDC login is advertised only when configuration is valid and a binding exists. A validated callback must exactly match the stored issuer and subject. It then issues a normal AutoSecrets Session without an AutoSecrets TOTP challenge.
- **OIDC token retention**: Request only authentication scopes, do not request `offline_access`, and do not persist authorization codes, access tokens, refresh tokens, or ID tokens after callback validation. Provider claims retained for display must be explicitly allowlisted and non-authoritative.
- **Unbinding and replacement**: Unbinding requires fresh local re-authentication according to the credential-management matrix, deletes the binding, revokes other Sessions, and records an Audit Event. Replacement is the explicit sequence of unbind followed by bind; there is no implicit overwrite.
- **Logout**: Logout deletes the AutoSecrets Session and grants only. It does not call provider logout or redirect to an OIDC end-session endpoint.
- **Anonymous capability response**: The authentication bootstrap/capability response may reveal only whether Bootstrap is required and whether a usable OIDC login button should be shown. Detailed provider, binding, or configuration-error state is available only to the authenticated Administrator.
- **Security settings response**: Authenticated settings expose the TOTP Login Policy state, enrollment state, OIDC availability, binding state, and sanitized OIDC diagnostic. They never return the TOTP seed after initial presentation, Recovery Code hashes, client secret, provider tokens, or raw token claims.
- **Migration policy**: A sole Administrator with confirmed TOTP migrates to policy enabled. A sole active Administrator without confirmed TOTP migrates to policy disabled. A sole pending Administrator with incomplete enrollment becomes active, its incomplete enrollment and Recovery Codes are removed, and the policy becomes disabled.
- **Ambiguous identity migration**: If more than one human identity record exists, Core refuses startup before serving management authentication. The diagnostic reports an ambiguous single-Administrator migration and requires an explicit host-local remediation; it never automatically selects by role, status, or creation time.
- **Compatibility cleanup**: Multi-member tables, role columns, invitation records, and compatibility repository methods may remain physically present but must have no public creation or management path. Their physical deletion is a later migration.
- **Audit behavior**: Record successful and denied local login, second-factor verification, OIDC login, binding, unbinding, policy enablement/disablement, password change, renewal, logout, and migration decisions. Audit payloads contain no password, TOTP seed or code, Recovery Code, OIDC code, token, client secret, or full claims.
- **OpenAPI and generated client**: Update the API description before regenerating the Web client. Machine-readable outcomes distinguish successful Session issuance, second-factor challenge, expired challenge, OIDC unavailable, binding conflict, and re-authentication failure while preserving generic anonymous credential errors.
- **Web authentication flow**: Bootstrap ends in an authenticated Session. Local login starts with username/password and conditionally transitions to a TOTP/Recovery Code view. The OIDC button appears only when the anonymous capability response permits it. Expired challenges return to the first step with focus and an accessible message.
- **Web security settings**: Add one focused security surface for local TOTP enable/disable and External Identity Binding/unbinding. Destructive security changes explain Session revocation and require inline re-authentication rather than relying only on the current Session.
- **Return destinations**: Local and OIDC flows accept only normalized same-origin application paths. External, protocol-relative, malformed, and authentication-loop destinations fall back to the default authenticated route.

## Testing Decisions

- **Good-test rule**: Tests assert externally observable authentication and authorization behavior: HTTP status and error codes, cookies, redirects, persisted policy effects visible through later requests, Session revocation, browser-visible controls, and Audit Events. Tests must not assert private helper calls, library internals, raw database query order, or a particular OIDC package's implementation details.
- **Primary acceptance seam**: Use one full-stack Playwright seam running the real Web application, Core, PostgreSQL, and a deterministic local OIDC Provider. This is the highest available seam and is the release-level proof for fresh Bootstrap, password-only login, TOTP enablement, two-stage login, Recovery Code login, Session renewal, Step-up, OIDC binding, OIDC login, local-only logout, unbinding, TOTP disablement, and continued local recovery.
- **Primary-seam scenarios**: Cover a fresh installation with policy disabled; enable TOTP and save Recovery Codes; log out and complete both login stages; consume one Recovery Code; bind the test provider after re-authentication; verify the OIDC button appears; log in through OIDC; verify logout does not end the provider Session; unlink OIDC; disable TOTP; and verify password-only login returns.
- **Core HTTP integration seam**: Extend the existing real-PostgreSQL authentication tests for cases that are expensive or unsafe to express only in a browser: challenge expiry and replay, TOTP counter replay, Recovery Code consumption, rate limiting, Session and Step-up revocation, proof-matrix enforcement, generic error envelopes, return-destination validation, and Audit Event redaction.
- **OIDC protocol integration seam**: Drive Core against a controllable HTTP test provider and test wrong issuer, audience, signature, nonce, state, PKCE verifier, subject, expiry, key rotation, malformed discovery, unreachable discovery, callback replay, login-purpose versus bind-purpose confusion, and an unbound valid provider identity. Assert outcomes at Core's HTTP boundary rather than against OIDC library functions.
- **Migration seam**: Apply migrations to database fixtures representing a confirmed TOTP Administrator, an active password-only Administrator, a pending incomplete enrollment, and multiple human identities. Start Core and assert the externally visible policy/login result or the required startup diagnostic.
- **Configuration seam**: Extend configuration tests for public URL normalization, HTTPS requirements, localhost development exceptions, issuer/client/scopes parsing, secret redaction, and incomplete OIDC configuration. These tests may remain focused unit tests because they define the deployment input boundary.
- **Web component seam**: Use React Testing Library and the existing mocked API server only for interaction states not economically distinguished by the full-stack suite: conditional OIDC button visibility, password-to-second-factor transition, TOTP versus Recovery Code control switching, expired-challenge focus recovery, and security-setting confirmation states.
- **Prior art**: Reuse the existing Core authentication application tests for Bootstrap, login/logout, password-change revocation, idle expiry, and legacy enrollment; reuse the existing Playwright vertical-slice harness for real Web/Core/PostgreSQL workflows; reuse the existing login and Bootstrap page tests for form-level accessibility and conditional rendering; and reuse configuration parsing tests for environment contracts.
- **Quality gates**: The OpenAPI document validates, generated client output is current, Go tests pass with race detection where supported, Web type checking and unit tests pass, the production Web build succeeds, and the full-stack Playwright authentication journey passes against the deterministic OIDC Provider.

## Out of Scope

- Supporting more than one Administrator, Viewer roles, Member Invitations, member provisioning, role management, or per-member authentication policies.
- Physically removing every multi-member compatibility table, role column, invitation record, or historical Audit Event in this delivery.
- Generic OAuth 2.0 providers that do not provide OpenID Connect identity claims and discovery.
- Supporting multiple OIDC providers or more than one External Identity Binding.
- Automatic identity linking by email, username, domain, or group and automatic creation of an Administrator from the first OIDC login.
- Requiring AutoSecrets TOTP after OIDC or interpreting provider `acr`/`amr` claims as an AutoSecrets policy engine.
- Calling provider APIs, synchronizing provider profiles or groups, requesting offline access, or storing provider tokens.
- Provider-initiated logout, global SSO logout, back-channel logout, front-channel logout, or provider account revocation synchronization.
- Trusted-device or “remember this browser” exemptions for local TOTP.
- SMS, email, push, WebAuthn, passkeys, or other authentication factors.
- Anonymous password recovery or an OIDC-only mode that disables the local password recovery path.
- A Web workflow that resolves an ambiguous multi-identity migration; remediation remains host-local and explicit.
- Redesigning bounded Session lifetimes, CSRF policy, idle-activity semantics, or non-authentication authorization rules.

## Further Notes

- This specification implements ADR-0026 and supersedes authentication behavior derived from the mandatory-MFA and multi-member ADRs.
- “TOTP Login Policy” and the UI phrase “local login requires TOTP” are deliberate. Calling the setting “MFA required” would incorrectly imply that AutoSecrets enforces a second factor on OIDC.
- The local password remains a recovery credential even for an Administrator who normally uses OIDC. Deployment documentation must state that losing both the local credential and provider access still requires the existing host-local recovery mechanism.
- OIDC client registration must list the callback generated from `CORE_PUBLIC_URL` exactly. Reverse-proxy documentation must not recommend deriving security-sensitive URLs from forwarded headers.
- The repository currently has no configured Git remote and already uses `.scratch/<feature>/spec.md` for local issues, so this specification is published to the local Markdown tracker with the canonical `ready-for-agent` status.

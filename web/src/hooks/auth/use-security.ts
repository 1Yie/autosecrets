import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiDelete, apiGet, apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";
import { useSessionStore } from "../../stores/session-store";
import type { EnrollmentContext } from "../../components/mfa-enrollment-steps";

export interface ExternalProviderSecurity {
  available: boolean;
  bound: boolean;
  issuer?: string;
  display_name?: string;
  configuration_error?: string;
}

export interface AuthenticationSecurity {
  totp_login_required: boolean;
  oidc: ExternalProviderSecurity;
  oauth?: ExternalProviderSecurity;
}

export interface CredentialProof {
  password: string;
  totp_code?: string;
}

export function useAuthenticationSecurity() {
  return useQuery({
    queryKey: ["auth-security"],
    queryFn: () => apiGet<AuthenticationSecurity>(API_PATHS.authSecurity),
    retry: false,
  });
}

export function useStartTOTPEnrollment() {
  return useMutation({
    mutationFn: (password: string) =>
      apiPost<EnrollmentContext>(API_PATHS.totpEnrollment, { password }),
  });
}

export function useDisableTOTP() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiDelete<{ totp_login_required: false }>(API_PATHS.totp, proof),
    onSuccess: () => client.invalidateQueries({ queryKey: ["auth-security"] }),
  });
}

export function useStartOIDCBinding() {
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiPost<{ authorization_url: string }>(API_PATHS.oidcBinding, {
        ...proof,
        return_to: "/dashboard/login-and-security",
      }),
  });
}

export function useDeleteOIDCBinding() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiDelete<{ bound: false }>(API_PATHS.oidcBinding, proof),
    onSuccess: () =>
      Promise.all([
        client.invalidateQueries({ queryKey: ["auth-security"] }),
        client.invalidateQueries({ queryKey: ["oidc-status"] }),
      ]),
  });
}

export function useStartOAuthBinding() {
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiPost<{ authorization_url: string }>(API_PATHS.oauthBinding, {
        ...proof,
        return_to: "/dashboard/login-and-security",
      }),
  });
}

export function useDeleteOAuthBinding() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiDelete<{ bound: false }>(API_PATHS.oauthBinding, proof),
    onSuccess: () =>
      Promise.all([
        client.invalidateQueries({ queryKey: ["auth-security"] }),
        client.invalidateQueries({ queryKey: ["oidc-status"] }),
      ]),
  });
}

export interface SessionReissue {
  csrf_token: string;
  username: string;
  id: string;
  role: string;
  expires_at: string;
}

function applyReissuedSession(session: SessionReissue) {
  useSessionStore.getState().setCsrfToken(session.csrf_token);
}

export function useChangeUsername() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      username: string;
      current_password: string;
      totp_code?: string;
    }) => apiPost<SessionReissue>(API_PATHS.authUsername, body),
    onSuccess: (session) => {
      applyReissuedSession(session);
      return client.invalidateQueries({ queryKey: ["me"] });
    },
  });
}

export function useChangePassword() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      current_password: string;
      new_password: string;
      totp_code?: string;
    }) => apiPost<SessionReissue>(API_PATHS.authPassword, body),
    onSuccess: (session) => {
      applyReissuedSession(session);
      return client.invalidateQueries({ queryKey: ["me"] });
    },
  });
}

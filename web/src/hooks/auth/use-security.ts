import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiDelete, apiGet, apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";
import type { EnrollmentContext } from "../../components/mfa-enrollment-steps";

export interface AuthenticationSecurity {
  totp_login_required: boolean;
  oidc: {
    available: boolean;
    bound: boolean;
    issuer?: string;
    display_name?: string;
    configuration_error?: string;
  };
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
    mutationFn: (password: string) => apiPost<EnrollmentContext>(API_PATHS.totpEnrollment, { password }),
  });
}

export function useDisableTOTP() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (proof: CredentialProof) => apiDelete<{ totp_login_required: false }>(API_PATHS.totp, proof),
    onSuccess: () => client.invalidateQueries({ queryKey: ["auth-security"] }),
  });
}

export function useStartOIDCBinding() {
  return useMutation({
    mutationFn: (proof: CredentialProof) =>
      apiPost<{ authorization_url: string }>(API_PATHS.oidcBinding, {
        ...proof,
        return_to: "/dashboard/security",
      }),
  });
}

export function useDeleteOIDCBinding() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (proof: CredentialProof) => apiDelete<{ bound: false }>(API_PATHS.oidcBinding, proof),
    onSuccess: () => Promise.all([
      client.invalidateQueries({ queryKey: ["auth-security"] }),
      client.invalidateQueries({ queryKey: ["oidc-status"] }),
    ]),
  });
}

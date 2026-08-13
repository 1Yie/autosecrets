import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type LoginForm, type SecondFactorForm } from "../../lib/constants/schemas";
import { useSessionStore } from "../../stores/session-store";

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: LoginForm) => {
      const res = await apiPost<{ csrf_token?: string; code?: string }>(API_PATHS.login, body);
      if (res.csrf_token) useSessionStore.getState().setCsrfToken(res.csrf_token);
      return res;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}

export function useCompleteLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: SecondFactorForm) => {
      const payload = body.totp_code
        ? { totp_code: body.totp_code }
        : { recovery_code: body.recovery_code };
      const res = await apiPost<{ csrf_token: string }>(API_PATHS.loginSecondFactor, payload);
      useSessionStore.getState().setCsrfToken(res.csrf_token);
      return res;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}

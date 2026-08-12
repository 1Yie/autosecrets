import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type LoginForm } from "../../lib/constants/schemas";
import { useSessionStore } from "../../stores/session-store";

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: LoginForm) => {
      const res = await apiPost<{ csrf_token: string }>(API_PATHS.login, body);
      useSessionStore.getState().setCsrfToken(res.csrf_token);
      return res;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}

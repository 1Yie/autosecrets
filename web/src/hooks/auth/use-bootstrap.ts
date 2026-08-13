import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type BootstrapForm } from "../../lib/constants/schemas";
import { useSessionStore } from "../../stores/session-store";
import type { Me } from "./use-me";

interface BootstrapResponse {
  id: string;
  username: string;
  csrf_token: string;
  expires_at: string;
}

export function useBootstrap() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (body: BootstrapForm) => {
      const result = await apiPost<BootstrapResponse>(API_PATHS.bootstrap, body);
      useSessionStore.getState().setCsrfToken(result.csrf_token);
      return result;
    },
    onSuccess: (result, body) => {
      client.setQueryData<Me>(["me"], {
        bootstrap_required: false,
        organization: { display_name: body.organization_name },
        member: { id: result.id, username: result.username, role: "administrator" },
        csrf_token: result.csrf_token,
        session_expires_at: result.expires_at,
        idle_expires_at: result.expires_at,
        totp_login_required: false,
        auth_method: "local",
      });
      void client.invalidateQueries({ queryKey: ["me"] });
    },
  });
}

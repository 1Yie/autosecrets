import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";
import { useSessionStore } from "../../stores/session-store";

export interface Me {
  bootstrap_required: boolean;
  organization?: { display_name: string };
  member?: { id: string; username: string; role: "administrator" };
  csrf_token?: string;
  session_expires_at?: string;
  idle_expires_at?: string;
  totp_login_required?: boolean;
  auth_method?: "local" | "oidc";
}

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const me = await apiGet<Me>(API_PATHS.me);
      if (me.csrf_token) useSessionStore.getState().setCsrfToken(me.csrf_token);
      return me;
    },
    retry: false,
  });
}

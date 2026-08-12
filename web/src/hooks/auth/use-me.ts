import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";
import { useSessionStore } from "../../stores/session-store";

export interface Me {
  bootstrap_required: boolean;
  admin?: { id: string; username: string };
  csrf_token?: string;
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

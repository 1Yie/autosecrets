import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { useSessionStore } from "../../stores/session-store";

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiPost(API_PATHS.logout),
    onSuccess: () => {
      useSessionStore.getState().clearSession();
      qc.clear();
    },
  });
}

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type BootstrapForm } from "../../lib/constants/schemas";
import { type Me } from "./use-me";

export function useBootstrap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: BootstrapForm) => apiPost(API_PATHS.bootstrap, body),
    onSuccess: () => {
      // The refetched /me would 401 (no session yet) and leave stale data
      // behind; set the transition explicitly so the UI moves to login.
      qc.setQueryData<Me>(["me"], { bootstrap_required: false });
    },
  });
}

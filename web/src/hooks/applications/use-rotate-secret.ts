import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

/** Rotate to the next candidate version (Core-driven, keep-old-value). */
export function useRotateSecret(secretId: string, appId: string, envId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiPost<{ version_seq: number; draft_version: number }>(
      API_PATHS.secretRotate(secretId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets", appId, envId] });
      qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
    },
  });
}

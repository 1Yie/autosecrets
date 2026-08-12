import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useInstallCommand() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; bundle_dir?: string }) =>
      apiPost<{ command: string; expires_at: string }>(API_PATHS.installCommand, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
  });
}

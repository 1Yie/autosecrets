import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useInstallCommand() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      apiPost<{ command: string; expires_at: string }>(API_PATHS.installCommand, { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
  });
}

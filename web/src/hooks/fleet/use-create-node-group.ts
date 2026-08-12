import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useCreateNodeGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => apiPost(API_PATHS.nodeGroups, { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["node-groups"] }),
  });
}

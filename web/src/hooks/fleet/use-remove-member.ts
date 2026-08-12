import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useRemoveMember(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (nodeId: string) => apiDelete(API_PATHS.groupMember(groupId, nodeId)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["node-groups"] }),
  });
}

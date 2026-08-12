import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useAddMember(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (nodeId: string) => apiPost(API_PATHS.groupMembers(groupId), { node_id: nodeId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["node-groups"] }),
  });
}

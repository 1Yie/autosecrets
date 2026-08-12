import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useCreateAssignment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { group_id: string; revision_id: string }) =>
      apiPost(API_PATHS.assignments, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["assignments"] }),
  });
}

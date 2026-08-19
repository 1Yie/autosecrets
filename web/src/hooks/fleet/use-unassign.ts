import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useUnassign() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (assignmentId: string) =>
			apiPost(API_PATHS.assignmentUnassign(assignmentId), {}),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["assignments"] });
			qc.invalidateQueries({ queryKey: ["nodes"] });
		},
	});
}

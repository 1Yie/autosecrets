import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useDeleteNodeGroup() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (groupId: string) => apiDelete(API_PATHS.nodeGroup(groupId)),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["node-groups"] });
			qc.invalidateQueries({ queryKey: ["assignments"] });
		},
	});
}

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useDeleteNode() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (nodeId: string) => apiDelete(API_PATHS.node(nodeId)),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["nodes"] });
			qc.invalidateQueries({ queryKey: ["node-groups"] });
			qc.invalidateQueries({ queryKey: ["assignments"] });
		},
	});
}

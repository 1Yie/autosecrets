import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useDeleteSecret(appId: string, envId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (secretId: string) => apiDelete(API_PATHS.secret(secretId)),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["secrets", appId, envId] });
			qc.invalidateQueries({ queryKey: ["draft", appId, envId] });
			qc.invalidateQueries({ queryKey: ["revisions", appId, envId] });
		},
	});
}

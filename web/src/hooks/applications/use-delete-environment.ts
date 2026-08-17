import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useDeleteEnvironment(appId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (envId: string) =>
			apiDelete(API_PATHS.environment(appId, envId)),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["application", appId] }),
	});
}

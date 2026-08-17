import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiDelete } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export function useDeleteApplication() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (appId: string) => apiDelete(API_PATHS.application(appId)),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
	});
}

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPatch } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface UpdateNodeResponse {
	id: string;
	poll_interval_seconds: number;
}

/** Adjusts a Managed Node's settings; today only the Agent polling interval. */
export function useUpdateNode(nodeId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (pollIntervalSeconds: number) =>
			apiPatch<UpdateNodeResponse>(API_PATHS.node(nodeId), {
				poll_interval_seconds: pollIntervalSeconds,
			}),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
	});
}

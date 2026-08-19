import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPatch } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";
import type { ManagedNode } from "./use-nodes";

export interface UpdateNodeBody {
	name?: string;
	bundle_dir?: string;
	poll_interval_seconds?: number;
}

export function useUpdateNode(nodeId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (body: UpdateNodeBody) =>
			apiPatch<ManagedNode>(API_PATHS.node(nodeId), body),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
	});
}

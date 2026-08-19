import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiPost } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";
import type { ManagedNode } from "./use-nodes";

export function useCreateNode() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (body: { name: string; bundle_dir?: string }) =>
			apiPost<ManagedNode>(API_PATHS.nodes, body),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
	});
}

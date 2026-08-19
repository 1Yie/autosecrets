import { useCursorPage } from "../shared/use-cursor-page";
import { API_PATHS } from "../../lib/constants/api-paths";
import { listPageQuery } from "../../lib/constants/pagination";

export interface ManagedNode {
	id: string;
	name: string;
	serial: string;
	created_at: string;
	last_seen_at: string | null;
	desired_etag: string;
	observed_revision: string;
	last_result: string;
	state: "never_online" | "healthy" | "converging" | "failed" | "offline";
	unassigned: boolean;
	poll_interval_seconds: number;
	bundle_dir: string;
	enrolled: boolean;
}

export function useNodes(limit?: number) {
	return useCursorPage<ManagedNode>(
		["nodes"],
		(cursor, page) => listPageQuery(API_PATHS.nodes, cursor, page),
		limit === undefined ? {} : { limit },
	);
}

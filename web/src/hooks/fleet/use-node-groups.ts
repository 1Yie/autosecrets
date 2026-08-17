import { useCursorPage } from "../shared/use-cursor-page";
import { API_PATHS } from "../../lib/constants/api-paths";
import { listPageQuery } from "../../lib/constants/pagination";

export interface NodeGroup {
	id: string;
	name: string;
	member_ids: string[];
}

export function useNodeGroups() {
	return useCursorPage<NodeGroup>(["node-groups"], (cursor, page) =>
		listPageQuery(API_PATHS.nodeGroups, cursor, page),
	);
}

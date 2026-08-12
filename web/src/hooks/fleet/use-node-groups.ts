import { useCursorPage } from "../shared/use-cursor-page";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface NodeGroup {
  id: string;
  name: string;
  member_ids: string[];
}

export function useNodeGroups() {
  return useCursorPage<NodeGroup>(["node-groups"], (cursor) =>
    `${API_PATHS.nodeGroups}?${cursor ? `cursor=${cursor}` : ""}`,
  );
}

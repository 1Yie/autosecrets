import { useCursorPage } from "../shared/use-cursor-page";
import { API_PATHS } from "../../lib/constants/api-paths";

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
}

export function useNodes() {
  return useCursorPage<ManagedNode>(["nodes"], (cursor) =>
    `${API_PATHS.nodes}?${cursor ? `cursor=${cursor}` : ""}`,
  );
}

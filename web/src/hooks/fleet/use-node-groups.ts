import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface NodeGroup {
  id: string;
  name: string;
  member_ids: string[];
}

export function useNodeGroups() {
  return useQuery({
    queryKey: ["node-groups"],
    queryFn: () => apiGet<NodeGroup[]>(API_PATHS.nodeGroups),
  });
}

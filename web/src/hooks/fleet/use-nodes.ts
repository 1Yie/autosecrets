import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
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
}

export function useNodes() {
  return useQuery({
    queryKey: ["nodes"],
    queryFn: () => apiGet<ManagedNode[]>(API_PATHS.nodes),
    refetchInterval: 10_000,
  });
}

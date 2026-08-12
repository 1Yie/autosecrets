import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";

export interface OverviewAttention {
  kind: string;
  count: number;
  resource?: string;
}

export interface Overview {
  generated_at: string;
  counts: {
    applications: number;
    environments: number;
    secrets: number;
    nodes: number;
    node_groups: number;
    assignments: number;
    audit_events: number;
  };
  attention: OverviewAttention[];
  recent_publishes: Array<{
    id: string;
    draft_version: number;
    file_count: number;
    created_by: string;
    created_at: string;
    operation_reason: { category: string; explanation: string; external_ref?: string };
  }>;
  recent_audit: Array<{
    id: number;
    actor: string;
    action: string;
    resource: string;
    result: string;
    correlation_id: string;
    created_at: string;
  }>;
}

export function useOverview() {
  return useQuery({
    queryKey: ["overview"],
    queryFn: () => apiGet<Overview>(API_PATHS.overview),
    refetchInterval: 15_000,
  });
}

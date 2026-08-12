import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";

export interface AuditEvent {
  id: number;
  actor: string;
  action: string;
  resource: string;
  result: string;
  correlation_id: string;
  created_at: string;
}

export function useAuditEvents(limit = 100) {
  return useQuery({
    queryKey: ["audit-events", limit],
    queryFn: () => apiGet<AuditEvent[]>(`${API_PATHS.auditEvents}?limit=${limit}`),
    refetchInterval: 30_000,
  });
}

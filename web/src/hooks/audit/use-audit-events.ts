import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";
import type { Page } from "../shared/use-cursor-page";

export interface AuditEvent {
  id: number;
  actor: string;
  action: string;
  resource: string;
  result: string;
  correlation_id: string;
  created_at: string;
  actor_type: string;
  actor_id: string;
  actor_display: string;
  resource_type: string;
  resource_id: string;
  resource_display: string;
  outcome: string;
  operation_reason_category: string;
  operation_reason_explanation: string;
  operation_reason_external_ref: string;
}

export interface AuditFilters {
  actor: string;
  action: string;
  resource: string;
  outcome: string;
  reason_category: string;
}

export const emptyAuditFilters: AuditFilters = {
  actor: "",
  action: "",
  resource: "",
  outcome: "",
  reason_category: "",
};

export function useAuditEvents() {
  const [filters, setFilters] = useState<AuditFilters>(emptyAuditFilters);
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const query = useQuery({
    queryKey: ["audit-events", filters, cursor],
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("limit", "25");
      if (cursor) params.set("cursor", cursor);
      for (const [key, value] of Object.entries(filters)) {
        if (value) params.set(key, value);
      }
      return apiGet<Page<AuditEvent>>(`${API_PATHS.auditEvents}?${params.toString()}`);
    },
    refetchInterval: 30_000,
  });
  const next = () => {
    if (!query.data?.next_cursor) return;
    setHistory((prev) => [...prev, cursor]);
    setCursor(query.data.next_cursor);
  };
  const prev = () => {
    if (history.length === 0) return;
    setHistory((prev) => prev.slice(0, -1));
    setCursor(history[history.length - 1]);
  };
  const applyFilters = (nextFilters: AuditFilters) => {
    setFilters(nextFilters);
    setCursor("");
    setHistory([]);
  };
  return {
    items: query.data?.items ?? [],
    nextCursor: query.data?.next_cursor ?? "",
    isFirstPage: history.length === 0,
    ...query,
    next,
    prev,
    filters,
    applyFilters,
  };
}

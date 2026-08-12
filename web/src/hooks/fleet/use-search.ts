import { useQuery } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiGet } from "../../lib/api";

export interface SearchResult {
  type: "application" | "environment" | "node" | "node_group";
  id: string;
  name: string;
}

export function useSearch(query: string) {
  const trimmed = query.trim();
  return useQuery({
    queryKey: ["search", trimmed],
    queryFn: () =>
      apiGet<{ results: SearchResult[] }>(
        `${API_PATHS.search}?q=${encodeURIComponent(trimmed)}`,
      ),
    enabled: trimmed.length >= 2,
  });
}

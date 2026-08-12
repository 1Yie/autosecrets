import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../lib/api";

export interface Page<T> {
  items: T[];
  next_cursor: string;
}

/** Shared cursor-pagination state: opaque server cursors stay in navigation
 * state (never in shareable URLs), with previous/next batch movement. */
export function useCursorPage<T>(queryKey: string[], path: (cursor: string) => string, limit = 25) {
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const query = useQuery({
    queryKey: [...queryKey, cursor],
    queryFn: () => apiGet<Page<T>>(`${path(cursor)}&limit=${limit}`),
  });
  const next = () => {
    if (!query.data?.next_cursor) return;
    setHistory((prev) => [...prev, cursor]);
    setCursor(query.data.next_cursor);
  };
  const prev = () => {
    if (history.length === 0) return;
    const last = history[history.length - 1];
    setHistory((prev) => prev.slice(0, -1));
    setCursor(last);
  };
  return {
    items: query.data?.items ?? [],
    nextCursor: query.data?.next_cursor ?? "",
    isFirstPage: history.length === 0,
    ...query,
    next,
    prev,
  };
}

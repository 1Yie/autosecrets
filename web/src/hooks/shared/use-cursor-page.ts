import { useState } from "react";
import { useQuery, type UseQueryOptions } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
import { DEFAULT_PAGE_SIZE } from "../../lib/constants/pagination";

export {
	DEFAULT_PAGE_SIZE,
	PAGE_SIZE_OPTIONS,
} from "../../lib/constants/pagination";

export interface Page<T> {
	items: T[];
	next_cursor: string;
	total?: number;
}

interface CursorPageOptions<T> {
	limit?: number;
	query?: Omit<UseQueryOptions<Page<T>>, "queryKey" | "queryFn">;
}

/** Shared pagination state: next/prev keep using opaque cursors; jump-to-page
 * sends a 1-based page number so Core can offset into the list. */
export function useCursorPage<T>(
	queryKey: unknown[],
	path: (cursor: string, page: number) => string,
	limitOrOptions: number | CursorPageOptions<T> = DEFAULT_PAGE_SIZE,
) {
	const initialLimit =
		typeof limitOrOptions === "number"
			? limitOrOptions
			: (limitOrOptions.limit ?? DEFAULT_PAGE_SIZE);
	const extraQuery =
		typeof limitOrOptions === "number" ? undefined : limitOrOptions.query;
	const [limit, setLimitState] = useState(initialLimit);
	const [cursor, setCursor] = useState("");
	const [pageIndex, setPageIndex] = useState(1);
	const [history, setHistory] = useState<string[]>([]);
	const query = useQuery({
		...extraQuery,
		queryKey: [...queryKey, cursor, pageIndex, limit],
		queryFn: () => apiGet<Page<T>>(`${path(cursor, pageIndex)}&limit=${limit}`),
	});
	const total = query.data?.total ?? 0;
	const pageCount = total > 0 ? Math.max(1, Math.ceil(total / limit)) : 0;

	const reset = () => {
		setCursor("");
		setHistory([]);
		setPageIndex(1);
	};

	const next = () => {
		if (
			!query.data?.next_cursor &&
			(pageCount === 0 || pageIndex >= pageCount)
		) {
			return;
		}
		setHistory((prev) => [...prev, cursor]);
		setCursor(query.data?.next_cursor ?? "");
		setPageIndex((prev) => prev + 1);
	};

	const prev = () => {
		if (pageIndex <= 1) return;
		const last = history.at(-1);
		setHistory((prevHistory) => prevHistory.slice(0, -1));
		setCursor(last ?? "");
		setPageIndex((prev) => prev - 1);
	};

	const goToPage = (page: number) => {
		if (!Number.isInteger(page) || page < 1) return;
		if (pageCount > 0 && page > pageCount) return;
		if (page === pageIndex) return;
		if (page === 1) {
			reset();
			return;
		}
		if (page === pageIndex + 1) {
			next();
			return;
		}
		if (page === pageIndex - 1) {
			prev();
			return;
		}
		setHistory([]);
		setCursor("");
		setPageIndex(page);
	};

	const setLimit = (nextLimit: number) => {
		if (nextLimit === limit) return;
		setLimitState(nextLimit);
		reset();
	};

	return {
		items: query.data?.items ?? [],
		nextCursor: query.data?.next_cursor ?? "",
		isFirstPage: pageIndex === 1,
		pageIndex,
		pageSize: limit,
		pageCount,
		total,
		...query,
		next,
		prev,
		goToPage,
		setLimit,
		reset,
	};
}

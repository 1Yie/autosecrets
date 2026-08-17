export const DEFAULT_PAGE_SIZE = 25;
export const PAGE_SIZE_OPTIONS = [10, 25, 50] as const;

export type VisiblePageItem = number | "ellipsis";

export function listPageQuery(base: string, cursor: string, page: number) {
	const params = new URLSearchParams();
	if (cursor) params.set("cursor", cursor);
	else if (page > 1) params.set("page", String(page));
	return `${base}?${params.toString()}`;
}

/** Compact 1-based page window: 1 2 3 … N, with the current page kept visible. */
export function visiblePageItems(
	current: number,
	total: number,
): VisiblePageItem[] {
	if (total <= 0) return [];
	if (total <= 7) {
		return Array.from({ length: total }, (_, index) => index + 1);
	}
	if (current <= 3) {
		return [1, 2, 3, 4, "ellipsis", total];
	}
	if (current >= total - 2) {
		return [1, "ellipsis", total - 3, total - 2, total - 1, total];
	}
	return [1, "ellipsis", current - 1, current, current + 1, "ellipsis", total];
}

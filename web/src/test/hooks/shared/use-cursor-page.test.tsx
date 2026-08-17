import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { useCursorPage } from "../../../hooks/shared/use-cursor-page";
import { server } from "../../server";

interface PageItem {
	id: string;
}

function wrapper({ children }: { children: React.ReactNode }) {
	const qc = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useCursorPage", () => {
	it("moves between pages and jumps back to a visited page", async () => {
		server.use(
			http.get("/api/v1/page-items", ({ request }) => {
				const url = new URL(request.url);
				const cursor = url.searchParams.get("cursor") ?? "";
				const page = url.searchParams.get("page") ?? "";
				if (cursor === "page-2" || page === "2") {
					return HttpResponse.json({
						items: [{ id: "c" }],
						next_cursor: "",
						total: 3,
					});
				}
				return HttpResponse.json({
					items: [{ id: "a" }, { id: "b" }],
					next_cursor: "page-2",
					total: 3,
				});
			}),
		);
		const { result } = renderHook(
			() =>
				useCursorPage<PageItem>(["page-items"], (cursor, page) => {
					const params = new URLSearchParams();
					if (cursor) params.set("cursor", cursor);
					else if (page > 1) params.set("page", String(page));
					return `/api/v1/page-items?${params.toString()}`;
				}),
			{ wrapper },
		);
		await waitFor(() =>
			expect(result.current.items.map((item) => item.id)).toEqual(["a", "b"]),
		);
		expect(result.current.pageIndex).toBe(1);
		expect(result.current.pageCount).toBe(1);
		expect(result.current.total).toBe(3);

		result.current.next();
		await waitFor(() =>
			expect(result.current.items.map((item) => item.id)).toEqual(["c"]),
		);
		expect(result.current.pageIndex).toBe(2);

		result.current.goToPage(1);
		await waitFor(() =>
			expect(result.current.items.map((item) => item.id)).toEqual(["a", "b"]),
		);
		expect(result.current.pageIndex).toBe(1);
	});

	it("jumps to an unvisited page with the page query", async () => {
		const seen: string[] = [];
		server.use(
			http.get("/api/v1/page-items", ({ request }) => {
				const url = new URL(request.url);
				seen.push(url.search);
				const page = Number(url.searchParams.get("page") ?? "1");
				return HttpResponse.json({
					items: [{ id: String(page) }],
					next_cursor: page < 3 ? "next" : "",
					total: 3,
				});
			}),
		);
		const { result } = renderHook(
			() =>
				useCursorPage<PageItem>(
					["page-jump"],
					(cursor, page) => {
						const params = new URLSearchParams();
						if (cursor) params.set("cursor", cursor);
						else if (page > 1) params.set("page", String(page));
						return `/api/v1/page-items?${params.toString()}`;
					},
					1,
				),
			{ wrapper },
		);
		await waitFor(() => expect(result.current.items[0]?.id).toBe("1"));
		result.current.goToPage(3);
		await waitFor(() => expect(result.current.items[0]?.id).toBe("3"));
		expect(result.current.pageIndex).toBe(3);
		expect(seen.some((query) => query.includes("page=3"))).toBe(true);
	});

	it("resets to the first page when the page size changes", async () => {
		server.use(
			http.get("/api/v1/page-items", ({ request }) => {
				const url = new URL(request.url);
				const limit = Number(url.searchParams.get("limit"));
				return HttpResponse.json({
					items: Array.from({ length: limit }, (_, index) => ({
						id: String(index + 1),
					})),
					next_cursor: "page-2",
					total: 40,
				});
			}),
		);
		const { result } = renderHook(
			() => useCursorPage<PageItem>(["page-size"], () => "/api/v1/page-items?"),
			{ wrapper },
		);
		await waitFor(() => expect(result.current.items).toHaveLength(25));
		result.current.next();
		await waitFor(() => expect(result.current.pageIndex).toBe(2));
		result.current.setLimit(10);
		await waitFor(() => expect(result.current.pageSize).toBe(10));
		expect(result.current.pageIndex).toBe(1);
		await waitFor(() => expect(result.current.items).toHaveLength(10));
	});
});

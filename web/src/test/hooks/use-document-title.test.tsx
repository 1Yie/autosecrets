import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useDocumentTitle } from "../../hooks/use-document-title";

describe("useDocumentTitle", () => {
	afterEach(() => {
		document.title = "";
	});

	it("sets title · AutoSecrets for a page title", () => {
		const { unmount } = renderHook(() => useDocumentTitle("概览"));
		expect(document.title).toBe("概览 · AutoSecrets");
		unmount();
	});

	it("updates the title when the page title changes", () => {
		const { rerender } = renderHook(({ title }) => useDocumentTitle(title), {
			initialProps: { title: "应用" },
		});
		expect(document.title).toBe("应用 · AutoSecrets");
		rerender({ title: "节点" });
		expect(document.title).toBe("节点 · AutoSecrets");
	});

	it("falls back to the product name when the title is undefined", () => {
		renderHook(() => useDocumentTitle(undefined));
		expect(document.title).toBe("AutoSecrets");
	});
});

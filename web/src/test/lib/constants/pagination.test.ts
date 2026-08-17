import { describe, expect, it } from "vitest";
import { visiblePageItems } from "../../../lib/constants/pagination";

describe("visiblePageItems", () => {
	it("returns every page when the range is short", () => {
		expect(visiblePageItems(1, 3)).toEqual([1, 2, 3]);
		expect(visiblePageItems(4, 7)).toEqual([1, 2, 3, 4, 5, 6, 7]);
	});

	it("keeps the start of a long range compact", () => {
		expect(visiblePageItems(1, 12)).toEqual([1, 2, 3, 4, "ellipsis", 12]);
		expect(visiblePageItems(3, 12)).toEqual([1, 2, 3, 4, "ellipsis", 12]);
	});

	it("keeps the current page visible in the middle", () => {
		expect(visiblePageItems(5, 12)).toEqual([
			1,
			"ellipsis",
			4,
			5,
			6,
			"ellipsis",
			12,
		]);
	});

	it("keeps the end of a long range compact", () => {
		expect(visiblePageItems(11, 12)).toEqual([1, "ellipsis", 9, 10, 11, 12]);
	});
});

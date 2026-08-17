import { describe, expect, it } from "vitest";
import { apiURL, resolveApiBase } from "../../../lib/env";

describe("resolveApiBase", () => {
	it("trims a trailing slash", () => {
		expect(resolveApiBase("http://127.0.0.1:18080/")).toBe(
			"http://127.0.0.1:18080",
		);
	});

	it("stays empty when unset", () => {
		expect(resolveApiBase(undefined)).toBe("");
		expect(resolveApiBase("")).toBe("");
	});
});

describe("apiURL", () => {
	it("leaves absolute URLs alone", () => {
		expect(apiURL("https://example/api/v1/me")).toBe("https://example/api/v1/me");
	});
});

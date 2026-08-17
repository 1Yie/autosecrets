import { describe, expect, it } from "vitest";
import {
	POLL_INTERVAL_OPTIONS,
	pollIntervalLabel,
} from "../../../lib/constants/poll-interval";

describe("pollIntervalLabel", () => {
	it("labels sub-minute intervals in seconds", () => {
		expect(pollIntervalLabel(5)).toBe("5秒");
		expect(pollIntervalLabel(30)).toBe("30秒");
	});

	it("labels whole minutes", () => {
		expect(pollIntervalLabel(60)).toBe("1分钟");
		expect(pollIntervalLabel(600)).toBe("10分钟");
	});

	it("labels whole hours", () => {
		expect(pollIntervalLabel(3600)).toBe("1小时");
	});

	it("falls back to seconds for odd values", () => {
		expect(pollIntervalLabel(45)).toBe("45秒");
		expect(pollIntervalLabel(90)).toBe("90秒");
	});
});

describe("POLL_INTERVAL_OPTIONS", () => {
	it("stays within the Core bounds of 5s-24h", () => {
		for (const seconds of POLL_INTERVAL_OPTIONS) {
			expect(seconds).toBeGreaterThanOrEqual(5);
			expect(seconds).toBeLessThanOrEqual(86400);
		}
	});
});

import { describe, expect, it } from "vitest";
import {
	getStoredScrollState,
	saveStoredScrollState,
} from "./scroll_state_storage.ts";

const STORAGE_KEY = "__vorma__scrollStateMap";

describe("scroll_state_storage", () => {
	it("drops malformed stored maps instead of keeping corrupt state", () => {
		sessionStorage.setItem(STORAGE_KEY, "{not-json");

		expect(getStoredScrollState("missing")).toBeUndefined();
		expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull();
	});

	it("can save fresh state after malformed stored data is cleared", () => {
		sessionStorage.setItem(STORAGE_KEY, "{not-json");

		saveStoredScrollState("k1", { x: 10, y: 20 });

		const raw = sessionStorage.getItem(STORAGE_KEY);
		expect(raw).toBeTruthy();
		expect(getStoredScrollState("k1")).toEqual({ x: 10, y: 20 });
	});

	it("evicts the oldest entry even when its key is an empty string", () => {
		const entries: Array<[string, { x: number; y: number }]> = [
			["", { x: 0, y: 0 }],
		];
		for (let i = 1; i < 50; i++) {
			entries.push([`k${i}`, { x: i, y: i }]);
		}
		sessionStorage.setItem(STORAGE_KEY, JSON.stringify(entries));

		saveStoredScrollState("k50", { x: 50, y: 50 });

		const raw = sessionStorage.getItem(STORAGE_KEY);
		expect(raw).toBeTruthy();
		const stored = JSON.parse(raw || "[]") as Array<
			[string, { x: number; y: number }]
		>;
		expect(stored).toHaveLength(50);
		expect(stored.some(([key]) => key === "")).toBe(false);
		expect(stored.some(([key]) => key === "k50")).toBe(true);
	});
});

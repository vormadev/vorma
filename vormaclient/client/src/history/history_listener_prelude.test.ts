import { describe, expect, it } from "vitest";
import { analyzeHistoryListenerPrelude } from "./history_listener_prelude.ts";

describe("history_listener_prelude", () => {
	it("treats POP updates on the same data target as same-document", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "POP" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
		});

		expect(result.didLocationKeyChange).toBe(true);
		expect(result.popWithinSameDoc).toBe(true);
		expect(result.shouldSaveScrollState).toBe(false);
	});

	it("treats query-order changes as different POP targets", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "POP" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?b=2&a=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?a=1&b=2",
			},
		});

		expect(result.popWithinSameDoc).toBe(false);
		expect(result.shouldSaveScrollState).toBe(true);
	});

	it("never flags non-POP actions as same-document POP", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "PUSH" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
		});

		expect(result.popWithinSameDoc).toBe(false);
		expect(result.shouldSaveScrollState).toBe(true);
	});
});

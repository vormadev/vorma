import { describe, expect, it, vi } from "vitest";
import {
	addStatusListener,
	dispatchStatusEvent,
} from "../../platform/events.ts";

describe("platform events", () => {
	it("dispatchStatusEvent does not throw when window is unavailable", () => {
		vi.stubGlobal("window", undefined);
		try {
			expect(() =>
				dispatchStatusEvent({
					isNavigating: false,
					isSubmitting: false,
					isRevalidating: false,
				}),
			).not.toThrow();
		} finally {
			vi.unstubAllGlobals();
		}
	});

	it("addStatusListener returns a safe cleanup when window is unavailable", () => {
		vi.stubGlobal("window", undefined);
		try {
			const cleanup = addStatusListener(() => {});
			expect(typeof cleanup).toBe("function");
			expect(() => cleanup()).not.toThrow();
		} finally {
			vi.unstubAllGlobals();
		}
	});
});

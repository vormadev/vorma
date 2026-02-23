import { describe, expect, it, vi } from "vitest";
import {
	addLocationListener,
	addStatusListener,
	dispatchLocationEvent,
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

	it("dispatches status detail to listeners when window is available", () => {
		const eventTarget = new EventTarget();
		vi.stubGlobal("window", eventTarget as unknown as Window);

		const listener = vi.fn();
		const cleanup = addStatusListener(listener);
		try {
			dispatchStatusEvent({
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: true,
			});
			expect(listener).toHaveBeenCalledTimes(1);
			expect(listener).toHaveBeenCalledWith(
				expect.objectContaining({
					detail: {
						isNavigating: true,
						isSubmitting: false,
						isRevalidating: true,
					},
				}),
			);
		} finally {
			cleanup();
			vi.unstubAllGlobals();
		}
	});

	it("dispatches location events without a detail payload", () => {
		const eventTarget = new EventTarget();
		vi.stubGlobal("window", eventTarget as unknown as Window);

		const listener = vi.fn();
		const cleanup = addLocationListener(listener);
		try {
			dispatchLocationEvent();
			expect(listener).toHaveBeenCalledTimes(1);
			expect(listener).toHaveBeenCalledWith(
				expect.objectContaining({
					detail: null,
					type: "vorma:location",
				}),
			);
		} finally {
			cleanup();
			vi.unstubAllGlobals();
		}
	});
});

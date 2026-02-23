import { describe, expect, it } from "vitest";
import { shouldTriggerFocusRevalidation } from "../../core/extras.ts";

describe("focus revalidation trigger policy state machine", () => {
	it("blocks focus revalidate while navigating", () => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: {
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(shouldRevalidate).toBe(false);
	});

	it("blocks focus revalidate while submitting", () => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: {
				isNavigating: false,
				isSubmitting: true,
				isRevalidating: false,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(shouldRevalidate).toBe(false);
	});

	it("blocks focus revalidate while a revalidation is in flight", () => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: true,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(shouldRevalidate).toBe(false);
	});

	it("blocks focus revalidate when stale window has not elapsed", () => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1049,
			lastTriggeredNavOrRevalidateTimestampMS: 1000,
			staleTimeMS: 50,
		});

		expect(shouldRevalidate).toBe(false);
	});

	it("allows focus revalidate exactly at stale window boundary", () => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1050,
			lastTriggeredNavOrRevalidateTimestampMS: 1000,
			staleTimeMS: 50,
		});

		expect(shouldRevalidate).toBe(true);
	});
});

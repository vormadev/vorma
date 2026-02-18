import { describe, expect, it } from "vitest";
import { decideFocusRevalidationTriggerExecutionPlan } from "../../core/extras.ts";

describe("focus revalidation trigger policy state machine", () => {
	it("blocks focus revalidate while navigating", () => {
		const plan = decideFocusRevalidationTriggerExecutionPlan({
			status: {
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(plan).toEqual({
			type: "skip",
			reason: "focus_revalidate_blocked_navigating",
		});
	});

	it("blocks focus revalidate while submitting", () => {
		const plan = decideFocusRevalidationTriggerExecutionPlan({
			status: {
				isNavigating: false,
				isSubmitting: true,
				isRevalidating: false,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(plan).toEqual({
			type: "skip",
			reason: "focus_revalidate_blocked_submitting",
		});
	});

	it("blocks focus revalidate while a revalidation is in flight", () => {
		const plan = decideFocusRevalidationTriggerExecutionPlan({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: true,
			},
			nowTimestampMS: 1000,
			lastTriggeredNavOrRevalidateTimestampMS: 0,
			staleTimeMS: 0,
		});

		expect(plan).toEqual({
			type: "skip",
			reason: "focus_revalidate_blocked_revalidating",
		});
	});

	it("blocks focus revalidate when stale window has not elapsed", () => {
		const plan = decideFocusRevalidationTriggerExecutionPlan({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1049,
			lastTriggeredNavOrRevalidateTimestampMS: 1000,
			staleTimeMS: 50,
		});

		expect(plan).toEqual({
			type: "skip",
			reason: "focus_revalidate_blocked_stale_window_not_elapsed",
		});
	});

	it("allows focus revalidate exactly at stale window boundary", () => {
		const plan = decideFocusRevalidationTriggerExecutionPlan({
			status: {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			},
			nowTimestampMS: 1050,
			lastTriggeredNavOrRevalidateTimestampMS: 1000,
			staleTimeMS: 50,
		});

		expect(plan).toEqual({
			type: "trigger_revalidate",
			reason: "focus_revalidate_allowed",
		});
	});
});

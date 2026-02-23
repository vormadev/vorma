import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	computeNavigationStatus,
	createRuntimeLanes,
	createStatusSignaler,
	type RuntimeLanes,
} from "../../core/navigation/runtime_slots.ts";
import type {
	NavigationEntry,
	SubmissionEntry,
} from "../../core/navigation/types.ts";

function createNavigationEntry(props: {
	operationID: number;
	intent: NavigationEntry["intent"];
	phase: NavigationEntry["phase"];
	type?: NavigationEntry["type"];
	targetUrl?: string;
}): NavigationEntry {
	return {
		operationID: props.operationID,
		control: {
			abortController: undefined,
			promise: new Promise<never>(() => {}),
		},
		type: props.type ?? "userNavigation",
		intent: props.intent,
		phase: props.phase,
		startTime: 0,
		targetUrl:
			props.targetUrl ?? `http://localhost:3000/${props.operationID}`,
		originUrl: "http://localhost:3000/",
	};
}

function createSubmissionEntry(props: {
	operationID: number;
	skipGlobalLoadingIndicator?: boolean;
}): SubmissionEntry {
	return {
		operationID: props.operationID,
		control: {
			abortController: undefined,
			promise: Promise.resolve(undefined),
		},
		startTime: 0,
		skipGlobalLoadingIndicator: props.skipGlobalLoadingIndicator,
	};
}

function buildBaseRuntimeLanes(): RuntimeLanes {
	return createRuntimeLanes();
}

describe("navigation runtime slots internals", () => {
	it("treats active non-complete user navigation as navigating", () => {
		const lanes = buildBaseRuntimeLanes();
		lanes.active = createNavigationEntry({
			operationID: 1,
			intent: "navigate",
			phase: "fetching",
		});

		expect(computeNavigationStatus({ lanes })).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not treat non-navigate active entries as navigating", () => {
		const lanes = buildBaseRuntimeLanes();
		lanes.active = createNavigationEntry({
			operationID: 1,
			type: "prefetch",
			intent: "none",
			phase: "fetching",
		});

		expect(computeNavigationStatus({ lanes })).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("reports revalidation and global submission status independently", () => {
		const lanes = buildBaseRuntimeLanes();
		lanes.revalidation = createNavigationEntry({
			operationID: 2,
			type: "revalidation",
			intent: "revalidate",
			phase: "waiting",
		});
		lanes.submissions.set(
			"skip",
			createSubmissionEntry({
				operationID: 10,
				skipGlobalLoadingIndicator: true,
			}),
		);
		lanes.submissions.set(
			"show",
			createSubmissionEntry({
				operationID: 11,
				skipGlobalLoadingIndicator: false,
			}),
		);

		expect(computeNavigationStatus({ lanes })).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: true,
		});
	});

	describe("status signaler", () => {
		beforeEach(() => {
			vi.useFakeTimers();
		});

		afterEach(() => {
			vi.useRealTimers();
		});

		it("deduplicates unchanged status snapshots", async () => {
			let status = {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			};
			const dispatchStatusEvent = vi.fn();
			const scheduleStatusUpdate = createStatusSignaler({
				getStatus: () => status,
				dispatchStatusEvent,
				debounceMS: 4,
			});

			scheduleStatusUpdate();
			scheduleStatusUpdate();
			await vi.advanceTimersByTimeAsync(5);

			expect(dispatchStatusEvent).toHaveBeenCalledTimes(1);
			expect(dispatchStatusEvent).toHaveBeenLastCalledWith(status);

			scheduleStatusUpdate();
			await vi.advanceTimersByTimeAsync(5);
			expect(dispatchStatusEvent).toHaveBeenCalledTimes(1);
		});

		it("dispatches when status changes", async () => {
			let status = {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			};
			const dispatchStatusEvent = vi.fn();
			const scheduleStatusUpdate = createStatusSignaler({
				getStatus: () => status,
				dispatchStatusEvent,
				debounceMS: 4,
			});

			scheduleStatusUpdate();
			await vi.advanceTimersByTimeAsync(5);
			expect(dispatchStatusEvent).toHaveBeenCalledTimes(1);

			status = {
				isNavigating: false,
				isSubmitting: true,
				isRevalidating: false,
			};
			scheduleStatusUpdate();
			await vi.advanceTimersByTimeAsync(5);

			expect(dispatchStatusEvent).toHaveBeenCalledTimes(2);
			expect(dispatchStatusEvent).toHaveBeenLastCalledWith(status);
		});
	});
});

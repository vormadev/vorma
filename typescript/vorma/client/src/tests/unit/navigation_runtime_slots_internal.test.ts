import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SubmissionEntry } from "../../../src/runtime.ts";
import {
	computeNavigationStatus,
	createInitialNavigationRuntimeEngineState,
	createRuntimeLanes,
	createStatusSignaler,
} from "../../runtime.ts";

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

function buildStatusInputs() {
	return {
		runtimeEngineState: createInitialNavigationRuntimeEngineState(),
		lanes: createRuntimeLanes(),
	};
}

describe("navigation runtime slots internals", () => {
	it("treats active non-complete user navigation as navigating", () => {
		const { runtimeEngineState, lanes } = buildStatusInputs();
		runtimeEngineState.lanes.navigate = {
			targetUrl: "http://localhost:3000/1",
			operationID: 1,
			phase: "fetching",
			ownership: "current",
		};

		expect(
			computeNavigationStatus({
				runtimeEngineState,
				submissions: lanes.submissions,
			}),
		).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not treat non-navigate active entries as navigating", () => {
		const { runtimeEngineState, lanes } = buildStatusInputs();
		runtimeEngineState.lanes.prefetch.set("http://localhost:3000/1", {
			targetUrl: "http://localhost:3000/1",
			operationID: 1,
			phase: "fetching",
			ownership: "current",
		});

		expect(
			computeNavigationStatus({
				runtimeEngineState,
				submissions: lanes.submissions,
			}),
		).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("reports revalidation and global submission status independently", () => {
		const { runtimeEngineState, lanes } = buildStatusInputs();
		runtimeEngineState.lanes.revalidate = {
			targetUrl: "http://localhost:3000/revalidate",
			operationID: 2,
			phase: "waiting",
			ownership: "current",
		};
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

		expect(
			computeNavigationStatus({
				runtimeEngineState,
				submissions: lanes.submissions,
			}),
		).toEqual({
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

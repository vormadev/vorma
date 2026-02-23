import { describe, expect, it, vi } from "vitest";
import { createNavigationLifecycleRuntime } from "../../core/navigation/runtime_lifecycle_runtime.ts";
import type { NavigationLanes } from "../../core/navigation/runtime_slots.ts";
import type {
	NavigationEntry,
	SubmissionEntry,
} from "../../core/navigation/types.ts";

function createNavigationEntry(props: {
	operationID: number;
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
	phase?: NavigationEntry["phase"];
}): NavigationEntry {
	return {
		operationID: props.operationID,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: props.type,
		intent: props.intent,
		phase: props.phase || "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl,
		originUrl: window.location.href,
	};
}

function createSubmissionEntry(operationID: number): SubmissionEntry {
	return {
		operationID,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve(undefined),
		},
		startTime: Date.now(),
	};
}

describe("navigation lifecycle runtime seam", () => {
	it("applies removals/transitions and records debug journal entries", () => {
		const activeEntry = createNavigationEntry({
			operationID: 1,
			targetUrl: "http://localhost:3000/runtime-seam",
			type: "browserHistory",
			intent: "navigate",
			phase: "fetching",
		});
		const slots: NavigationLanes = {
			active: activeEntry,
			prefetch: new Map(),
			revalidation: null,
		};
		const scheduleStatusUpdate = vi.fn();
		const lifecycleRuntime = createNavigationLifecycleRuntime({
			lanes: slots,
			getScheduleStatusUpdate: () => scheduleStatusUpdate,
		});

		lifecycleRuntime.transitionPhase({
			targetUrl: activeEntry.targetUrl,
			phase: "waiting",
			reason: "runtime_seam_waiting",
		});
		expect(activeEntry.phase).toBe("waiting");

		const deleted = lifecycleRuntime.deleteNavigation({
			key: activeEntry.targetUrl,
			reason: "runtime_seam_deleted",
		});
		expect(deleted).toBe(true);
		expect(slots.active).toBeNull();
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(2);

		const journal = lifecycleRuntime.getDebugJournal();
		expect(
			journal.some((entry) => entry.reason === "runtime_seam_waiting"),
		).toBe(true);
		expect(
			journal.some((entry) => entry.reason === "runtime_seam_deleted"),
		).toBe(true);
	});

	it("dispatches begin/submission/clear-all transition events into debug journal", () => {
		const winnerEntry = createNavigationEntry({
			operationID: 2,
			targetUrl: "http://localhost:3000/begin-winner",
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
		});
		const beforeEntry = createNavigationEntry({
			operationID: 3,
			targetUrl: "http://localhost:3000/begin-before",
			type: "prefetch",
			intent: "none",
			phase: "waiting",
		});
		const slots: NavigationLanes = {
			active: winnerEntry,
			prefetch: new Map(),
			revalidation: null,
		};
		const lifecycleRuntime = createNavigationLifecycleRuntime({
			lanes: slots,
			getScheduleStatusUpdate: () => () => {},
		});

		lifecycleRuntime.dispatchBeginNavigationArbitrated({
			navigationType: "userNavigation",
			targetUrl: winnerEntry.targetUrl,
			beforeEntriesByOperationID: new Map([
				[beforeEntry.operationID, beforeEntry],
			]),
		});

		const submissionEntry = createSubmissionEntry(10);
		lifecycleRuntime.dispatchSubmissionStateTransition({
			submissionEntry,
			targetUrl: "http://localhost:3000/current",
			fromState: "submitting",
			toState: "removed",
			reason: "runtime_seam_submission",
		});

		lifecycleRuntime.dispatchClearAll({
			navigationEntries: [winnerEntry],
			submissionEntries: [submissionEntry],
			targetUrl: "http://localhost:3000/current",
		});

		const journal = lifecycleRuntime.getDebugJournal();
		expect(
			journal.some(
				(entry) => entry.reason === "superseded_by_userNavigation",
			),
		).toBe(true);
		expect(
			journal.some((entry) => entry.reason === "runtime_seam_submission"),
		).toBe(true);
		expect(journal.some((entry) => entry.reason === "clear_all")).toBe(
			true,
		);
	});

	it("keeps lifecycle behavior intact while debug journaling is disabled", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		vi.resetModules();

		const lifecycleRuntimeModule =
			await import("../../core/navigation/runtime_lifecycle_runtime.ts");
		const activeEntry = createNavigationEntry({
			operationID: 21,
			targetUrl: "http://localhost:3000/runtime-seam-disabled",
			type: "browserHistory",
			intent: "navigate",
			phase: "fetching",
		});
		const slots: NavigationLanes = {
			active: activeEntry,
			prefetch: new Map(),
			revalidation: null,
		};
		const scheduleStatusUpdate = vi.fn();
		try {
			const lifecycleRuntime =
				lifecycleRuntimeModule.createNavigationLifecycleRuntime({
					lanes: slots,
					getScheduleStatusUpdate: () => scheduleStatusUpdate,
				});

			lifecycleRuntime.transitionPhase({
				targetUrl: activeEntry.targetUrl,
				phase: "waiting",
				reason: "runtime_seam_disabled_waiting",
			});
			expect(activeEntry.phase).toBe("waiting");

			const deleted = lifecycleRuntime.deleteNavigation({
				key: activeEntry.targetUrl,
				reason: "runtime_seam_disabled_deleted",
			});
			expect(deleted).toBe(true);
			expect(slots.active).toBeNull();
			expect(scheduleStatusUpdate).toHaveBeenCalledTimes(2);
			expect(lifecycleRuntime.getDebugJournal()).toEqual([]);

			expect(() => lifecycleRuntime.clearDebugJournal()).not.toThrow();
			expect(lifecycleRuntime.getDebugJournal()).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});
});

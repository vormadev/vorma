import { describe, expect, it, vi } from "vitest";
import type {
	NavigationEntry,
	NavigationLanes,
	SubmissionEntry,
} from "../../../src/runtime.ts";
import {
	applyNavigationRemovalLifecycleTransition,
	buildClearAllLifecycleTransitionEvent,
	buildNavigationBeginArbitratedLifecycleTransitionEvent,
	buildNavigationFailureLifecycleTransitionEvent,
	buildNavigationPhaseLifecycleTransitionEvent,
	buildSubmissionStateLifecycleTransitionEvent,
} from "../../runtime.ts";

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

describe("runtime lifecycle transitions", () => {
	it("applies removal transition and returns a navigation_removed event", () => {
		const entry = createNavigationEntry({
			operationID: 1,
			targetUrl: "http://localhost:3000/remove-me",
			type: "browserHistory",
			intent: "navigate",
		});
		const slots: NavigationLanes = {
			active: entry,
			prefetch: new Map(),
			revalidation: null,
		};
		const scheduleStatusUpdate = vi.fn();

		const result = applyNavigationRemovalLifecycleTransition({
			lanes: slots,
			targetUrl: "http://localhost:3000/remove-me",
			scheduleStatusUpdate,
			reason: "test_remove",
			causedByOperationID: 99,
		});

		expect(result.deleted).toBe(true);
		expect(result.transitionEvent).toEqual({
			type: "navigation_removed",
			entry,
			reason: "test_remove",
			causedByOperationID: 99,
		});
		expect(slots.active).toBeNull();
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("sets causedByOperationID to null when not provided for removal transitions", () => {
		const entry = createNavigationEntry({
			operationID: 4,
			targetUrl: "http://localhost:3000/remove-me-no-cause",
			type: "browserHistory",
			intent: "navigate",
		});
		const slots: NavigationLanes = {
			active: entry,
			prefetch: new Map(),
			revalidation: null,
		};
		const scheduleStatusUpdate = vi.fn();

		const result = applyNavigationRemovalLifecycleTransition({
			lanes: slots,
			targetUrl: "http://localhost:3000/remove-me-no-cause",
			scheduleStatusUpdate,
			reason: "test_remove_no_cause",
		});

		expect(result.deleted).toBe(true);
		expect(result.transitionEvent).toEqual({
			type: "navigation_removed",
			entry,
			reason: "test_remove_no_cause",
			causedByOperationID: null,
		});
		expect(slots.active).toBeNull();
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("no-ops removal transition when target is missing", () => {
		const slots: NavigationLanes = {
			active: null,
			prefetch: new Map(),
			revalidation: null,
		};
		const scheduleStatusUpdate = vi.fn();

		const result = applyNavigationRemovalLifecycleTransition({
			lanes: slots,
			targetUrl: "http://localhost:3000/not-found",
			scheduleStatusUpdate,
			reason: "test_remove_missing",
		});

		expect(result).toEqual({
			deleted: false,
			transitionEvent: null,
		});
		expect(scheduleStatusUpdate).not.toHaveBeenCalled();
	});

	it("builds navigation phase transition events from explicit phases", () => {
		const entry = createNavigationEntry({
			operationID: 2,
			targetUrl: "http://localhost:3000/phase-me",
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
		});
		const transitionEvent = buildNavigationPhaseLifecycleTransitionEvent({
			entry,
			fromPhase: "fetching",
			toPhase: "waiting",
			reason: "test_phase_waiting",
		});

		expect(transitionEvent).toEqual({
			type: "navigation_phase_transitioned",
			entry,
			fromPhase: "fetching",
			toPhase: "waiting",
			reason: "test_phase_waiting",
		});
	});

	it("allows same-phase transition events when reason logging is needed", () => {
		const entry = createNavigationEntry({
			operationID: 3,
			targetUrl: "http://localhost:3000/phase-unchanged",
			type: "prefetch",
			intent: "none",
			phase: "waiting",
		});
		const transitionEvent = buildNavigationPhaseLifecycleTransitionEvent({
			entry,
			fromPhase: "waiting",
			toPhase: "waiting",
			reason: "test_phase_unchanged",
		});

		expect(transitionEvent).toEqual({
			type: "navigation_phase_transitioned",
			entry,
			fromPhase: "waiting",
			toPhase: "waiting",
			reason: "test_phase_unchanged",
		});
	});

	it("builds begin-arbitrated transition event with updated after snapshot", () => {
		const beforeEntry = createNavigationEntry({
			operationID: 10,
			targetUrl: "http://localhost:3000/before",
			type: "prefetch",
			intent: "none",
		});
		const winnerEntry = createNavigationEntry({
			operationID: 11,
			targetUrl: "http://localhost:3000/winner",
			type: "browserHistory",
			intent: "navigate",
		});
		const slots: NavigationLanes = {
			active: winnerEntry,
			prefetch: new Map(),
			revalidation: null,
		};
		const event = buildNavigationBeginArbitratedLifecycleTransitionEvent({
			navigationType: "browserHistory",
			targetUrl: "http://localhost:3000/winner",
			beforeEntriesByOperationID: new Map([[10, beforeEntry]]),
			lanes: slots,
			winnerEntry,
		});

		expect(event.type).toBe("navigation_begin_arbitrated");
		if (event.type !== "navigation_begin_arbitrated") {
			return;
		}
		expect(event.beforeEntriesByOperationID.size).toBe(1);
		expect(event.afterEntriesByOperationID.size).toBe(1);
		expect(event.afterEntriesByOperationID.get(11)).toBe(winnerEntry);
		expect(event.winnerEntry).toBe(winnerEntry);
	});

	it("builds submission and clear-all transition events", () => {
		const submissionEntry = createSubmissionEntry(20);
		const navigationEntry = createNavigationEntry({
			operationID: 21,
			targetUrl: "http://localhost:3000/clear-all-nav",
			type: "userNavigation",
			intent: "navigate",
		});

		const submissionEvent = buildSubmissionStateLifecycleTransitionEvent({
			submissionEntry,
			targetUrl: "http://localhost:3000/current",
			fromState: "submitting",
			toState: "removed",
			reason: "submission_finished",
			causedByOperationID: null,
		});
		expect(submissionEvent).toEqual({
			type: "submission_state_transitioned",
			submissionEntry,
			targetUrl: "http://localhost:3000/current",
			fromState: "submitting",
			toState: "removed",
			reason: "submission_finished",
			causedByOperationID: null,
		});

		const clearAllEvent = buildClearAllLifecycleTransitionEvent({
			navigationEntries: [navigationEntry],
			submissionEntries: [submissionEntry],
			targetUrl: "http://localhost:3000/current",
		});
		expect(clearAllEvent).toEqual({
			type: "clear_all",
			navigationEntries: [navigationEntry],
			submissionEntries: [submissionEntry],
			targetUrl: "http://localhost:3000/current",
		});
	});

	it("builds navigation-failed transition events", () => {
		const entry = createNavigationEntry({
			operationID: 40,
			targetUrl: "http://localhost:3000/reducer-failed",
			type: "userNavigation",
			intent: "navigate",
			phase: "waiting",
		});

		const transitionEvent = buildNavigationFailureLifecycleTransitionEvent({
			targetUrl: entry.targetUrl,
			entry,
			reason: "reducer_failed",
		});

		expect(transitionEvent).toEqual({
			type: "navigation_failed",
			targetUrl: entry.targetUrl,
			entry,
			reason: "reducer_failed",
		});
	});
});

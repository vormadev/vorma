import { describe, expect, it, vi } from "vitest";
import {
	beginSubmissionLifecycle,
	finishSubmissionLifecycle,
	type SubmitExecutionContext,
} from "../../core/navigation/runtime_submit.ts";
import type { SubmissionEntry } from "../../core/navigation/types.ts";

let nextSubmissionOperationID = 1;

function createSubmissionEntry(): SubmissionEntry {
	return {
		operationID: nextSubmissionOperationID++,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve(undefined),
		},
		startTime: Date.now(),
	};
}

function createSubmitExecutionContext(): {
	context: SubmitExecutionContext;
	transitionEvents: Array<{
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}>;
	scheduleStatusUpdate: ReturnType<typeof vi.fn>;
} {
	const transitionEvents: Array<{
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}> = [];
	const scheduleStatusUpdate = vi.fn();
	const context: SubmitExecutionContext = {
		submissions: new Map(),
		scheduleStatusUpdate,
		allocateSubmissionOperationID: () => 1,
		onSubmissionStateTransition: (props) => {
			transitionEvents.push(props);
		},
		navigate: async () => ({ didNavigate: false }),
	};

	return {
		context,
		transitionEvents,
		scheduleStatusUpdate,
	};
}

describe("submission lifecycle execution", () => {
	it("begins with dedupe abort transition when a keyed submission already exists", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const existingSubmissionEntry = createSubmissionEntry();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, existingSubmissionEntry);
		const existingAbortSpy = vi.spyOn(
			existingSubmissionEntry.control.abortController!,
			"abort",
		);

		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry,
		});

		expect(existingAbortSpy).toHaveBeenCalledWith("deduped");
		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([
			{
				submissionEntry: existingSubmissionEntry,
				fromState: "submitting",
				toState: "aborted",
				reason: "submission_deduped_by_newer_submission",
				causedByOperationID: submissionEntry.operationID,
			},
			{
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("begins unique submissions without a dedupe abort transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = Symbol("submission");

		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry: undefined,
		});

		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([
			{
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("finishes by removing current submissions and emitting removal transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, submissionEntry);

		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: true,
		});

		expect(context.submissions.has(submissionKey)).toBe(false);
		expect(transitionEvents).toEqual([
			{
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: "submission_finished",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("finishes stale submissions without removing or emitting removal transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, submissionEntry);

		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: false,
		});

		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});
});

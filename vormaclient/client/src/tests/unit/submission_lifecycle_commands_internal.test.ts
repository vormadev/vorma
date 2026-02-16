import { describe, expect, it } from "vitest";
import {
	buildSubmissionLifecycleBeginCommands,
	buildSubmissionLifecycleFinishCommands,
} from "../../core/navigation/runtime_submit_lifecycle_commands.ts";
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

describe("submission lifecycle command builders", () => {
	it("builds begin commands with dedupe abort when an existing dedupe-key submission exists", () => {
		const existingSubmissionEntry = createSubmissionEntry();
		const submissionEntry = createSubmissionEntry();
		const commands = buildSubmissionLifecycleBeginCommands({
			submissionKey: "submission:users:save",
			submissionEntry,
			existingSubmissionEntry,
		});

		expect(commands).toEqual([
			{
				type: "abort_submission_entry",
				submissionEntry: existingSubmissionEntry,
				reason: "submission_deduped_by_newer_submission",
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry: existingSubmissionEntry,
				fromState: "submitting",
				toState: "aborted",
				reason: "submission_deduped_by_newer_submission",
				causedByOperationID: submissionEntry.operationID,
			},
			{
				type: "set_submission_entry",
				submissionKey: "submission:users:save",
				submissionEntry,
				reason: "submission_started",
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
			},
			{
				type: "schedule_status_update",
				reason: "submission_started",
			},
		]);
	});

	it("builds begin commands without dedupe abort for unique submissions", () => {
		const submissionEntry = createSubmissionEntry();
		const submissionKey = Symbol("submission");
		const commands = buildSubmissionLifecycleBeginCommands({
			submissionKey,
			submissionEntry,
			existingSubmissionEntry: undefined,
		});

		expect(commands).toEqual([
			{
				type: "set_submission_entry",
				submissionKey,
				submissionEntry,
				reason: "submission_started",
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
			},
			{
				type: "schedule_status_update",
				reason: "submission_started",
			},
		]);
	});

	it("builds finish commands with removal transitions when submission is current", () => {
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		const commands = buildSubmissionLifecycleFinishCommands({
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: true,
		});

		expect(commands).toEqual([
			{
				type: "delete_submission_entry",
				submissionKey,
				submissionEntry,
				reason: "submission_finished",
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: "submission_finished",
			},
			{
				type: "schedule_status_update",
				reason: "submission_finished",
			},
		]);
	});

	it("builds finish commands without removal transitions when submission is stale", () => {
		const submissionEntry = createSubmissionEntry();
		const commands = buildSubmissionLifecycleFinishCommands({
			submissionKey: "submission:users:save",
			submissionEntry,
			shouldRemoveSubmissionEntry: false,
		});

		expect(commands).toEqual([
			{
				type: "schedule_status_update",
				reason: "submission_finished",
			},
		]);
	});
});

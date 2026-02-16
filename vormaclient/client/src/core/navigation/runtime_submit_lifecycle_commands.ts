import type { SubmissionEntry } from "./types.ts";

export type SubmissionLifecycleCommand =
	| {
			type: "abort_submission_entry";
			submissionEntry: SubmissionEntry;
			reason: "submission_deduped_by_newer_submission";
	  }
	| {
			type: "set_submission_entry";
			submissionKey: string | symbol;
			submissionEntry: SubmissionEntry;
			reason: "submission_started";
	  }
	| {
			type: "delete_submission_entry";
			submissionKey: string | symbol;
			submissionEntry: SubmissionEntry;
			reason: "submission_finished";
	  }
	| {
			type: "emit_submission_state_transition";
			submissionEntry: SubmissionEntry;
			fromState: string;
			toState: string;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "schedule_status_update";
			reason:
				| "submission_started"
				| "submission_finished"
				| "submission_deduped_by_newer_submission";
	  };

export function buildSubmissionLifecycleBeginCommands(props: {
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	existingSubmissionEntry: SubmissionEntry | undefined;
}): SubmissionLifecycleCommand[] {
	const { submissionKey, submissionEntry, existingSubmissionEntry } = props;
	const commands: SubmissionLifecycleCommand[] = [];

	if (existingSubmissionEntry) {
		commands.push(
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
		);
	}

	commands.push(
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
	);

	return commands;
}

export function buildSubmissionLifecycleFinishCommands(props: {
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	shouldRemoveSubmissionEntry: boolean;
}): SubmissionLifecycleCommand[] {
	const { submissionKey, submissionEntry, shouldRemoveSubmissionEntry } =
		props;
	const commands: SubmissionLifecycleCommand[] = [];

	if (shouldRemoveSubmissionEntry) {
		commands.push(
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
		);
	}

	commands.push({
		type: "schedule_status_update",
		reason: "submission_finished",
	});

	return commands;
}

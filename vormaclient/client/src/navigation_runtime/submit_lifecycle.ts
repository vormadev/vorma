import type { SubmitOptions, SubmissionEntry } from "./types.ts";

export type SubmitLifecycleContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
};

export type ActiveSubmission = {
	abortController: AbortController;
	submissionKey: string | symbol;
	entry: SubmissionEntry;
};

export function createActiveSubmission(
	options?: SubmitOptions,
): ActiveSubmission {
	const abortController = new AbortController();
	const submissionKey = options?.dedupeKey
		? `submission:${options.dedupeKey}`
		: Symbol("submission");

	const entry: SubmissionEntry = {
		control: {
			abortController,
			promise: Promise.resolve() as any,
		},
		startTime: Date.now(),
		skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
	};

	return { abortController, submissionKey, entry };
}

export function beginSubmissionLifecycle(
	context: SubmitLifecycleContext,
	activeSubmission: ActiveSubmission,
): void {
	// Abort duplicate submission
	if (typeof activeSubmission.submissionKey === "string") {
		const existing = context.submissions.get(
			activeSubmission.submissionKey,
		);
		if (existing) {
			existing.control.abortController?.abort("deduped");
		}
	}

	context.submissions.set(
		activeSubmission.submissionKey,
		activeSubmission.entry,
	);
	context.scheduleStatusUpdate();
}

export function finishSubmissionLifecycle(
	context: SubmitLifecycleContext,
	activeSubmission: ActiveSubmission,
): void {
	// A deduped replacement may have reused the same key. Only remove
	// the entry if this submission still owns the key.
	if (
		context.submissions.get(activeSubmission.submissionKey) ===
		activeSubmission.entry
	) {
		context.submissions.delete(activeSubmission.submissionKey);
	}

	context.scheduleStatusUpdate();
}

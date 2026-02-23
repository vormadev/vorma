import { getIsGETRequest, resolveAbsoluteHref } from "vorma/kit/url";
import { __vormaClientGlobal } from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import {
	effectuateRedirectDataResult,
	handleRedirects,
	type RedirectData,
} from "../redirects.ts";
import { syncBuildIDFromResponse } from "./runtime_navigation_successful_runtime.ts";
import type { NavigateProps, SubmissionEntry, SubmitOptions } from "./types.ts";
import { hasSubmissionOperationOwnership } from "./types.ts";

type SubmissionLifecycle = {
	abortController: AbortController;
	isCurrent: () => boolean;
	begin: () => void;
	finish: () => void;
};

export type SubmitStalenessCheckpoint =
	| "post_request"
	| "pre_finalize"
	| "post_response_classification"
	| "post_redirect_effectuation"
	| "pre_success_return"
	| "post_auto_revalidate";

const submitStalenessContinueReasonByCheckpoint = {
	post_request: "submit_staleness_post_request_continue_current",
	pre_finalize: "submit_staleness_pre_finalize_continue_current",
	post_response_classification:
		"submit_staleness_post_response_classification_continue_current",
	post_redirect_effectuation:
		"submit_staleness_post_redirect_effectuation_continue_current",
	pre_success_return: "submit_staleness_pre_success_return_continue_current",
	post_auto_revalidate:
		"submit_staleness_post_auto_revalidate_continue_current",
} as const satisfies Record<SubmitStalenessCheckpoint, string>;

const submitStalenessStopReasonByCheckpoint = {
	post_request: "submit_staleness_post_request_stop_not_current",
	pre_finalize: "submit_staleness_pre_finalize_stop_not_current",
	post_response_classification:
		"submit_staleness_post_response_classification_stop_not_current",
	post_redirect_effectuation:
		"submit_staleness_post_redirect_effectuation_stop_not_current",
	pre_success_return: "submit_staleness_pre_success_return_stop_not_current",
	post_auto_revalidate:
		"submit_staleness_post_auto_revalidate_stop_not_current",
} as const satisfies Record<SubmitStalenessCheckpoint, string>;

type SubmitStalenessContinueReason =
	(typeof submitStalenessContinueReasonByCheckpoint)[SubmitStalenessCheckpoint];
type SubmitStalenessStopReason =
	(typeof submitStalenessStopReasonByCheckpoint)[SubmitStalenessCheckpoint];

const submissionLifecycleReason = {
	dedupedByNewerSubmission: "submission_deduped_by_newer_submission",
	started: "submission_started",
	finished: "submission_finished",
} as const;

export type SubmitStalenessCheckpointExecutionPlan =
	| {
			checkpoint: SubmitStalenessCheckpoint;
			type: "continue";
			reason: SubmitStalenessContinueReason;
	  }
	| {
			checkpoint: SubmitStalenessCheckpoint;
			type: "stop";
			reason: SubmitStalenessStopReason;
	  };

export function decideSubmitStalenessCheckpointExecutionPlan(props: {
	checkpoint: SubmitStalenessCheckpoint;
	isSubmissionCurrent: boolean;
}): SubmitStalenessCheckpointExecutionPlan {
	if (props.isSubmissionCurrent) {
		return {
			checkpoint: props.checkpoint,
			type: "continue",
			reason: submitStalenessContinueReasonByCheckpoint[props.checkpoint],
		};
	}

	return {
		checkpoint: props.checkpoint,
		type: "stop",
		reason: submitStalenessStopReasonByCheckpoint[props.checkpoint],
	};
}

export type SubmissionLifecycleCommand =
	| {
			type: "abort_submission_entry";
			submissionEntry: SubmissionEntry;
			reason: typeof submissionLifecycleReason.dedupedByNewerSubmission;
	  }
	| {
			type: "set_submission_entry";
			submissionKey: string | symbol;
			submissionEntry: SubmissionEntry;
			reason: typeof submissionLifecycleReason.started;
	  }
	| {
			type: "delete_submission_entry";
			submissionKey: string | symbol;
			submissionEntry: SubmissionEntry;
			reason: typeof submissionLifecycleReason.finished;
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
				| typeof submissionLifecycleReason.started
				| typeof submissionLifecycleReason.finished
				| typeof submissionLifecycleReason.dedupedByNewerSubmission;
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
				reason: submissionLifecycleReason.dedupedByNewerSubmission,
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry: existingSubmissionEntry,
				fromState: "submitting",
				toState: "aborted",
				reason: submissionLifecycleReason.dedupedByNewerSubmission,
				causedByOperationID: submissionEntry.operationID,
			},
		);
	}

	commands.push(
		{
			type: "set_submission_entry",
			submissionKey,
			submissionEntry,
			reason: submissionLifecycleReason.started,
		},
		{
			type: "emit_submission_state_transition",
			submissionEntry,
			fromState: "none",
			toState: "submitting",
			reason: submissionLifecycleReason.started,
		},
		{
			type: "schedule_status_update",
			reason: submissionLifecycleReason.started,
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
				reason: submissionLifecycleReason.finished,
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: submissionLifecycleReason.finished,
			},
		);
	}

	commands.push({
		type: "schedule_status_update",
		reason: submissionLifecycleReason.finished,
	});

	return commands;
}

function createSubmissionEntry(
	abortController: AbortController,
	operationID: number,
	options?: SubmitOptions,
): SubmissionEntry {
	return {
		operationID,
		control: {
			abortController,
			promise: Promise.resolve() as Promise<unknown>,
		},
		startTime: Date.now(),
		skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
	};
}

function executeSubmissionLifecycleCommands(props: {
	context: SubmitExecutionContext;
	commands: SubmissionLifecycleCommand[];
}): void {
	const { context, commands } = props;

	for (const command of commands) {
		switch (command.type) {
			case "abort_submission_entry":
				command.submissionEntry.control.abortController?.abort(
					"deduped",
				);
				break;
			case "set_submission_entry":
				context.submissions.set(
					command.submissionKey,
					command.submissionEntry,
				);
				break;
			case "delete_submission_entry":
				context.submissions.delete(command.submissionKey);
				break;
			case "emit_submission_state_transition":
				context.onSubmissionStateTransition?.({
					submissionEntry: command.submissionEntry,
					fromState: command.fromState,
					toState: command.toState,
					reason: command.reason,
					causedByOperationID: command.causedByOperationID ?? null,
				});
				break;
			case "schedule_status_update":
				context.scheduleStatusUpdate();
				break;
		}
	}
}

export type SubmitExecutionContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	allocateSubmissionOperationID: () => number;
	onSubmissionStateTransition?: (props: {
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}) => void;
	navigate: (props: NavigateProps) => Promise<{
		didNavigate: boolean;
	}>;
};

function createSubmissionLifecycle(
	context: SubmitExecutionContext,
	options?: SubmitOptions,
): SubmissionLifecycle {
	const abortController = new AbortController();
	const submissionEntry = createSubmissionEntry(
		abortController,
		context.allocateSubmissionOperationID(),
		options,
	);
	const submissionKey = options?.dedupeKey
		? `submission:${options.dedupeKey}`
		: Symbol("submission");

	const isCurrent = (): boolean =>
		hasSubmissionOperationOwnership({
			entry: context.submissions.get(submissionKey),
			expectedOperationID: submissionEntry.operationID,
		});

	const begin = (): void => {
		const existingSubmissionEntry =
			typeof submissionKey === "string"
				? context.submissions.get(submissionKey)
				: undefined;
		executeSubmissionLifecycleCommands({
			context,
			commands: buildSubmissionLifecycleBeginCommands({
				submissionKey,
				submissionEntry,
				existingSubmissionEntry,
			}),
		});
	};

	const finish = (): void => {
		executeSubmissionLifecycleCommands({
			context,
			commands: buildSubmissionLifecycleFinishCommands({
				submissionKey,
				submissionEntry,
				shouldRemoveSubmissionEntry: isCurrent(),
			}),
		});
	};

	return {
		abortController,
		isCurrent,
		begin,
		finish,
	};
}

function buildSubmitRequestInit(props: {
	requestInit?: RequestInit;
	signal: AbortSignal;
}): RequestInit {
	const { requestInit, signal } = props;
	const headers = new Headers(requestInit?.headers);
	const deploymentID = __vormaClientGlobal.get("deploymentID");
	if (deploymentID) {
		headers.set("x-deployment-id", deploymentID);
	}

	return {
		...requestInit,
		headers,
		signal,
	};
}

async function executeSubmitRequest(props: {
	abortController: AbortController;
	url: URL;
	requestInit: RequestInit;
}): Promise<{ redirectData: RedirectData | null; response: Response }> {
	const result = await handleRedirects({
		abortController: props.abortController,
		url: props.url,
		redirectCount: 0,
		requestInit: props.requestInit,
	});

	if (!result.response) {
		throw new Error("Submit request completed without a response.");
	}

	return {
		redirectData: result.redirectData,
		response: result.response,
	};
}

type PreparedSubmitRequest = {
	url: URL;
	requestInit: RequestInit;
};

function prepareSubmitRequest(props: {
	url: string | URL;
	requestInit?: RequestInit;
	signal: AbortSignal;
}): PreparedSubmitRequest {
	const { url, requestInit, signal } = props;
	return {
		url: new URL(resolveAbsoluteHref({ href: url })),
		requestInit: buildSubmitRequestInit({
			requestInit,
			signal,
		}),
	};
}

type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

function getAbortedSubmitResult<T>(): SubmitResult<T> {
	return { success: false, error: "Aborted" };
}

function getUnknownSubmitErrorResult<T>(): SubmitResult<T> {
	return { success: false, error: "Unknown error" };
}

function getSubmitErrorResult<T>(error: string): SubmitResult<T> {
	return { success: false, error };
}

function getSubmitRedirectFailureResult<T>(): SubmitResult<T> {
	return getSubmitErrorResult<T>("Redirect failed");
}

function getStaleSubmitResultFromCheckpointIfAny<T>(props: {
	checkpoint: SubmitStalenessCheckpoint;
	isSubmissionCurrent: () => boolean;
}): SubmitResult<T> | null {
	const stalenessCheckpointExecutionPlan =
		decideSubmitStalenessCheckpointExecutionPlan({
			checkpoint: props.checkpoint,
			isSubmissionCurrent: props.isSubmissionCurrent(),
		});
	if (stalenessCheckpointExecutionPlan.type === "continue") {
		return null;
	}

	return getAbortedSubmitResult<T>();
}

function shouldAutoRevalidateSubmitResult(props: {
	requestInit?: RequestInit;
	redirectData: RedirectData | null;
	options?: SubmitOptions;
}): boolean {
	const { requestInit, redirectData, options } = props;
	const isGET = getIsGETRequest(requestInit);
	const redirected = redirectData?.status === "did";
	return !isGET && !redirected && options?.revalidate !== false;
}

function hasNoContentResponseBody(response: Response): boolean {
	if (response.status === 204 || response.status === 205) {
		return true;
	}

	return response.headers.get("content-length") === "0";
}

function responseDeclaresJSON(response: Response): boolean {
	const contentType = response.headers.get("content-type");
	if (!contentType) {
		return false;
	}

	const normalizedContentType = contentType.toLowerCase();
	return (
		normalizedContentType.includes("application/json") ||
		normalizedContentType.includes("+json")
	);
}

function responseDeclaresContentType(response: Response): boolean {
	return response.headers.has("content-type");
}

async function readSubmitSuccessResponseData(
	response: Response,
): Promise<unknown> {
	if (hasNoContentResponseBody(response)) {
		return undefined;
	}

	const maybeTextFn = (
		response as Response & {
			text?: () => Promise<string>;
		}
	).text;
	if (typeof maybeTextFn === "function") {
		const text = await maybeTextFn.call(response);
		if (text === "") {
			return undefined;
		}
		if (responseDeclaresJSON(response)) {
			return JSON.parse(text);
		}
		if (!responseDeclaresContentType(response)) {
			try {
				return JSON.parse(text);
			} catch {
				return text;
			}
		}
		return text;
	}

	const maybeJSONFn = (
		response as Response & { json?: () => Promise<unknown> }
	).json;
	if (
		typeof maybeJSONFn === "function" &&
		(responseDeclaresJSON(response) ||
			!responseDeclaresContentType(response))
	) {
		return maybeJSONFn.call(response);
	}

	return undefined;
}

function getSubmitRuntimeErrorResult<T>(props: {
	error: unknown;
	abortSignal: AbortSignal;
}): SubmitResult<T> {
	const { error, abortSignal } = props;
	if (isAbortError(error) || abortSignal.aborted) {
		return getAbortedSubmitResult<T>();
	}

	if (error instanceof Error) {
		logError(error);
		return getSubmitErrorResult<T>(error.message);
	}

	logError(error);
	return getUnknownSubmitErrorResult<T>();
}

export async function executeSubmitRuntime<T = unknown>(
	context: SubmitExecutionContext,
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<{ success: true; data: T } | { success: false; error: string }> {
	const submissionLifecycle = createSubmissionLifecycle(context, options);
	submissionLifecycle.begin();

	try {
		const preparedSubmitRequest = prepareSubmitRequest({
			url,
			requestInit,
			signal: submissionLifecycle.abortController.signal,
		});

		const { redirectData, response } = await executeSubmitRequest({
			abortController: submissionLifecycle.abortController,
			url: preparedSubmitRequest.url,
			requestInit: preparedSubmitRequest.requestInit,
		});
		const staleAfterRequest = getStaleSubmitResultFromCheckpointIfAny<T>({
			checkpoint: "post_request",
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});
		if (staleAfterRequest) {
			return staleAfterRequest;
		}

		syncBuildIDFromResponse(response);

		const staleBeforeFinalize = getStaleSubmitResultFromCheckpointIfAny<T>({
			checkpoint: "pre_finalize",
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});
		if (staleBeforeFinalize) {
			return staleBeforeFinalize;
		}

		const shouldReturnSubmitError = !response.ok;
		const shouldEffectuateRedirect = redirectData?.status === "should";
		const shouldAutoRevalidate =
			!shouldReturnSubmitError &&
			!shouldEffectuateRedirect &&
			shouldAutoRevalidateSubmitResult({
				requestInit,
				redirectData,
				options,
			});
		const staleAfterResponseClassification =
			getStaleSubmitResultFromCheckpointIfAny<T>({
				checkpoint: "post_response_classification",
				isSubmissionCurrent: submissionLifecycle.isCurrent,
			});
		if (staleAfterResponseClassification) {
			return staleAfterResponseClassification;
		}

		if (shouldReturnSubmitError) {
			return getSubmitErrorResult<T>(String(response.status));
		}

		if (shouldEffectuateRedirect) {
			const redirectResult = await effectuateRedirectDataResult(
				redirectData,
				0,
			);
			const staleAfterRedirectEffectuation =
				getStaleSubmitResultFromCheckpointIfAny<T>({
					checkpoint: "post_redirect_effectuation",
					isSubmissionCurrent: submissionLifecycle.isCurrent,
				});
			if (staleAfterRedirectEffectuation) {
				return staleAfterRedirectEffectuation;
			}
			if (!redirectResult || redirectResult.status !== "did") {
				return getSubmitRedirectFailureResult<T>();
			}
			return { success: true, data: undefined as T };
		}

		const data = await readSubmitSuccessResponseData(response);
		const staleBeforeReturn = getStaleSubmitResultFromCheckpointIfAny<T>({
			checkpoint: "pre_success_return",
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});
		if (staleBeforeReturn) {
			return staleBeforeReturn;
		}

		if (shouldAutoRevalidate) {
			await context.navigate({
				href: window.location.href,
				navigationType: "revalidation",
			});
			const staleAfterAutoRevalidate =
				getStaleSubmitResultFromCheckpointIfAny<T>({
					checkpoint: "post_auto_revalidate",
					isSubmissionCurrent: submissionLifecycle.isCurrent,
				});
			if (staleAfterAutoRevalidate) {
				return staleAfterAutoRevalidate;
			}
		}

		return { success: true, data: data as T };
	} catch (error) {
		return getSubmitRuntimeErrorResult<T>({
			error,
			abortSignal: submissionLifecycle.abortController.signal,
		});
	} finally {
		submissionLifecycle.finish();
	}
}

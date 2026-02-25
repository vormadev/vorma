import { getIsGETRequest, resolveAbsoluteHref } from "vorma/kit/url";
import { __vormaClientGlobal } from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import { assertProgrammaticSameOriginOrThrow } from "../../platform/url.ts";
import {
	effectuateRedirectDataResult,
	handleRedirects,
	type RedirectData,
} from "../redirects.ts";
import { syncBuildIDFromResponse } from "./runtime_navigation_successful_runtime.ts";
import type {
	NavigateProps,
	SubmissionEntry,
	SubmitOptions,
	SubmitResult,
} from "./types.ts";
import { hasSubmissionOperationOwnership } from "./types.ts";

type SubmissionLifecycle = {
	abortController: AbortController;
	isCurrent: () => boolean;
	begin: () => void;
	finish: () => void;
};

const submissionLifecycleReason = {
	dedupedByNewerSubmission: "submission_deduped_by_newer_submission",
	started: "submission_started",
	finished: "submission_finished",
} as const;

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

function emitSubmissionStateTransition(props: {
	context: SubmitExecutionContext;
	submissionEntry: SubmissionEntry;
	fromState: string;
	toState: string;
	reason: string;
	causedByOperationID?: number | null;
}): void {
	props.context.onSubmissionStateTransition?.({
		submissionEntry: props.submissionEntry,
		fromState: props.fromState,
		toState: props.toState,
		reason: props.reason,
		causedByOperationID: props.causedByOperationID ?? null,
	});
}

export function beginSubmissionLifecycle(props: {
	context: SubmitExecutionContext;
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	existingSubmissionEntry: SubmissionEntry | undefined;
}): void {
	const { context, submissionKey, submissionEntry, existingSubmissionEntry } =
		props;
	if (existingSubmissionEntry) {
		existingSubmissionEntry.control.abortController?.abort("deduped");
		emitSubmissionStateTransition({
			context,
			submissionEntry: existingSubmissionEntry,
			fromState: "submitting",
			toState: "aborted",
			reason: submissionLifecycleReason.dedupedByNewerSubmission,
			causedByOperationID: submissionEntry.operationID,
		});
	}

	context.submissions.set(submissionKey, submissionEntry);
	emitSubmissionStateTransition({
		context,
		submissionEntry,
		fromState: "none",
		toState: "submitting",
		reason: submissionLifecycleReason.started,
	});
	context.scheduleStatusUpdate();
}

export function finishSubmissionLifecycle(props: {
	context: SubmitExecutionContext;
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	shouldRemoveSubmissionEntry: boolean;
}): void {
	const {
		context,
		submissionKey,
		submissionEntry,
		shouldRemoveSubmissionEntry,
	} = props;
	if (shouldRemoveSubmissionEntry) {
		context.submissions.delete(submissionKey);
		emitSubmissionStateTransition({
			context,
			submissionEntry,
			fromState: "submitting",
			toState: "removed",
			reason: submissionLifecycleReason.finished,
		});
	}

	context.scheduleStatusUpdate();
}

function createSubmissionLifecycle(
	context: SubmitExecutionContext,
	options?: SubmitOptions,
): SubmissionLifecycle {
	const abortController = new AbortController();
	const submissionEntry: SubmissionEntry = {
		operationID: context.allocateSubmissionOperationID(),
		control: {
			abortController,
			promise: Promise.resolve() as Promise<unknown>,
		},
		startTime: Date.now(),
		skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
	};
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
		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry,
		});
	};

	const finish = (): void => {
		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: isCurrent(),
		});
	};

	return {
		abortController,
		isCurrent,
		begin,
		finish,
	};
}

function getAbortedSubmitResult<T>(): SubmitResult<T> {
	return { success: false, error: "Aborted" };
}

function getSubmitErrorResult<T>(error: string): SubmitResult<T> {
	return { success: false, error };
}

function getStaleSubmitResultIfNotCurrent<T>(props: {
	isSubmissionCurrent: () => boolean;
}): SubmitResult<T> | null {
	if (props.isSubmissionCurrent()) {
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
	return getSubmitErrorResult<T>("Unknown error");
}

export async function executeSubmitRuntime<T = unknown>(
	context: SubmitExecutionContext,
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	const submissionLifecycle = createSubmissionLifecycle(context, options);
	submissionLifecycle.begin();
	const getStaleSubmitResult = (): SubmitResult<T> | null =>
		getStaleSubmitResultIfNotCurrent<T>({
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});

	try {
		const submitRequestURL = new URL(resolveAbsoluteHref({ href: url }));
		assertProgrammaticSameOriginOrThrow({
			absoluteHref: submitRequestURL.href,
			apiName: "submit(...)",
		});
		const submitRequestHeaders = new Headers(requestInit?.headers);
		const deploymentID = __vormaClientGlobal.get("deploymentID");
		if (deploymentID) {
			submitRequestHeaders.set("x-deployment-id", deploymentID);
		}
		const submitRequestInit: RequestInit = {
			...requestInit,
			headers: submitRequestHeaders,
			signal: submissionLifecycle.abortController.signal,
		};

		const submitRequestResult = await handleRedirects({
			abortController: submissionLifecycle.abortController,
			url: submitRequestURL,
			redirectCount: 0,
			requestInit: submitRequestInit,
		});
		if (!submitRequestResult.response) {
			throw new Error("Submit request completed without a response.");
		}
		const { redirectData, response } = submitRequestResult;

		const staleAfterRequest = getStaleSubmitResult();
		if (staleAfterRequest) {
			return staleAfterRequest;
		}

		syncBuildIDFromResponse(response);

		const staleBeforeFinalize = getStaleSubmitResult();
		if (staleBeforeFinalize) {
			return staleBeforeFinalize;
		}

		const shouldEffectuateRedirect = redirectData?.status === "should";
		const shouldReturnSubmitError = !response.ok;
		const staleAfterResponseClassification = getStaleSubmitResult();
		if (staleAfterResponseClassification) {
			return staleAfterResponseClassification;
		}

		if (shouldEffectuateRedirect) {
			const redirectResult = await effectuateRedirectDataResult(
				redirectData,
				0,
			);
			const staleAfterRedirectEffectuation = getStaleSubmitResult();
			if (staleAfterRedirectEffectuation) {
				return staleAfterRedirectEffectuation;
			}
			if (!redirectResult || redirectResult.status !== "did") {
				return getSubmitErrorResult<T>("Redirect failed");
			}
			return { success: true, data: undefined as T };
		}

		if (shouldReturnSubmitError) {
			return getSubmitErrorResult<T>(String(response.status));
		}

		const shouldAutoRevalidate = shouldAutoRevalidateSubmitResult({
			requestInit,
			redirectData,
			options,
		});
		const data = await readSubmitSuccessResponseData(response);
		const staleBeforeReturn = getStaleSubmitResult();
		if (staleBeforeReturn) {
			return staleBeforeReturn;
		}

		if (shouldAutoRevalidate) {
			await context.navigate({
				href: window.location.href,
				navigationType: "revalidation",
			});
			const staleAfterAutoRevalidate = getStaleSubmitResult();
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

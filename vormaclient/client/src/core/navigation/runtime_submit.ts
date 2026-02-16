import { getIsGETRequest, resolveAbsoluteHref } from "vorma/kit/url";
import { __vormaClientGlobal } from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import {
	effectuateRedirectDataResult,
	handleRedirects,
	type RedirectData,
} from "../redirects.ts";
import type { NavigateProps, SubmitOptions, SubmissionEntry } from "./types.ts";
import { syncBuildIDFromResponse } from "./runtime_navigation_outcome.ts";

type SubmissionLifecycle = {
	abortController: AbortController;
	isCurrent: () => boolean;
	begin: () => void;
	finish: () => void;
};

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
		context.submissions.get(submissionKey) === submissionEntry;

	const begin = (): void => {
		if (typeof submissionKey === "string") {
			const existing = context.submissions.get(submissionKey);
			if (existing) {
				existing.control.abortController?.abort("deduped");
				context.onSubmissionStateTransition?.({
					submissionEntry: existing,
					fromState: "submitting",
					toState: "aborted",
					reason: "submission_deduped_by_newer_submission",
					causedByOperationID: submissionEntry.operationID,
				});
			}
		}

		context.submissions.set(submissionKey, submissionEntry);
		context.onSubmissionStateTransition?.({
			submissionEntry,
			fromState: "none",
			toState: "submitting",
			reason: "submission_started",
		});
		context.scheduleStatusUpdate();
	};

	const finish = (): void => {
		if (isCurrent()) {
			context.submissions.delete(submissionKey);
			context.onSubmissionStateTransition?.({
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: "submission_finished",
			});
		}

		context.scheduleStatusUpdate();
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
		isPrefetch: false,
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

function getStaleSubmitResultIfAny<T>(
	isSubmissionCurrent: () => boolean,
): SubmitResult<T> | null {
	if (isSubmissionCurrent()) {
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
		return true;
	}

	const normalizedContentType = contentType.toLowerCase();
	return (
		normalizedContentType.includes("application/json") ||
		normalizedContentType.includes("+json")
	);
}

async function readSubmitSuccessResponseData(
	response: Response,
): Promise<unknown> {
	if (hasNoContentResponseBody(response)) {
		return undefined;
	}

	if (responseDeclaresJSON(response)) {
		return await response.json();
	}

	const maybeTextFn = (
		response as Response & { text?: () => Promise<string> }
	).text;
	if (typeof maybeTextFn !== "function") {
		return undefined;
	}

	const text = await maybeTextFn.call(response);
	return text === "" ? undefined : text;
}

type SubmitResponseAction =
	| {
			type: "error";
			error: string;
	  }
	| {
			type: "redirectShould";
			redirectData: RedirectData;
	  }
	| {
			type: "parseJSON";
			shouldAutoRevalidate: boolean;
	  };

function decideSubmitResponseAction(props: {
	response: Response;
	redirectData: RedirectData | null;
	requestInit?: RequestInit;
	options?: SubmitOptions;
}): SubmitResponseAction {
	const { response, redirectData, requestInit, options } = props;

	if (!response.ok) {
		return {
			type: "error",
			error: String(response.status),
		};
	}

	if (redirectData?.status === "should") {
		return {
			type: "redirectShould",
			redirectData,
		};
	}

	return {
		type: "parseJSON",
		shouldAutoRevalidate: shouldAutoRevalidateSubmitResult({
			requestInit,
			redirectData,
			options,
		}),
	};
}

async function executeSubmitResponseAction<T>(props: {
	action: SubmitResponseAction;
	response: Response;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	isSubmissionCurrent: () => boolean;
}): Promise<SubmitResult<T>> {
	const { action, response, navigate, isSubmissionCurrent } = props;

	switch (action.type) {
		case "error":
			return getSubmitErrorResult<T>(action.error);
		case "redirectShould": {
			const redirectResult = await effectuateRedirectDataResult(
				action.redirectData,
				0,
			);
			const staleAfterRedirectEffectuation =
				getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
			if (staleAfterRedirectEffectuation) {
				return staleAfterRedirectEffectuation;
			}
			if (!redirectResult || redirectResult.status !== "did") {
				return getSubmitRedirectFailureResult<T>();
			}
			return { success: true, data: undefined as T };
		}
		case "parseJSON": {
			const data = await readSubmitSuccessResponseData(response);
			const staleBeforeReturn =
				getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
			if (staleBeforeReturn) {
				return staleBeforeReturn;
			}

			if (action.shouldAutoRevalidate) {
				await navigate({
					href: window.location.href,
					navigationType: "revalidation",
				});
				const staleAfterAutoRevalidate =
					getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
				if (staleAfterAutoRevalidate) {
					return staleAfterAutoRevalidate;
				}
			}

			return { success: true, data: data as T };
		}
	}
}

async function finalizeSubmitResponse<T>(props: {
	response: Response;
	redirectData: RedirectData | null;
	requestInit?: RequestInit;
	options?: SubmitOptions;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	isSubmissionCurrent: () => boolean;
}): Promise<SubmitResult<T>> {
	const {
		response,
		redirectData,
		requestInit,
		options,
		navigate,
		isSubmissionCurrent,
	} = props;

	const staleBeforeResponse =
		getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
	if (staleBeforeResponse) {
		return staleBeforeResponse;
	}

	const responseAction = decideSubmitResponseAction({
		response,
		redirectData,
		requestInit,
		options,
	});
	const staleAfterResponseClassification =
		getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
	if (staleAfterResponseClassification) {
		return staleAfterResponseClassification;
	}

	return await executeSubmitResponseAction({
		action: responseAction,
		response,
		navigate,
		isSubmissionCurrent,
	});
}

type SubmitPostRequestAction<T> =
	| {
			type: "stop";
			result: SubmitResult<T>;
	  }
	| {
			type: "finalize";
			response: Response;
			redirectData: RedirectData | null;
	  };

function decideSubmitPostRequestAction<T>(props: {
	response: Response;
	redirectData: RedirectData | null;
	isSubmissionCurrent: () => boolean;
}): SubmitPostRequestAction<T> {
	const { response, redirectData, isSubmissionCurrent } = props;
	const staleAfterRequest = getStaleSubmitResultIfAny<T>(isSubmissionCurrent);
	if (staleAfterRequest) {
		return {
			type: "stop",
			result: staleAfterRequest,
		};
	}

	return {
		type: "finalize",
		response,
		redirectData,
	};
}

async function executeSubmitPostRequestAction<T>(props: {
	action: SubmitPostRequestAction<T>;
	requestInit?: RequestInit;
	options?: SubmitOptions;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	isSubmissionCurrent: () => boolean;
}): Promise<SubmitResult<T>> {
	const { action, requestInit, options, navigate, isSubmissionCurrent } =
		props;

	switch (action.type) {
		case "stop":
			return action.result;
		case "finalize":
			syncBuildIDFromResponse(action.response);
			return await finalizeSubmitResponse<T>({
				response: action.response,
				redirectData: action.redirectData,
				requestInit,
				options,
				navigate,
				isSubmissionCurrent,
			});
	}
}

type SubmitRuntimeErrorAction =
	| {
			type: "aborted";
	  }
	| {
			type: "known";
			error: Error;
	  }
	| {
			type: "unknown";
			error: unknown;
	  };

function decideSubmitRuntimeErrorAction(props: {
	error: unknown;
	abortSignal: AbortSignal;
}): SubmitRuntimeErrorAction {
	const { error, abortSignal } = props;
	if (isAbortError(error) || abortSignal.aborted) {
		return { type: "aborted" };
	}
	if (error instanceof Error) {
		return {
			type: "known",
			error,
		};
	}
	return {
		type: "unknown",
		error,
	};
}

function executeSubmitRuntimeErrorAction<T>(
	action: SubmitRuntimeErrorAction,
): SubmitResult<T> {
	switch (action.type) {
		case "aborted":
			return getAbortedSubmitResult<T>();
		case "known":
			logError(action.error);
			return getSubmitErrorResult<T>(action.error.message);
		case "unknown":
			logError(action.error);
			return getUnknownSubmitErrorResult<T>();
	}
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

		const postRequestAction = decideSubmitPostRequestAction<T>({
			response,
			redirectData,
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});

		return await executeSubmitPostRequestAction<T>({
			action: postRequestAction,
			requestInit,
			options,
			navigate: context.navigate,
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});
	} catch (error) {
		const errorAction = decideSubmitRuntimeErrorAction({
			error,
			abortSignal: submissionLifecycle.abortController.signal,
		});
		return executeSubmitRuntimeErrorAction<T>(errorAction);
	} finally {
		submissionLifecycle.finish();
	}
}

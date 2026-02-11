import { isAbortError } from "../utils/errors.ts";
import { logError } from "../utils/logging.ts";
import {
	beginSubmissionLifecycle,
	createActiveSubmission,
	finishSubmissionLifecycle,
} from "./submit_lifecycle.ts";
import { syncBuildIDFromResponse } from "./successful_navigation_effects.ts";
import {
	buildSubmitRequestInit,
	executeSubmitRequest,
} from "./submit_request.ts";
import { finalizeSubmitResponse } from "./submit_response.ts";
import type { NavigateProps, SubmitOptions, SubmissionEntry } from "./types.ts";

export type SubmitExecutionContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	navigate: (props: NavigateProps) => Promise<{
		didNavigate: boolean;
	}>;
};

export async function executeSubmit<T = any>(
	context: SubmitExecutionContext,
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<{ success: true; data: T } | { success: false; error: string }> {
	const activeSubmission = createActiveSubmission(options);
	beginSubmissionLifecycle(context, activeSubmission);

	try {
		const urlToUse = new URL(url, window.location.href);
		const finalRequestInit = buildSubmitRequestInit({
			requestInit,
			signal: activeSubmission.abortController.signal,
		});

		const { redirectData, response } = await executeSubmitRequest({
			abortController: activeSubmission.abortController,
			url: urlToUse,
			requestInit: finalRequestInit,
		});

		if (response) {
			syncBuildIDFromResponse(response);
		}

		return await finalizeSubmitResponse<T>({
			response,
			redirectData,
			requestInit,
			options,
			navigate: context.navigate,
		});
	} catch (error) {
		if (isAbortError(error)) {
			return { success: false, error: "Aborted" };
		}
		logError(error);
		return {
			success: false,
			error: error instanceof Error ? error.message : "Unknown error",
		};
	} finally {
		finishSubmissionLifecycle(context, activeSubmission);
	}
}

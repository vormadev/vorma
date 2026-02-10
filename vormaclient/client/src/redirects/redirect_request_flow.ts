import { logError } from "../utils/logging.ts";
import { buildRedirectRequestInit } from "./redirect_request_init.ts";

export type RedirectRequestFlowInput = {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	redirectCount?: number;
};

export type RedirectRequestFlowResult =
	| {
			kind: "too_many_redirects";
	  }
	| {
			kind: "ok";
			response: Response;
			requestInit: RequestInit;
	  };

export async function executeRedirectRequestFlow(
	input: RedirectRequestFlowInput,
): Promise<RedirectRequestFlowResult> {
	const MAX_REDIRECTS = 10;
	const redirectCount = input.redirectCount || 0;

	if (redirectCount >= MAX_REDIRECTS) {
		logError("Too many redirects");
		return { kind: "too_many_redirects" };
	}

	const requestInit = buildRedirectRequestInit(
		input.requestInit,
		input.abortController.signal,
	);
	const response = await fetch(input.url, requestInit);

	return {
		kind: "ok",
		response,
		requestInit,
	};
}

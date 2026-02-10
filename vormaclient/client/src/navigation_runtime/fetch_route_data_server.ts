import { handleRedirects, type RedirectData } from "../redirects/redirects.ts";
import type { GetRouteDataOutput } from "../vorma_ctx/vorma_ctx.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export type ServerRouteDataResult = {
	redirectData: RedirectData | null;
	response?: Response;
	json?: GetRouteDataOutput;
};

export async function createServerRouteDataPromise(props: {
	abortController: AbortController;
	url: URL;
	isPrefetch: boolean;
	redirectCount?: number;
}): Promise<ServerRouteDataResult> {
	const result = await handleRedirects(props);

	if (result.response && result.response.ok && !result.redirectData?.status) {
		const json = await result.response.json();
		return { ...result, json };
	}

	return { ...result, json: undefined };
}

type ResolvedServerRouteDataResult =
	| {
			type: "outcome";
			outcome:
				| Extract<NavigationOutcome, { type: "aborted" }>
				| Extract<NavigationOutcome, { type: "redirect" }>;
	  }
	| {
			type: "success";
			response: Response;
			json: GetRouteDataOutput;
	  };

export function resolveServerRouteDataResult(props: {
	controller: AbortController;
	navigationProps: NavigateProps;
	serverResult: ServerRouteDataResult;
}): ResolvedServerRouteDataResult {
	const { controller, navigationProps, serverResult } = props;
	const { redirectData, response, json } = serverResult;

	const redirected = redirectData?.status === "did";
	const responseNotOK = !response?.ok && response?.status !== 304;

	if (redirected || !response) {
		controller.abort();
		return { type: "outcome", outcome: { type: "aborted" } };
	}

	if (responseNotOK) {
		controller.abort();
		throw new Error(`Fetch failed with status ${response.status}`);
	}

	if (redirectData?.status === "should") {
		controller.abort();
		return {
			type: "outcome",
			outcome: { type: "redirect", redirectData, props: navigationProps },
		};
	}

	if (!json) {
		controller.abort();
		throw new Error("No JSON response");
	}

	return { type: "success", response, json };
}

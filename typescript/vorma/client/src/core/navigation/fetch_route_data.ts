import { isAbortError, logError } from "../../platform/safety.ts";
import { getClientOnlyOutcomeIfSkippable } from "./fetch_route_data_skip.ts";
import {
	buildRouteDataRequestURL,
	buildServerSuccessOutcome,
	createServerRouteDataPromise,
	resolveServerRouteDataResult,
	startParallelClientLoaders,
} from "./fetch_route_data_server.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export async function fetchRouteData(
	controller: AbortController,
	props: NavigateProps,
): Promise<NavigationOutcome> {
	try {
		const targetURL = new URL(props.href, window.location.href);
		const clientOnlyOutcome = getClientOnlyOutcomeIfSkippable({
			navigationProps: props,
			controller,
			targetHref: targetURL.href,
		});
		if (clientOnlyOutcome) {
			return clientOnlyOutcome;
		}

		const requestURL = buildRouteDataRequestURL({
			targetHref: targetURL.href,
			navigationType: props.navigationType,
		});

		const serverPromise = createServerRouteDataPromise({
			abortController: controller,
			url: requestURL,
			isPrefetch: props.navigationType === "prefetch",
			redirectCount: props.redirectCount,
		});

		const runningLoaders = await startParallelClientLoaders({
			pathname: requestURL.pathname,
			serverPromise,
			signal: controller.signal,
		});

		const resolvedServerResult = resolveServerRouteDataResult({
			controller,
			navigationProps: props,
			serverResult: await serverPromise,
		});
		if (resolvedServerResult.type === "outcome") {
			return resolvedServerResult.outcome;
		}
		const { response, json } = resolvedServerResult;

		return buildServerSuccessOutcome({
			response,
			json,
			navigationProps: props,
			runningLoaders,
			signal: controller.signal,
		});
	} catch (error) {
		if (!isAbortError(error)) {
			logError("Navigation failed", error);
		}
		throw error;
	}
}

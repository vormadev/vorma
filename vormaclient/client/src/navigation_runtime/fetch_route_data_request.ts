import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import { buildClientOnlyOutcome } from "./client_only_outcome.ts";
import { canSkipServerFetch } from "./skip_server_fetch.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export function getClientOnlyOutcomeIfSkippable(props: {
	navigationProps: NavigateProps;
	controller: AbortController;
	targetHref: string;
}): NavigationOutcome | undefined {
	const { navigationProps, controller, targetHref } = props;

	// Check if we can skip the server fetch (not for revalidations/actions).
	if (
		navigationProps.navigationType === "revalidation" ||
		navigationProps.navigationType === "action"
	) {
		return undefined;
	}

	const skipCheck = canSkipServerFetch(targetHref);
	if (!skipCheck.canSkip) {
		return undefined;
	}

	return buildClientOnlyOutcome(skipCheck, navigationProps, controller);
}

export function buildRouteDataRequestURL(props: {
	targetHref: string;
	navigationType: NavigateProps["navigationType"];
}): URL {
	const url = new URL(props.targetHref);
	url.searchParams.set(
		"vorma_json",
		__vormaClientGlobal.get("buildID") || "1",
	);

	if (props.navigationType === "revalidation") {
		const deploymentID = __vormaClientGlobal.get("deploymentID");
		if (deploymentID) {
			url.searchParams.set("dpl", deploymentID);
		}
	}

	return url;
}

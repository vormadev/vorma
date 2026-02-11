import { completeClientLoaders } from "../client_loaders.ts";
import { observePromiseRejection } from "../utils/promise_safety.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../vorma_ctx/vorma_ctx.ts";
import type { SkipCheckResult } from "./skip_server_fetch.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export function buildClientOnlyOutcome(
	skipCheck: Extract<SkipCheckResult, { canSkip: true }>,
	props: NavigateProps,
	controller: AbortController,
): NavigationOutcome {
	const { matchResult, importURLs, exportKeys, loadersData } = skipCheck;

	const json: GetRouteDataOutput = {
		matchedPatterns: matchResult.matches.map(
			(match: { registeredPattern: { originalPattern: string } }) =>
				match.registeredPattern.originalPattern,
		),
		loadersData,
		importURLs,
		exportKeys,
		hasRootData: __vormaClientGlobal.get("hasRootData"),
		params: matchResult.params,
		splatValues: matchResult.splatValues,
		deps: [],
		cssBundles: [],
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		errorExportKeys: [],
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		activeComponents: undefined as unknown as [],
	};

	const response = new Response(JSON.stringify(json), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Vorma-Build-Id": __vormaClientGlobal.get("buildID") || "1",
		},
	});

	const currentClientLoadersData =
		__vormaClientGlobal.get("clientLoadersData") || [];
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const runningLoaders = new Map<string, Promise<any>>();

	for (let i = 0; i < json.matchedPatterns.length; i++) {
		const pattern = json.matchedPatterns[i];
		if (!pattern) continue;

		if (patternToWaitFnMap[pattern]) {
			const currentMatchedPatterns =
				__vormaClientGlobal.get("matchedPatterns") || [];
			const currentPatternIndex = currentMatchedPatterns.indexOf(pattern);

			if (
				currentPatternIndex !== -1 &&
				currentClientLoadersData[currentPatternIndex] !== undefined
			) {
				runningLoaders.set(
					pattern,
					Promise.resolve(
						currentClientLoadersData[currentPatternIndex],
					),
				);
			}
		}
	}

	const waitFnPromise = completeClientLoaders(
		json,
		__vormaClientGlobal.get("buildID") || "1",
		runningLoaders,
		controller.signal,
	);
	observePromiseRejection(waitFnPromise);

	return {
		type: "success",
		response,
		props,
		json,
		cssBundlePromises: [],
		waitFnPromise,
	};
}

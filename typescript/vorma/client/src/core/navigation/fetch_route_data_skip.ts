import { completeClientLoaders } from "../render_runtime.ts";
import { __vormaClientGlobal } from "../../app/context.ts";
import type { GetRouteDataOutput } from "../../app/context.ts";
import type { VormaClientGlobal } from "../../app/context.ts";
import { observePromiseRejection } from "../../platform/safety.ts";
import {
	buildSkipCheckContext,
	getMatchedPatternsOrThrow,
	isSkipEligibilityViolated,
	type SkipCheckContext,
	type SkipMatchResult,
} from "./fetch_route_data_skip_match.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export type SkipCheckResult =
	| { canSkip: false }
	| {
			canSkip: true;
			matchResult: SkipMatchResult;
			importURLs: string[];
			exportKeys: string[];
			errorExportKeys: string[];
			loadersData: unknown[];
	  };

type ClientOnlyOutcomeGlobalSnapshot = {
	buildID: string;
	hasRootData: boolean;
	currentMatchedPatterns: string[];
	currentClientLoadersData: unknown[];
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
};

function getClientOnlyOutcomeGlobalSnapshot(): ClientOnlyOutcomeGlobalSnapshot {
	return {
		buildID: __vormaClientGlobal.get("buildID") || "1",
		hasRootData: __vormaClientGlobal.get("hasRootData"),
		currentMatchedPatterns:
			__vormaClientGlobal.get("matchedPatterns") || [],
		currentClientLoadersData:
			__vormaClientGlobal.get("clientLoadersData") || [],
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap") || {},
	};
}

function buildSkipResultItem(props: {
	ctx: SkipCheckContext;
	pattern: string;
}): {
	importURL: string;
	exportKey: string;
	errorExportKey: string;
	loaderData: unknown;
} | null {
	const { ctx, pattern } = props;
	const moduleInfo = ctx.clientModuleMap[pattern];
	if (!moduleInfo) {
		return null;
	}

	const hasServerLoader = ctx.routeManifest[pattern] === 1;
	if (!hasServerLoader) {
		return {
			importURL: moduleInfo.importURL,
			exportKey: moduleInfo.exportKey,
			errorExportKey: moduleInfo.errorExportKey || "",
			loaderData: undefined,
		};
	}

	const currentPatternIndex = ctx.currentMatchedPatterns.indexOf(pattern);
	if (currentPatternIndex === -1) {
		return null;
	}
	const loaderData = ctx.currentLoadersData[currentPatternIndex];
	if (loaderData === undefined) {
		return null;
	}

	return {
		importURL: moduleInfo.importURL,
		exportKey: moduleInfo.exportKey,
		errorExportKey: moduleInfo.errorExportKey || "",
		loaderData,
	};
}

function buildSkipResultFromContext(ctx: SkipCheckContext): SkipCheckResult {
	const importURLs: string[] = [];
	const exportKeys: string[] = [];
	const errorExportKeys: string[] = [];
	const loadersData: unknown[] = [];
	const matchedPatterns = getMatchedPatternsOrThrow({
		matches: ctx.matchResult.matches,
		context: "Route matcher",
	});

	for (const pattern of matchedPatterns) {
		const item = buildSkipResultItem({ ctx, pattern });
		if (!item) {
			return { canSkip: false };
		}

		importURLs.push(item.importURL);
		exportKeys.push(item.exportKey);
		errorExportKeys.push(item.errorExportKey);
		loadersData.push(item.loaderData);
	}

	return {
		canSkip: true,
		matchResult: ctx.matchResult,
		importURLs,
		exportKeys,
		errorExportKeys,
		loadersData,
	};
}

export function canSkipServerFetch(targetUrl: string): SkipCheckResult {
	const ctx = buildSkipCheckContext(targetUrl);
	if (!ctx) {
		return { canSkip: false };
	}

	if (isSkipEligibilityViolated(ctx)) {
		return { canSkip: false };
	}

	return buildSkipResultFromContext(ctx);
}

function buildClientOnlyOutcome(
	skipCheck: Extract<SkipCheckResult, { canSkip: true }>,
	props: NavigateProps,
	controller: AbortController,
): NavigationOutcome {
	const {
		matchResult,
		importURLs,
		exportKeys,
		errorExportKeys,
		loadersData,
	} = skipCheck;
	const matchedPatterns = getMatchedPatternsOrThrow({
		matches: matchResult.matches,
		context: "Route matcher",
	});

	const globalSnapshot = getClientOnlyOutcomeGlobalSnapshot();
	const {
		buildID,
		hasRootData,
		currentMatchedPatterns,
		currentClientLoadersData,
		patternToWaitFnMap,
	} = globalSnapshot;

	const json: GetRouteDataOutput = {
		matchedPatterns,
		loadersData,
		importURLs,
		exportKeys,
		hasRootData,
		params: matchResult.params,
		splatValues: matchResult.splatValues,
		deps: [],
		cssBundles: [],
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		errorExportKeys,
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
	};

	const response = new Response(JSON.stringify(json), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Vorma-Build-Id": buildID,
		},
	});
	const runningLoaders = new Map<string, Promise<unknown>>();

	for (const pattern of json.matchedPatterns) {
		if (patternToWaitFnMap[pattern]) {
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
		buildID,
		runningLoaders,
		controller.signal,
	);
	observePromiseRejection(waitFnPromise);

	return {
		type: "success",
		response,
		props,
		json,
		preloadPlan: {
			moduleDependencies: [],
			cssBundles: [],
		},
		waitFnPromise,
	};
}

export function getClientOnlyOutcomeIfSkippable(props: {
	navigationProps: NavigateProps;
	controller: AbortController;
	targetHref: string;
}): NavigationOutcome | undefined {
	const { navigationProps, controller, targetHref } = props;

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

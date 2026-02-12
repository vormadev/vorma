import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import type { PatternRegistry } from "vorma/kit/matcher/register";
import {
	completeClientLoaders,
	buildClientLoaderServerData,
	createUnavailableServerDataError,
	findPartialMatchesOnClient,
} from "../render_runtime.ts";
import { AssetManager } from "../render_runtime.ts";
import {
	getBuildIDFromResponse,
	handleRedirects,
	type RedirectData,
} from "../redirects.ts";
import { __vormaClientGlobal } from "../../app/context.ts";
import type {
	ClientLoaderAwaitedServerData,
	GetRouteDataOutput,
	VormaClientGlobal,
} from "../../app/context.ts";
import {
	isAbortError,
	logError,
	observePromiseRejection,
} from "../../platform/safety.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

type SkipMatch = {
	registeredPattern: {
		originalPattern: string;
		normalizedSegments: Array<{
			segType: string;
			normalizedVal: string;
		}>;
		lastSegType: string;
	};
};

type SkipMatchResult = {
	params: Record<string, string>;
	splatValues: string[];
	matches: SkipMatch[];
};

export type SkipCheckContext = {
	routeManifest: Record<string, number>;
	patternRegistry: PatternRegistry;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	clientModuleMap: VormaClientGlobal["clientModuleMap"];
	currentMatchedPatterns: string[];
	currentParams: Record<string, string>;
	currentSplatValues: string[];
	currentLoadersData: unknown[];
	url: URL;
	matchResult: SkipMatchResult;
};

export type SkipCheckResult =
	| { canSkip: false }
	| {
			canSkip: true;
			matchResult: SkipMatchResult;
			importURLs: string[];
			exportKeys: string[];
			loadersData: unknown[];
	  };

function getMatchedPatternOrThrow(
	match: SkipMatch | undefined,
	index: number,
	context: string,
): string {
	if (!match) {
		throw new Error(
			`${context} returned a sparse matches array at index ${index}.`,
		);
	}

	const pattern = match.registeredPattern.originalPattern;
	if (!pattern) {
		throw new Error(
			`${context} returned an empty route pattern at index ${index}.`,
		);
	}

	return pattern;
}

function buildSkipCheckContext(
	targetUrl: string,
): SkipCheckContext | undefined {
	const routeManifest = __vormaClientGlobal.get("routeManifest");
	if (!routeManifest) {
		return undefined;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		return undefined;
	}

	const url = new URL(targetUrl);
	const nestedMatchResult = findNestedMatches(patternRegistry, url.pathname);
	if (!nestedMatchResult) {
		return undefined;
	}
	const matchResult: SkipMatchResult = nestedMatchResult;

	return {
		routeManifest,
		patternRegistry,
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap") || {},
		clientModuleMap: __vormaClientGlobal.get("clientModuleMap") || {},
		currentMatchedPatterns:
			__vormaClientGlobal.get("matchedPatterns") || [],
		currentParams: __vormaClientGlobal.get("params") || {},
		currentSplatValues: __vormaClientGlobal.get("splatValues") || [],
		currentLoadersData: __vormaClientGlobal.get("loadersData") || [],
		url,
		matchResult,
	};
}

function hasServerLoaderRemoval(ctx: SkipCheckContext): boolean {
	for (const pattern of ctx.currentMatchedPatterns) {
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (hasServerLoader) {
			const stillMatched = ctx.matchResult.matches.some(
				(m) => m.registeredPattern.originalPattern === pattern,
			);
			if (!stillMatched) {
				return true;
			}
		}
	}
	return false;
}

function hasNewClientLoader(ctx: SkipCheckContext): boolean {
	for (let i = 0; i < ctx.matchResult.matches.length; i++) {
		const pattern = getMatchedPatternOrThrow(
			ctx.matchResult.matches[i],
			i,
			"Route matcher",
		);
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];
		const wasAlreadyMatched = ctx.currentMatchedPatterns.includes(pattern);
		if (hasClientLoader && !wasAlreadyMatched) {
			return true;
		}
	}
	return false;
}

function findOutermostLoaderIndex(ctx: SkipCheckContext): number {
	for (let i = ctx.matchResult.matches.length - 1; i >= 0; i--) {
		const pattern = getMatchedPatternOrThrow(
			ctx.matchResult.matches[i],
			i,
			"Route matcher",
		);
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];

		if (hasServerLoader || hasClientLoader) {
			return i;
		}
	}
	return -1;
}

function didSearchParamsChange(ctx: SkipCheckContext): boolean {
	const currentUrlObj = new URL(window.location.href);
	return currentUrlObj.search !== ctx.url.search;
}

function didOutermostParamsChange(
	ctx: SkipCheckContext,
	outermostLoaderIndex: number,
): boolean {
	const outermostMatch = ctx.matchResult.matches[outermostLoaderIndex]!;

	for (const seg of outermostMatch.registeredPattern.normalizedSegments) {
		if (seg.segType === "dynamic") {
			const paramName = seg.normalizedVal.substring(1);
			if (
				ctx.matchResult.params[paramName] !==
				ctx.currentParams[paramName]
			) {
				return true;
			}
		}
	}

	const hasSplat = outermostMatch.registeredPattern.lastSegType === "splat";
	if (hasSplat) {
		if (
			!jsonDeepEquals(ctx.matchResult.splatValues, ctx.currentSplatValues)
		) {
			return true;
		}
	}

	return false;
}

export function isSkipEligibilityViolated(ctx: SkipCheckContext): boolean {
	if (hasServerLoaderRemoval(ctx)) {
		return true;
	}

	if (hasNewClientLoader(ctx)) {
		return true;
	}

	const outermostLoaderIndex = findOutermostLoaderIndex(ctx);

	if (outermostLoaderIndex !== -1 && didSearchParamsChange(ctx)) {
		return true;
	}

	if (
		outermostLoaderIndex !== -1 &&
		didOutermostParamsChange(ctx, outermostLoaderIndex)
	) {
		return true;
	}

	return false;
}

function buildSkipResultItem(props: {
	ctx: SkipCheckContext;
	pattern: string;
}): { importURL: string; exportKey: string; loaderData: unknown } | null {
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
		loaderData,
	};
}

function buildSkipResultFromContext(ctx: SkipCheckContext): SkipCheckResult {
	const importURLs: string[] = [];
	const exportKeys: string[] = [];
	const loadersData: unknown[] = [];

	for (let i = 0; i < ctx.matchResult.matches.length; i++) {
		const pattern = getMatchedPatternOrThrow(
			ctx.matchResult.matches[i],
			i,
			"Route matcher",
		);
		const item = buildSkipResultItem({ ctx, pattern });
		if (!item) {
			return { canSkip: false };
		}

		importURLs.push(item.importURL);
		exportKeys.push(item.exportKey);
		loadersData.push(item.loaderData);
	}

	return {
		canSkip: true,
		matchResult: ctx.matchResult,
		importURLs,
		exportKeys,
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
	const { matchResult, importURLs, exportKeys, loadersData } = skipCheck;
	const matchedPatterns: string[] = [];
	for (let i = 0; i < matchResult.matches.length; i++) {
		matchedPatterns.push(
			getMatchedPatternOrThrow(
				matchResult.matches[i],
				i,
				"Route matcher",
			),
		);
	}

	const buildID = __vormaClientGlobal.get("buildID") || "1";
	const currentMatchedPatterns =
		__vormaClientGlobal.get("matchedPatterns") || [];
	const currentClientLoadersData =
		__vormaClientGlobal.get("clientLoadersData") || [];
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};

	const json: GetRouteDataOutput = {
		matchedPatterns,
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
		cssBundlePromises: [],
		waitFnPromise,
	};
}

function getClientOnlyOutcomeIfSkippable(props: {
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

function buildRouteDataRequestURL(props: {
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

type ServerRouteDataResult = {
	redirectData: RedirectData | null;
	response?: Response;
	json?: GetRouteDataOutput;
};

async function createServerRouteDataPromise(props: {
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

function resolveServerRouteDataResult(props: {
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

function buildServerDataForPattern(
	pattern: string,
	serverResult: ServerRouteDataResult,
): ClientLoaderAwaitedServerData<unknown, unknown> | null {
	const { response, json } = serverResult;
	if (!response || !response.ok || !json) {
		return null;
	}

	const matchedPatterns = json.matchedPatterns || [];
	const loadersData = json.loadersData || [];
	if (!matchedPatterns.includes(pattern)) {
		return null;
	}

	const buildID = getBuildIDFromResponse(response) || "1";

	return buildClientLoaderServerData({
		pattern,
		matchedPatterns,
		loadersData,
		hasRootData: !!json.hasRootData,
		buildID,
	});
}

async function startParallelClientLoaders(props: {
	pathname: string;
	serverPromise: Promise<ServerRouteDataResult>;
	signal: AbortSignal;
}): Promise<Map<string, Promise<unknown>>> {
	const matchResult = await findPartialMatchesOnClient(props.pathname);
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const runningLoaders = new Map<string, Promise<unknown>>();

	if (!matchResult) {
		return runningLoaders;
	}

	const { params, splatValues, matches } = matchResult;

	for (let i = 0; i < matches.length; i++) {
		const pattern = getMatchedPatternOrThrow(
			matches[i] as SkipMatch | undefined,
			i,
			"Partial route matcher",
		);
		const loaderFn = patternToWaitFnMap[pattern];

		if (!loaderFn) {
			continue;
		}

		const serverDataPromise = props.serverPromise.then(
			(serverResult) => {
				const serverData = buildServerDataForPattern(
					pattern,
					serverResult,
				);
				if (!serverData) {
					throw createUnavailableServerDataError();
				}
				return serverData;
			},
			() => {
				throw createUnavailableServerDataError();
			},
		);

		const loaderPromise = loaderFn({
			params,
			splatValues,
			serverDataPromise,
			signal: props.signal,
		});
		observePromiseRejection(loaderPromise);

		runningLoaders.set(pattern, loaderPromise);
	}

	return runningLoaders;
}

function buildServerSuccessOutcome(props: {
	response: Response;
	json: GetRouteDataOutput;
	navigationProps: NavigateProps;
	runningLoaders: Map<string, Promise<unknown>>;
	signal: AbortSignal;
}): Extract<NavigationOutcome, { type: "success" }> {
	const { response, json, navigationProps, runningLoaders, signal } = props;

	const depsToPreload = import.meta.env.DEV
		? [...new Set(json.importURLs)]
		: json.deps;
	for (const dep of depsToPreload ?? []) {
		if (dep) AssetManager.preloadModule(dep);
	}

	const buildID = getBuildIDFromResponse(response);

	const waitFnPromise = completeClientLoaders(
		json,
		buildID,
		runningLoaders,
		signal,
	);
	observePromiseRejection(waitFnPromise);

	const cssBundlePromises: Array<Promise<unknown>> = [];
	for (const bundle of json.cssBundles ?? []) {
		cssBundlePromises.push(AssetManager.preloadCSS(bundle));
	}

	return {
		type: "success",
		response,
		json,
		props: navigationProps,
		cssBundlePromises,
		waitFnPromise,
	};
}

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

import type {
	ClientLoaderAwaitedServerData,
	GetRouteDataOutput,
	VormaClientGlobal,
} from "../../app/context.ts";
import { __vormaClientGlobal } from "../../app/context.ts";
import {
	isAbortError,
	logError,
	observePromiseRejection,
} from "../../platform/safety.ts";
import {
	getBuildIDFromResponse,
	handleRedirects,
	type RedirectData,
} from "../redirects.ts";
import {
	buildClientLoaderServerData,
	completeClientLoaders,
	createUnavailableServerDataError,
	findPartialMatchesOnClient,
} from "../render_runtime.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export type ServerSuccessPreloadPlan = {
	moduleDependencies: string[];
	cssBundles: string[];
};

type RouteMatch = {
	registeredPattern: {
		originalPattern: string;
	};
};

function getMatchedPatternOrThrow(props: {
	match: RouteMatch | undefined;
	index: number;
	context: string;
}): string {
	const { match, index, context } = props;
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

function getMatchedPatternsOrThrow(props: {
	matches: Array<RouteMatch | undefined>;
	context: string;
}): string[] {
	const { matches, context } = props;
	const matchedPatterns: string[] = [];
	for (let i = 0; i < matches.length; i++) {
		matchedPatterns.push(
			getMatchedPatternOrThrow({
				match: matches[i],
				index: i,
				context,
			}),
		);
	}

	return matchedPatterns;
}

export function buildServerSuccessPreloadPlan(props: {
	signalAborted: boolean;
	isDev: boolean;
	importURLs: string[];
	deps: string[];
	cssBundles: string[];
}): ServerSuccessPreloadPlan {
	if (props.signalAborted) {
		return {
			moduleDependencies: [],
			cssBundles: [],
		};
	}

	const moduleDependenciesToPreload = props.isDev
		? [...new Set(props.importURLs)]
		: props.deps;
	const moduleDependencies: string[] = [];
	for (const dependency of moduleDependenciesToPreload) {
		if (typeof dependency !== "string" || dependency.length === 0) {
			continue;
		}
		moduleDependencies.push(dependency);
	}

	const cssBundles: string[] = [];
	for (const bundle of props.cssBundles) {
		if (typeof bundle !== "string" || bundle.length === 0) {
			continue;
		}
		cssBundles.push(bundle);
	}

	return {
		moduleDependencies,
		cssBundles,
	};
}

export function buildRouteDataRequestURL(props: {
	targetHref: string;
	navigationType: NavigateProps["navigationType"];
}): URL {
	const buildID = __vormaClientGlobal.get("buildID") || "1";
	const deploymentID = __vormaClientGlobal.get("deploymentID");
	const url = new URL(props.targetHref);
	url.searchParams.set("vorma_json", buildID);

	if (props.navigationType === "revalidation") {
		if (deploymentID) {
			url.searchParams.set("dpl", deploymentID);
		}
	}

	return url;
}

export type ServerRouteDataResult = {
	redirectData: RedirectData | null;
	response?: Response;
	json?: GetRouteDataOutput;
};

export async function createServerRouteDataPromise(props: {
	abortController: AbortController;
	url: URL;
	redirectCount?: number;
}): Promise<ServerRouteDataResult> {
	const result = await handleRedirects(props);

	if (result.response && result.response.ok && !result.redirectData?.status) {
		const json = await result.response.json();
		return { ...result, json };
	}

	return { ...result, json: undefined };
}

export type ResolvedServerRouteDataResult =
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
	const responseNotOK = !response?.ok;

	if (redirected || !response) {
		controller.abort();
		return { type: "outcome", outcome: { type: "aborted" } };
	}

	if (response.status === 304) {
		controller.abort();
		throw new Error("Fetch returned 304 without route JSON payload.");
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

function buildServerDataByMatchedPattern(props: {
	matchedPatterns: string[];
	serverResult: ServerRouteDataResult;
}): Map<string, ClientLoaderAwaitedServerData<unknown, unknown>> {
	const serverDataByPattern = new Map<
		string,
		ClientLoaderAwaitedServerData<unknown, unknown>
	>();

	for (const pattern of props.matchedPatterns) {
		const serverData = buildServerDataForPattern(
			pattern,
			props.serverResult,
		);
		if (serverData) {
			serverDataByPattern.set(pattern, serverData);
		}
	}

	return serverDataByPattern;
}

export async function startParallelClientLoaders(props: {
	pathname: string;
	serverPromise: Promise<ServerRouteDataResult>;
	signal: AbortSignal;
}): Promise<Map<string, Promise<unknown>>> {
	const matchResult = await findPartialMatchesOnClient(props.pathname);
	const patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"] =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const runningLoaders = new Map<string, Promise<unknown>>();

	if (!matchResult) {
		return runningLoaders;
	}

	const { params, splatValues, matches } = matchResult;
	const matchedPatterns = getMatchedPatternsOrThrow({
		matches: matches as Array<RouteMatch | undefined>,
		context: "Partial route matcher",
	});
	const serverDataByPatternPromise = props.serverPromise.then(
		(serverResult) =>
			buildServerDataByMatchedPattern({
				matchedPatterns,
				serverResult,
			}),
		() => {
			throw createUnavailableServerDataError();
		},
	);

	for (const pattern of matchedPatterns) {
		const loaderFn = patternToWaitFnMap[pattern];

		if (!loaderFn) {
			continue;
		}

		const serverDataPromise = serverDataByPatternPromise.then(
			(serverDataByPattern) => {
				const serverData = serverDataByPattern.get(pattern);
				if (serverData) {
					return serverData;
				}
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

export function buildServerSuccessOutcome(props: {
	response: Response;
	json: GetRouteDataOutput;
	navigationProps: NavigateProps;
	runningLoaders: Map<string, Promise<unknown>>;
	signal: AbortSignal;
}): Extract<NavigationOutcome, { type: "success" }> {
	const { response, json, navigationProps, runningLoaders, signal } = props;
	const preloadPlan = buildServerSuccessPreloadPlan({
		signalAborted: signal.aborted,
		isDev: import.meta.env.DEV,
		importURLs: json.importURLs ?? [],
		deps: json.deps ?? [],
		cssBundles: json.cssBundles ?? [],
	});
	const buildID = getBuildIDFromResponse(response);

	const waitFnPromise = completeClientLoaders(
		json,
		buildID,
		runningLoaders,
		signal,
	);
	observePromiseRejection(waitFnPromise);

	return {
		type: "success",
		response,
		json,
		props: navigationProps,
		preloadPlan,
		waitFnPromise,
	};
}

export async function fetchRouteData(
	controller: AbortController,
	props: NavigateProps,
): Promise<NavigationOutcome> {
	try {
		const targetURL = new URL(props.href, window.location.href);
		const requestURL = buildRouteDataRequestURL({
			targetHref: targetURL.href,
			navigationType: props.navigationType,
		});

		const serverPromise = createServerRouteDataPromise({
			abortController: controller,
			url: requestURL,
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

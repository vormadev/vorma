import {
	buildClientLoaderServerData,
	completeClientLoaders,
	createUnavailableServerDataError,
	findPartialMatchesOnClient,
} from "../render_runtime.ts";
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
import {
	getMatchedPatternsOrThrow,
	type SkipMatch,
} from "./fetch_route_data_skip_match.ts";
import { getClientOnlyOutcomeIfSkippable } from "./fetch_route_data_skip.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

type RouteDataRequestGlobalSnapshot = {
	buildID: string;
	deploymentID: string;
};

type StartParallelClientLoadersGlobalSnapshot = {
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
};

export type ServerSuccessPreloadExecutionPlan =
	| {
			type: "skip";
			reason: "server_success_preload_skipped_signal_aborted";
	  }
	| {
			type: "preload";
			moduleDependenciesToPreload: string[];
			cssBundlesToPreload: string[];
			reason: "server_success_preload_allowed";
	  };

export function decideServerSuccessPreloadExecutionPlan(props: {
	signalAborted: boolean;
	isDev: boolean;
	importURLs: string[];
	deps: string[];
	cssBundles: string[];
}): ServerSuccessPreloadExecutionPlan {
	if (props.signalAborted) {
		return {
			type: "skip",
			reason: "server_success_preload_skipped_signal_aborted",
		};
	}

	const moduleDependenciesToPreload = props.isDev
		? [...new Set(props.importURLs)]
		: props.deps;

	return {
		type: "preload",
		moduleDependenciesToPreload,
		cssBundlesToPreload: props.cssBundles,
		reason: "server_success_preload_allowed",
	};
}

export type ServerSuccessPreloadCommand =
	| {
			type: "preload_module_dependency";
			dependency: string;
			reason: "server_success_preload_allowed";
	  }
	| {
			type: "preload_css_bundle";
			bundle: string;
			reason: "server_success_preload_allowed";
	  };

export function buildServerSuccessPreloadCommands(props: {
	executionPlan: ServerSuccessPreloadExecutionPlan;
}): ServerSuccessPreloadCommand[] {
	if (props.executionPlan.type === "skip") {
		return [];
	}

	const commands: ServerSuccessPreloadCommand[] = [];
	for (const dependency of props.executionPlan.moduleDependenciesToPreload) {
		if (typeof dependency !== "string" || dependency.length === 0) {
			continue;
		}
		commands.push({
			type: "preload_module_dependency",
			dependency,
			reason: props.executionPlan.reason,
		});
	}
	for (const bundle of props.executionPlan.cssBundlesToPreload) {
		if (typeof bundle !== "string" || bundle.length === 0) {
			continue;
		}
		commands.push({
			type: "preload_css_bundle",
			bundle,
			reason: props.executionPlan.reason,
		});
	}

	return commands;
}

function getRouteDataRequestGlobalSnapshot(): RouteDataRequestGlobalSnapshot {
	return {
		buildID: __vormaClientGlobal.get("buildID") || "1",
		deploymentID: __vormaClientGlobal.get("deploymentID"),
	};
}

function getStartParallelClientLoadersGlobalSnapshot(): StartParallelClientLoadersGlobalSnapshot {
	return {
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap") || {},
	};
}

export function buildRouteDataRequestURL(props: {
	targetHref: string;
	navigationType: NavigateProps["navigationType"];
}): URL {
	const { buildID, deploymentID } = getRouteDataRequestGlobalSnapshot();
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

export async function startParallelClientLoaders(props: {
	pathname: string;
	serverPromise: Promise<ServerRouteDataResult>;
	signal: AbortSignal;
}): Promise<Map<string, Promise<unknown>>> {
	const matchResult = await findPartialMatchesOnClient(props.pathname);
	const { patternToWaitFnMap } =
		getStartParallelClientLoadersGlobalSnapshot();
	const runningLoaders = new Map<string, Promise<unknown>>();

	if (!matchResult) {
		return runningLoaders;
	}

	const { params, splatValues, matches } = matchResult;
	const matchedPatterns = getMatchedPatternsOrThrow({
		matches: matches as Array<SkipMatch | undefined>,
		context: "Partial route matcher",
	});

	for (const pattern of matchedPatterns) {
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

export function buildServerSuccessOutcome(props: {
	response: Response;
	json: GetRouteDataOutput;
	navigationProps: NavigateProps;
	runningLoaders: Map<string, Promise<unknown>>;
	signal: AbortSignal;
}): Extract<NavigationOutcome, { type: "success" }> {
	const { response, json, navigationProps, runningLoaders, signal } = props;
	const preloadExecutionPlan = decideServerSuccessPreloadExecutionPlan({
		signalAborted: signal.aborted,
		isDev: import.meta.env.DEV,
		importURLs: json.importURLs ?? [],
		deps: json.deps ?? [],
		cssBundles: json.cssBundles ?? [],
	});
	const preloadCommands = buildServerSuccessPreloadCommands({
		executionPlan: preloadExecutionPlan,
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
		preloadCommands,
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

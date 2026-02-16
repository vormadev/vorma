import {
	buildClientLoaderServerData,
	completeClientLoaders,
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
import { observePromiseRejection } from "../../platform/safety.ts";
import {
	getMatchedPatternsOrThrow,
	type SkipMatch,
} from "./fetch_route_data_skip_match.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

type RouteDataRequestGlobalSnapshot = {
	buildID: string;
	deploymentID: string;
};

type StartParallelClientLoadersGlobalSnapshot = {
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
};

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

	if (!signal.aborted) {
		const depsToPreload = import.meta.env.DEV
			? [...new Set(json.importURLs)]
			: json.deps;
		for (const dep of depsToPreload ?? []) {
			if (dep) AssetManager.preloadModule(dep);
		}
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
	if (!signal.aborted) {
		for (const bundle of json.cssBundles ?? []) {
			cssBundlePromises.push(AssetManager.preloadCSS(bundle));
		}
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

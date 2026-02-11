import { findPartialMatchesOnClient } from "../client_loaders.ts";
import {
	buildClientLoaderServerData,
	createUnavailableServerDataError,
} from "../client_loader_server_data.ts";
import { observePromiseRejection } from "../utils/promise_safety.ts";
import { getBuildIDFromResponse } from "../redirects/redirects.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import type { ClientLoaderAwaitedServerData } from "../vorma_ctx/vorma_ctx.ts";
import type { ServerRouteDataResult } from "./fetch_route_data_server.ts";

function buildServerDataForPattern(
	pattern: string,
	serverResult: ServerRouteDataResult,
): ClientLoaderAwaitedServerData<any, any> | null {
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
}): Promise<Map<string, Promise<any>>> {
	const matchResult = await findPartialMatchesOnClient(props.pathname);
	const patternToWaitFnMap = __vormaClientGlobal.get("patternToWaitFnMap");
	const runningLoaders = new Map<string, Promise<any>>();

	if (!matchResult) {
		return runningLoaders;
	}

	const { params, splatValues, matches } = matchResult;

	for (let i = 0; i < matches.length; i++) {
		const match = matches[i];
		if (!match) continue;

		const pattern = match.registeredPattern.originalPattern;
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

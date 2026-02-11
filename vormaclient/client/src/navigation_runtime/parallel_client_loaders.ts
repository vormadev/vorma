import { findPartialMatchesOnClient } from "../client_loaders.ts";
import {
	buildClientLoaderServerData,
	createUnavailableServerDataError,
} from "../client_loader_server_data.ts";
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
		// These promises may outlive/escape the current navigation (for example
		// pure prefetches that resolve to redirect/abort outcomes). Attach a
		// side-chain catch so abandoned loaders never surface as unhandled
		// rejections. The original promise still rejects when awaited later by
		// completeClientLoaders(...).
		void loaderPromise.catch(() => {});

		runningLoaders.set(pattern, loaderPromise);
	}

	return runningLoaders;
}

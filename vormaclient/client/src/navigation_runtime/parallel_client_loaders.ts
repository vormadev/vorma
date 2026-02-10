import { findPartialMatchesOnClient } from "../client_loaders.ts";
import { getBuildIDFromResponse } from "../redirects/redirects.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import type { ClientLoaderAwaitedServerData } from "../vorma_ctx/vorma_ctx.ts";
import type { ServerRouteDataResult } from "./fetch_route_data_server.ts";

function buildEmptyServerData(): ClientLoaderAwaitedServerData<any, any> {
	return {
		matchedPatterns: [],
		loaderData: undefined,
		rootData: null,
		buildID: "1",
	};
}

function buildServerDataForPattern(
	pattern: string,
	serverResult: ServerRouteDataResult,
): ClientLoaderAwaitedServerData<any, any> {
	const { response, json } = serverResult;
	if (!response || !response.ok || !json) {
		return buildEmptyServerData();
	}

	const serverIdx = json.matchedPatterns?.indexOf(pattern);
	const loaderData =
		serverIdx !== -1 && serverIdx !== undefined
			? json.loadersData[serverIdx]
			: undefined;
	const rootData = json.hasRootData ? json.loadersData[0] : null;
	const buildID = getBuildIDFromResponse(response) || "1";

	return {
		matchedPatterns: json.matchedPatterns || [],
		loaderData,
		rootData,
		buildID,
	};
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

		const serverDataPromise = props.serverPromise
			.then((serverResult) =>
				buildServerDataForPattern(pattern, serverResult),
			)
			.catch(() => buildEmptyServerData());

		const loaderPromise = loaderFn({
			params,
			splatValues,
			serverDataPromise,
			signal: props.signal,
		});

		runningLoaders.set(pattern, loaderPromise);
	}

	return runningLoaders;
}

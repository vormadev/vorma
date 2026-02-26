import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { registerPattern } from "vorma/kit/matcher/register";
import {
	__vormaClientGlobal,
	getRuntimeRouteSnapshot,
	updateRuntimeRouteSnapshot,
	type ClientLoaderAwaitedServerData,
	type GetRouteDataOutput,
	type VormaClientGlobal,
} from "../app/context.ts";
import { isAbortError, logError } from "../platform/safety.ts";
import {
	ComponentLoader,
	getEffectiveErrorDataFromSnapshot,
} from "./render_component_runtime.ts";

export type PartialWaitFnJSON = Pick<
	GetRouteDataOutput,
	| "matchedPatterns"
	| "splatValues"
	| "params"
	| "hasRootData"
	| "loadersData"
	| "outermostServerErrorIdx"
	| "importURLs"
>;

export type ClientLoadersResult = {
	data: Array<unknown>;
	errorMessage?: string;
};

export function createUnavailableServerDataError(): Error {
	const error = new Error(
		"Server loader data is unavailable for abandoned or failed navigation.",
	);
	error.name = "AbortError";
	return error;
}

function patternRequiresServerData(pattern: string): boolean {
	const routeManifest = __vormaClientGlobal.get("routeManifest");
	return routeManifest?.[pattern] === 1;
}

export function buildClientLoaderServerData(props: {
	pattern: string;
	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	hasRootData: boolean;
	buildID: string;
}): ClientLoaderAwaitedServerData<unknown, unknown> | null {
	const { pattern, matchedPatterns, loadersData, hasRootData, buildID } =
		props;
	const serverIdx = matchedPatterns.indexOf(pattern);

	if (serverIdx === -1) {
		return null;
	}

	const loaderData = loadersData[serverIdx];
	const rootData = hasRootData ? loadersData[0] : null;

	if (patternRequiresServerData(pattern) && loaderData === undefined) {
		return null;
	}

	if (hasRootData && rootData === undefined) {
		return null;
	}

	return {
		matchedPatterns,
		loaderData,
		rootData,
		buildID,
	};
}

type ClientLoaderWorkItems = {
	loaderPromises: Array<Promise<unknown>>;
	abortControllers: Array<AbortController | null>;
};

function buildClientLoaderWorkItems(props: {
	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	params: Record<string, string>;
	splatValues: Array<string>;
	hasRootData: boolean;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	outermostServerErrorIdx: number | undefined;
	runningLoaders?: Map<string, Promise<unknown>>;
	signal: AbortSignal;
	buildID: string;
}): ClientLoaderWorkItems {
	const {
		matchedPatterns,
		loadersData,
		params,
		splatValues,
		hasRootData,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		buildID,
	} = props;

	const loaderPromises: Array<Promise<unknown>> = [];
	const abortControllers: Array<AbortController | null> = [];

	let i = 0;
	for (const pattern of matchedPatterns) {
		if (
			outermostServerErrorIdx !== undefined &&
			i === outermostServerErrorIdx
		) {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
			i++;
			continue;
		}

		if (runningLoaders?.has(pattern)) {
			loaderPromises.push(runningLoaders.get(pattern)!);
			abortControllers.push(null);
		} else if (patternToWaitFnMap[pattern]) {
			const controller = new AbortController();
			abortControllers.push(controller);

			if (signal.aborted) {
				controller.abort();
			} else {
				signal.addEventListener("abort", () => controller.abort(), {
					once: true,
				});
			}

			const serverData = buildClientLoaderServerData({
				pattern,
				matchedPatterns,
				loadersData,
				hasRootData,
				buildID,
			});
			const serverDataPromise = serverData
				? Promise.resolve(serverData)
				: Promise.reject(createUnavailableServerDataError());

			const loaderPromise = patternToWaitFnMap[pattern]({
				params,
				splatValues,
				serverDataPromise,
				signal: controller.signal,
			});
			loaderPromises.push(loaderPromise);
		} else {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
		}
		i++;
	}

	return { loaderPromises, abortControllers };
}

function wrapLoaderPromisesWithChildAbort(props: {
	loaderPromises: Array<Promise<unknown>>;
	abortControllers: Array<AbortController | null>;
}): Array<Promise<unknown>> {
	const { loaderPromises, abortControllers } = props;
	return loaderPromises.map(async (promise, index) => {
		return promise.catch((error) => {
			if (!isAbortError(error)) {
				for (let j = index + 1; j < abortControllers.length; j++) {
					abortControllers[j]?.abort();
				}
			}
			throw error;
		});
	});
}

function processSettledClientLoaderResults(props: {
	results: Array<PromiseSettledResult<unknown>>;
	matchedPatterns: Array<string>;
}): {
	data: Array<unknown>;
	errorMessage: string | undefined;
} {
	const { results, matchedPatterns } = props;
	const data: Array<unknown> = [];
	let errorMessage: string | undefined;

	for (const [resultIndex, result] of results.entries()) {
		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			if (!isAbortError(result.reason)) {
				const pattern = matchedPatterns[resultIndex];
				logError(
					`Client loader error for pattern ${pattern}:`,
					result.reason,
				);
				errorMessage =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);
			}
			data.push(undefined);
			break;
		}
	}

	return { data, errorMessage };
}

async function executeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
	runningLoaders?: Map<string, Promise<unknown>>,
): Promise<ClientLoadersResult> {
	await ComponentLoader.loadComponents(json.importURLs ?? []);

	const matchedPatterns = json.matchedPatterns ?? [];
	const loadersData = json.loadersData ?? [];
	const params = json.params ?? {};
	const splatValues = json.splatValues ?? [];
	const hasRootData = !!json.hasRootData;
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const outermostServerErrorIdx = json.outermostServerErrorIdx;

	const { loaderPromises, abortControllers } = buildClientLoaderWorkItems({
		matchedPatterns,
		loadersData,
		params,
		splatValues,
		hasRootData,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		buildID,
	});

	const wrappedPromises = wrapLoaderPromisesWithChildAbort({
		loaderPromises,
		abortControllers,
	});
	const results = await Promise.allSettled(wrappedPromises);

	const { data, errorMessage } = processSettledClientLoaderResults({
		results,
		matchedPatterns,
	});

	return { data, errorMessage };
}

function buildClientLoaderSnapshotFromGlobal(): PartialWaitFnJSON {
	const runtimeRouteSnapshot = getRuntimeRouteSnapshot();
	return {
		hasRootData: runtimeRouteSnapshot.hasRootData,
		importURLs: runtimeRouteSnapshot.importURLs,
		loadersData: runtimeRouteSnapshot.loadersData,
		matchedPatterns: runtimeRouteSnapshot.matchedPatterns,
		outermostServerErrorIdx: runtimeRouteSnapshot.outermostServerErrorIdx,
		params: runtimeRouteSnapshot.params,
		splatValues: runtimeRouteSnapshot.splatValues,
	};
}

export function setClientLoadersState(
	clientLoadersResult: ClientLoadersResult | undefined,
): void {
	if (!clientLoadersResult) {
		return;
	}

	const normalizedClientLoaderData = clientLoadersResult.data ?? [];
	updateRuntimeRouteSnapshot({
		updater: (runtimeRouteSnapshot) => {
			const outermostClientErrorIdx = clientLoadersResult.errorMessage
				? normalizedClientLoaderData.length > 0
					? normalizedClientLoaderData.length - 1
					: undefined
				: undefined;
			const nextSnapshot = {
				...runtimeRouteSnapshot,
				clientLoadersData: normalizedClientLoaderData,
				outermostClientErrorIdx,
				outermostClientError: clientLoadersResult.errorMessage,
			};
			const effectiveErrData = getEffectiveErrorDataFromSnapshot({
				outermostServerErrorIdx: nextSnapshot.outermostServerErrorIdx,
				outermostClientErrorIdx: nextSnapshot.outermostClientErrorIdx,
				outermostServerError: nextSnapshot.outermostServerError,
				outermostClientError: nextSnapshot.outermostClientError,
			});
			return {
				...nextSnapshot,
				outermostErrorIdx: effectiveErrData.index,
				outermostError: effectiveErrData.error,
			};
		},
	});
}

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await executeClientLoaders(
		buildClientLoaderSnapshotFromGlobal(),
		__vormaClientGlobal.get("buildID"),
		new AbortController().signal,
	);

	setClientLoadersState(clientLoadersResult);
}

export function registerClientLoaderPatternOrThrow(pattern: string): void {
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		throw new Error("Pattern registry has not been initialized.");
	}
	registerPattern(patternRegistry, pattern);
}

export async function registerClientLoaderPattern(
	pattern: string,
): Promise<void> {
	registerClientLoaderPatternOrThrow(pattern);
}

export const __registerClientLoaderPattern = registerClientLoaderPattern;

export async function findPartialMatchesOnClient(pathname: string) {
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	if (!patternRegistry) {
		return null;
	}

	if (Object.keys(patternToWaitFnMap).length === 0) {
		return null;
	}

	const fullResult = findNestedMatches(patternRegistry, pathname);
	if (fullResult) {
		return fullResult;
	}

	const segments = pathname.split("/").filter(Boolean);
	for (let i = segments.length; i >= 0; i--) {
		const partialPath =
			i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
		const result = findNestedMatches(patternRegistry, partialPath);
		if (result) {
			return result;
		}
	}

	return null;
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<unknown>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

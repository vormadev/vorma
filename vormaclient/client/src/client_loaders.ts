import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { registerPattern } from "vorma/kit/matcher/register";
import { ComponentLoader, getEffectiveErrorData } from "./component_loader.ts";
import { isAbortError } from "./utils/errors.ts";
import { logError } from "./utils/logging.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

export function setClientLoadersState(
	clr: ClientLoadersResult | undefined,
): void {
	if (clr) {
		__vormaClientGlobal.set("clientLoadersData", clr.data ?? []);
		__vormaClientGlobal.set(
			"outermostClientErrorIdx",
			clr.errorMessage ? clr.data.length - 1 : undefined,
		);
		__vormaClientGlobal.set("outermostClientError", clr.errorMessage);
	}
}

export function deriveAndSetErrorState(): void {
	const effectiveErrData = getEffectiveErrorData();
	__vormaClientGlobal.set("outermostErrorIdx", effectiveErrData.index);
	__vormaClientGlobal.set("outermostError", effectiveErrData.error);
}

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await runWaitFns(
		{
			hasRootData: __vormaClientGlobal.get("hasRootData"),
			importURLs: __vormaClientGlobal.get("importURLs"),
			loadersData: __vormaClientGlobal.get("loadersData"),
			matchedPatterns: __vormaClientGlobal.get("matchedPatterns"),
			params: __vormaClientGlobal.get("params"),
			splatValues: __vormaClientGlobal.get("splatValues"),
		},
		__vormaClientGlobal.get("buildID"),
		new AbortController().signal,
	);

	setClientLoadersState(clientLoadersResult);
	deriveAndSetErrorState();
}

export async function __registerClientLoaderPattern(
	pattern: string,
): Promise<void> {
	registerPattern(__vormaClientGlobal.get("patternRegistry"), pattern);
}

// This is needed because the matcher, by definition, will only
// match when you have a full path match. If the path you are
// testing is longer than the registered patterns, you will get
// no match, even if some registered patterns would potentially
// be in the parent segments. This fixes that.
export async function findPartialMatchesOnClient(pathname: string) {
	const patternToWaitFnMap = __vormaClientGlobal.get("patternToWaitFnMap");
	if (Object.keys(patternToWaitFnMap).length === 0) {
		return null;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");

	// First try the full path
	const fullResult = findNestedMatches(patternRegistry, pathname);
	if (fullResult) {
		// If we get a full match, we have everything we need
		return fullResult;
	}

	// If no full match, try progressively shorter paths to find partial matches
	const segments = pathname.split("/").filter(Boolean);

	// Try from longest to shortest
	for (let i = segments.length; i >= 0; i--) {
		const partialPath =
			i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
		const result = findNestedMatches(patternRegistry, partialPath);
		if (result) {
			return result; // First match is the longest
		}
	}

	return null;
}

type PartialWaitFnJSON = Pick<
	GetRouteDataOutput,
	| "matchedPatterns"
	| "splatValues"
	| "params"
	| "hasRootData"
	| "loadersData"
	| "importURLs"
>;

export type ClientLoadersResult = {
	data: Array<any>;
	errorMessage?: string;
};

async function executeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
	runningLoaders?: Map<string, Promise<any>>,
): Promise<ClientLoadersResult> {
	await ComponentLoader.loadComponents(json.importURLs);

	const matchedPatterns = json.matchedPatterns ?? [];
	const patternToWaitFnMap = __vormaClientGlobal.get("patternToWaitFnMap");
	const outermostServerErrorIdx = __vormaClientGlobal.get(
		"outermostServerErrorIdx",
	);

	const loaderPromises: Array<Promise<any>> = [];
	const abortControllers: Array<AbortController | null> = [];

	// Build arrays of all promises and their corresponding abort controllers
	let i = 0;
	for (const pattern of matchedPatterns) {
		if (
			outermostServerErrorIdx !== undefined &&
			i === outermostServerErrorIdx
		) {
			// This route has a server error, skip its client loader
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
			i++;
			continue;
		}

		if (runningLoaders?.has(pattern)) {
			// This loader is already running (started parallel to fetch)
			loaderPromises.push(runningLoaders.get(pattern)!);
			// We can't create a new controller for it, but we can wrap it
			abortControllers.push(null);
		} else if (patternToWaitFnMap[pattern]) {
			// This is a new client loader we need to run
			const controller = new AbortController();
			abortControllers.push(controller);

			// Wire up the main navigation signal to this loader's controller
			if (signal.aborted) {
				controller.abort();
			} else {
				signal.addEventListener("abort", () => controller.abort(), {
					once: true,
				});
			}

			const serverDataPromise = Promise.resolve({
				matchedPatterns: json.matchedPatterns,
				loaderData: json.loadersData[i],
				rootData: json.hasRootData ? json.loadersData[0] : null,
				buildID: buildID,
			});

			const loaderPromise = patternToWaitFnMap[pattern]({
				params: json.params || {},
				splatValues: json.splatValues || [],
				serverDataPromise,
				signal: controller.signal,
			});
			loaderPromises.push(loaderPromise);
		} else {
			// No client loader for this route
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
		}
		i++;
	}

	// Wrap all promises with the child-aborting logic
	const wrappedPromises = loaderPromises.map(async (promise, index) => {
		return promise.catch((error) => {
			// If this promise failed with a true error (not just an abort)
			if (!isAbortError(error)) {
				// Abort all subsequent (child) loaders immediately
				for (let j = index + 1; j < abortControllers.length; j++) {
					abortControllers[j]?.abort();
				}
			}
			// Re-throw the error so Promise.allSettled sees it as 'rejected'
			throw error;
		});
	});

	// Await all wrapped promises. They run in parallel,
	// but a rejection in one now triggers aborts in its children.
	const results = await Promise.allSettled(wrappedPromises);

	// Process the results
	const data: Array<any> = [];
	let errorMessage: string | undefined;

	for (let i = 0; i < results.length; i++) {
		const result = results[i];
		if (!result) {
			data.push(undefined);
			continue;
		}

		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			// This is a rejection
			if (!isAbortError(result.reason)) {
				// This is the first true error we've hit
				const pattern = matchedPatterns[i];
				logError(
					`Client loader error for pattern ${pattern}:`,
					result.reason,
				);
				errorMessage =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);

				// We found the highest error. Stop processing.
				// The .catch() wrapper already aborted any children.
			}
			data.push(undefined);
			break; // Stop at the first error
		}
	}

	return { data, errorMessage };
}

async function runWaitFns(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal);
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<any>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

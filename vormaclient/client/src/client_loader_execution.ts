import { ComponentLoader } from "./component_loader.ts";
import { isAbortError } from "./utils/errors.ts";
import { logError } from "./utils/logging.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

export type PartialWaitFnJSON = Pick<
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

export async function executeClientLoaders(
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
				buildID,
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

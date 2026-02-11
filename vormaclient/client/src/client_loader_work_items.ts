import type {
	GetRouteDataOutput,
	VormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

type ClientLoaderJSON = Pick<
	GetRouteDataOutput,
	| "matchedPatterns"
	| "splatValues"
	| "params"
	| "hasRootData"
	| "loadersData"
	| "importURLs"
>;

export type ClientLoaderWorkItems = {
	loaderPromises: Array<Promise<any>>;
	abortControllers: Array<AbortController | null>;
};

export function buildClientLoaderWorkItems(props: {
	matchedPatterns: Array<string>;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	outermostServerErrorIdx: number | undefined;
	runningLoaders?: Map<string, Promise<any>>;
	signal: AbortSignal;
	json: ClientLoaderJSON;
	buildID: string;
}): ClientLoaderWorkItems {
	const {
		matchedPatterns,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		json,
		buildID,
	} = props;

	const loaderPromises: Array<Promise<any>> = [];
	const abortControllers: Array<AbortController | null> = [];

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

	return { loaderPromises, abortControllers };
}

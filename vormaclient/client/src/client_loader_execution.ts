import { ComponentLoader } from "./component_loader.ts";
import { wrapLoaderPromisesWithChildAbort } from "./client_loader_promise_wrapping.ts";
import { processSettledClientLoaderResults } from "./client_loader_result_processing.ts";
import { buildClientLoaderWorkItems } from "./client_loader_work_items.ts";
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

	const { loaderPromises, abortControllers } = buildClientLoaderWorkItems({
		matchedPatterns,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		json,
		buildID,
	});

	// Wrap all promises with the child-aborting logic
	const wrappedPromises = wrapLoaderPromisesWithChildAbort({
		loaderPromises,
		abortControllers,
	});

	// Await all wrapped promises. They run in parallel,
	// but a rejection in one now triggers aborts in its children.
	const results = await Promise.allSettled(wrappedPromises);

	const { data, errorMessage } = processSettledClientLoaderResults({
		results,
		matchedPatterns,
	});

	return { data, errorMessage };
}

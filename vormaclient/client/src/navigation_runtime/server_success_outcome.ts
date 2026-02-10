import { AssetManager } from "../asset_manager.ts";
import { completeClientLoaders } from "../client_loaders.ts";
import { getBuildIDFromResponse } from "../redirects/redirects.ts";
import type { GetRouteDataOutput } from "../vorma_ctx/vorma_ctx.ts";
import type { NavigateProps, NavigationOutcome } from "./types.ts";

export function buildServerSuccessOutcome(props: {
	response: Response;
	json: GetRouteDataOutput;
	navigationProps: NavigateProps;
	runningLoaders: Map<string, Promise<any>>;
	signal: AbortSignal;
}): Extract<NavigationOutcome, { type: "success" }> {
	const { response, json, navigationProps, runningLoaders, signal } = props;

	// deps are only present in prod because they stem from the rollup metafile
	const depsToPreload = import.meta.env.DEV
		? [...new Set(json.importURLs)]
		: json.deps;
	for (const dep of depsToPreload ?? []) {
		if (dep) AssetManager.preloadModule(dep);
	}

	const buildID = getBuildIDFromResponse(response);

	// Complete client loader execution
	const waitFnPromise = completeClientLoaders(
		json,
		buildID,
		runningLoaders,
		signal,
	);

	const cssBundlePromises: Array<Promise<any>> = [];
	for (const bundle of json.cssBundles ?? []) {
		cssBundlePromises.push(AssetManager.preloadCSS(bundle));
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

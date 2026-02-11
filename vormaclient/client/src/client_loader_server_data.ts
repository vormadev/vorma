import {
	__vormaClientGlobal,
	type ClientLoaderAwaitedServerData,
} from "./vorma_ctx/vorma_ctx.ts";

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
	loadersData: Array<any>;
	hasRootData: boolean;
	buildID: string;
}): ClientLoaderAwaitedServerData<any, any> | null {
	const { pattern, matchedPatterns, loadersData, hasRootData, buildID } =
		props;
	const serverIdx = matchedPatterns.indexOf(pattern);

	if (serverIdx === -1) {
		return null;
	}

	const loaderData = loadersData[serverIdx];
	const rootData = hasRootData ? loadersData[0] : null;

	// If a route is declared as server-loaded in the manifest, the response
	// payload must include a value at that pattern index (null is allowed).
	if (patternRequiresServerData(pattern) && loaderData === undefined) {
		return null;
	}

	// Root data is index 0 whenever hasRootData is true.
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

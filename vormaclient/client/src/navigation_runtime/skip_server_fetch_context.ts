import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import type { SkipCheckContext } from "./skip_server_fetch_types.ts";

export function buildSkipCheckContext(
	targetUrl: string,
): SkipCheckContext | undefined {
	// Early return: no route manifest
	const routeManifest = __vormaClientGlobal.get("routeManifest");
	if (!routeManifest) {
		return undefined;
	}

	// Early return: no pattern registry
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		return undefined;
	}

	// Early return: no match
	const url = new URL(targetUrl);
	const matchResult = findNestedMatches(patternRegistry, url.pathname);
	if (!matchResult) {
		return undefined;
	}

	return {
		routeManifest,
		patternRegistry,
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap") || {},
		clientModuleMap: __vormaClientGlobal.get("clientModuleMap") || {},
		currentMatchedPatterns:
			__vormaClientGlobal.get("matchedPatterns") || [],
		currentParams: __vormaClientGlobal.get("params") || {},
		currentSplatValues: __vormaClientGlobal.get("splatValues") || [],
		currentLoadersData: __vormaClientGlobal.get("loadersData") || [],
		url,
		matchResult,
	};
}

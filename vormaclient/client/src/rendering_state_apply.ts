import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

export function applyRouteDataToGlobalState(json: GetRouteDataOutput): void {
	const stateKeys = [
		"outermostServerError",
		"outermostServerErrorIdx",
		"errorExportKeys",
		"matchedPatterns",
		"loadersData",
		"importURLs",
		"exportKeys",
		"hasRootData",
		"params",
		"splatValues",
	] as const;

	for (const key of stateKeys) {
		__vormaClientGlobal.set(key, json[key]);
	}
}

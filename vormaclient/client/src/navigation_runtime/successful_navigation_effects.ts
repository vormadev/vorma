import { AssetManager } from "../asset_manager.ts";
import { dispatchBuildIDEvent } from "../events.ts";
import { getBuildIDFromResponse } from "../redirects/redirects.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../vorma_ctx/vorma_ctx.ts";

export function applyResponseArtifactsWhenBuildMatches(
	response: Response,
	json: GetRouteDataOutput,
): void {
	const currentBuildID = __vormaClientGlobal.get("buildID");
	const responseBuildID = getBuildIDFromResponse(response);

	if (responseBuildID !== currentBuildID) {
		return;
	}

	const clientModuleMap = __vormaClientGlobal.get("clientModuleMap") || {};
	const matchedPatterns = json.matchedPatterns || [];
	const importURLs = json.importURLs || [];
	const exportKeys = json.exportKeys || [];
	const errorExportKeys = json.errorExportKeys || [];

	for (let i = 0; i < matchedPatterns.length; i++) {
		const pattern = matchedPatterns[i];
		const importURL = importURLs[i];
		const exportKey = exportKeys[i];
		const errorExportKey = errorExportKeys[i];

		if (pattern && importURL) {
			clientModuleMap[pattern] = {
				importURL,
				exportKey: exportKey || "default",
				errorExportKey: errorExportKey || "",
			};
		}
	}

	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);

	// Apply CSS bundles immediately, even for prefetches.
	if (json.cssBundles && json.cssBundles.length > 0) {
		AssetManager.applyCSS(json.cssBundles);
	}
}

export function syncBuildIDFromResponse(response: Response): void {
	const oldID = __vormaClientGlobal.get("buildID");
	const newID = getBuildIDFromResponse(response);
	if (!newID || newID === oldID) {
		return;
	}

	__vormaClientGlobal.set("buildID", newID);
	dispatchBuildIDEvent({ newID, oldID });
}

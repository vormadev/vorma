import type { VormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function initializeClientModuleMapFromInitialRouteState(): void {
	const clientModuleMap: VormaClientGlobal["clientModuleMap"] = {};

	const initialMatchedPatterns =
		__vormaClientGlobal.get("matchedPatterns") || [];
	const initialImportURLs = __vormaClientGlobal.get("importURLs") || [];
	const initialExportKeys = __vormaClientGlobal.get("exportKeys") || [];
	const initialErrorExportKeys =
		__vormaClientGlobal.get("errorExportKeys") || [];

	for (let i = 0; i < initialMatchedPatterns.length; i++) {
		const pattern = initialMatchedPatterns[i];
		const importURL = initialImportURLs[i];
		const exportKey = initialExportKeys[i];
		const errorExportKey = initialErrorExportKeys[i];

		if (pattern && importURL) {
			clientModuleMap[pattern] = {
				importURL,
				exportKey: exportKey || "default",
				errorExportKey: errorExportKey || "",
			};
		}
	}

	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);
}

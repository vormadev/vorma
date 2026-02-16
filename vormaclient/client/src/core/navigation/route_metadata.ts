import type { VormaClientGlobal } from "../../app/context.ts";

export type RouteModuleMetadataInput = {
	matchedPatterns?: Array<string>;
	importURLs?: Array<string>;
	exportKeys?: Array<string>;
	errorExportKeys?: Array<string>;
};

function toRouteModuleMetadataArrays(props: RouteModuleMetadataInput): {
	matchedPatterns: Array<string>;
	importURLs: Array<string>;
	exportKeys: Array<string>;
	errorExportKeys: Array<string>;
} {
	return {
		matchedPatterns: props.matchedPatterns || [],
		importURLs: props.importURLs || [],
		exportKeys: props.exportKeys || [],
		errorExportKeys: props.errorExportKeys || [],
	};
}

export function mergeClientModuleMapWithRouteModuleMetadata(props: {
	currentClientModuleMap: VormaClientGlobal["clientModuleMap"] | undefined;
	routeModuleMetadata: RouteModuleMetadataInput;
}): VormaClientGlobal["clientModuleMap"] {
	const { currentClientModuleMap, routeModuleMetadata } = props;
	const nextClientModuleMap: VormaClientGlobal["clientModuleMap"] = {
		...(currentClientModuleMap || {}),
	};
	const { matchedPatterns, importURLs, exportKeys, errorExportKeys } =
		toRouteModuleMetadataArrays(routeModuleMetadata);

	for (let index = 0; index < matchedPatterns.length; index += 1) {
		const pattern = matchedPatterns[index];
		const importURL = importURLs[index];
		if (!pattern || !importURL) {
			continue;
		}

		nextClientModuleMap[pattern] = {
			importURL,
			exportKey: exportKeys[index] || "default",
			errorExportKey: errorExportKeys[index] || "",
		};
	}

	return nextClientModuleMap;
}

export function buildClientModuleMapFromRouteModuleMetadata(props: {
	routeModuleMetadata: RouteModuleMetadataInput;
}): VormaClientGlobal["clientModuleMap"] {
	return mergeClientModuleMapWithRouteModuleMetadata({
		currentClientModuleMap: undefined,
		routeModuleMetadata: props.routeModuleMetadata,
	});
}

import { jsonDeepEquals } from "vorma/kit/json";
import { getEffectiveErrorData } from "./component_loader_error_data.ts";
import { loadComponentModules } from "./component_loader_imports.ts";
import {
	buildActiveComponents,
	resolveErrorBoundaryComponent,
} from "./component_loader_selection.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export { getEffectiveErrorData } from "./component_loader_error_data.ts";

async function loadComponents(importURLs: string[]): Promise<Map<string, any>> {
	return loadComponentModules(importURLs);
}

async function handleComponents(importURLs: string[]): Promise<void> {
	const modulesMap = await loadComponents(importURLs);
	const originalImportURLs = __vormaClientGlobal.get("importURLs");
	const exportKeys = __vormaClientGlobal.get("exportKeys") ?? [];
	const newActiveComponents = buildActiveComponents({
		importURLs: originalImportURLs,
		exportKeys,
		modulesMap,
	});

	// Only update if components actually changed
	if (
		!jsonDeepEquals(
			newActiveComponents,
			__vormaClientGlobal.get("activeComponents"),
		)
	) {
		__vormaClientGlobal.set("activeComponents", newActiveComponents);
	}
}

async function handleErrorBoundaryComponent(
	importURLs: string[],
): Promise<void> {
	const modulesMap = await loadComponents(importURLs);
	const originalImportURLs = __vormaClientGlobal.get("importURLs");

	// Handle error boundary
	const errorIdx = getEffectiveErrorData().index;

	if (errorIdx != null) {
		const newErrorBoundary = resolveErrorBoundaryComponent({
			errorIdx,
			importURLs: originalImportURLs,
			errorExportKeys: __vormaClientGlobal.get("errorExportKeys"),
			modulesMap,
			defaultErrorBoundary: __vormaClientGlobal.get(
				"defaultErrorBoundary",
			),
		});

		// Only update if changed
		const currentErrorBoundary = __vormaClientGlobal.get(
			"activeErrorBoundary",
		);
		if (currentErrorBoundary !== newErrorBoundary) {
			__vormaClientGlobal.set("activeErrorBoundary", newErrorBoundary);
		}
	}
}

export const ComponentLoader = {
	loadComponents,
	handleComponents,
	handleErrorBoundaryComponent,
};

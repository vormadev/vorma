import { jsonDeepEquals } from "vorma/kit/json";
import { resolvePublicHref } from "../platform/url.ts";
import { __vormaClientGlobal } from "../app/context.ts";

type ModuleExports = Record<string, unknown>;
export type ComponentModulesMap = Map<string, ModuleExports | undefined>;

export function getEffectiveErrorData(): {
	index: number | undefined;
	error: string | undefined;
} {
	const serverErrorIdx = __vormaClientGlobal.get("outermostServerErrorIdx");
	const clientErrorIdx = __vormaClientGlobal.get("outermostClientErrorIdx");
	const serverError = __vormaClientGlobal.get("outermostServerError");
	const clientError = __vormaClientGlobal.get("outermostClientError");
	let errorIdx: number | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		errorIdx = Math.min(serverErrorIdx, clientErrorIdx);
	} else {
		errorIdx = serverErrorIdx ?? clientErrorIdx;
	}

	if (errorIdx == null) {
		return {
			index: undefined,
			error: undefined,
		};
	}

	let error: string | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		if (serverErrorIdx === clientErrorIdx) {
			error = serverError ?? clientError;
		} else {
			error = errorIdx === serverErrorIdx ? serverError : clientError;
		}
	} else {
		error = errorIdx === serverErrorIdx ? serverError : clientError;
	}

	return {
		index: errorIdx,
		error,
	};
}

async function loadComponentModules(
	importURLs: string[] = [],
): Promise<ComponentModulesMap> {
	const dedupedURLs = [...new Set(importURLs)];
	const modules = await Promise.all(
		dedupedURLs.map(async (url) => {
			if (!url) return undefined;
			return import(/* @vite-ignore */ resolvePublicHref(url));
		}),
	);
	return new Map(dedupedURLs.map((url, i) => [url, modules[i]]));
}

function buildActiveComponents(props: {
	importURLs: string[];
	exportKeys: string[];
	modulesMap: ComponentModulesMap;
}): Array<unknown> {
	const { importURLs, exportKeys, modulesMap } = props;
	return importURLs.map((url, i) => {
		const module = modulesMap.get(url);
		const key = exportKeys[i] ?? "default";
		return module?.[key] ?? null;
	});
}

function resolveErrorBoundaryComponent(props: {
	errorIdx: number;
	importURLs: string[];
	errorExportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
	defaultErrorBoundary: unknown;
}): unknown {
	const {
		errorIdx,
		importURLs,
		errorExportKeys,
		modulesMap,
		defaultErrorBoundary,
	} = props;
	const errorModuleURL = importURLs[errorIdx];
	let errorComponent;

	if (errorModuleURL) {
		const errorModule = modulesMap.get(errorModuleURL);
		const errorKey = errorExportKeys ? errorExportKeys[errorIdx] : null;
		if (errorKey && errorModule) {
			errorComponent = errorModule[errorKey];
		}
	}

	return errorComponent ?? defaultErrorBoundary;
}

export async function loadComponents(
	importURLs?: string[],
): Promise<ComponentModulesMap> {
	return loadComponentModules(importURLs);
}

export function setActiveComponentsFromModules(props: {
	importURLs: Array<string> | undefined;
	exportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
}): void {
	const newActiveComponents = buildActiveComponents({
		importURLs: props.importURLs ?? [],
		exportKeys: props.exportKeys ?? [],
		modulesMap: props.modulesMap,
	});

	if (
		!jsonDeepEquals(
			newActiveComponents,
			__vormaClientGlobal.get("activeComponents"),
		)
	) {
		__vormaClientGlobal.set("activeComponents", newActiveComponents);
	}
}

export function setActiveErrorBoundaryFromModules(props: {
	importURLs: Array<string> | undefined;
	errorExportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
}): void {
	const errorIdx = getEffectiveErrorData().index;
	if (errorIdx == null) {
		return;
	}

	const newErrorBoundary = resolveErrorBoundaryComponent({
		errorIdx,
		importURLs: props.importURLs ?? [],
		errorExportKeys: props.errorExportKeys,
		modulesMap: props.modulesMap,
		defaultErrorBoundary: __vormaClientGlobal.get("defaultErrorBoundary"),
	});

	const currentErrorBoundary = __vormaClientGlobal.get("activeErrorBoundary");
	if (currentErrorBoundary !== newErrorBoundary) {
		__vormaClientGlobal.set("activeErrorBoundary", newErrorBoundary);
	}
}

export async function handleComponents(
	importURLs?: string[],
): Promise<ComponentModulesMap> {
	const modulesMap = await loadComponents(importURLs);
	setActiveComponentsFromModules({
		importURLs: __vormaClientGlobal.get("importURLs"),
		exportKeys: __vormaClientGlobal.get("exportKeys"),
		modulesMap,
	});
	return modulesMap;
}

export async function handleErrorBoundaryComponent(
	importURLs?: string[],
	modulesMapOverride?: ComponentModulesMap,
): Promise<void> {
	const modulesMap = modulesMapOverride ?? (await loadComponents(importURLs));
	setActiveErrorBoundaryFromModules({
		importURLs: __vormaClientGlobal.get("importURLs"),
		errorExportKeys: __vormaClientGlobal.get("errorExportKeys"),
		modulesMap,
	});
}

export const ComponentLoader = {
	loadComponents,
	handleComponents,
	handleErrorBoundaryComponent,
};

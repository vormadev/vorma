import { jsonDeepEquals } from "vorma/kit/json";
import {
	__vormaClientGlobal,
	getRuntimeRouteSnapshot,
	updateRuntimeRouteSnapshot,
} from "../app/context.ts";
import { resolvePublicHref } from "../platform/url.ts";

type ModuleExports = Record<string, unknown>;
export type ComponentModulesMap = Map<string, ModuleExports | undefined>;

export function getEffectiveErrorDataFromSnapshot(props: {
	outermostServerErrorIdx: number | undefined;
	outermostClientErrorIdx: number | undefined;
	outermostServerError: string | undefined;
	outermostClientError: string | undefined;
}): {
	index: number | undefined;
	error: string | undefined;
} {
	const {
		outermostServerErrorIdx: serverErrorIdx,
		outermostClientErrorIdx: clientErrorIdx,
		outermostServerError: serverError,
		outermostClientError: clientError,
	} = props;
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

export function getEffectiveErrorData(): {
	index: number | undefined;
	error: string | undefined;
} {
	const snapshot = getRuntimeRouteSnapshot();
	return getEffectiveErrorDataFromSnapshot({
		outermostServerErrorIdx: snapshot.outermostServerErrorIdx,
		outermostClientErrorIdx: snapshot.outermostClientErrorIdx,
		outermostServerError: snapshot.outermostServerError,
		outermostClientError: snapshot.outermostClientError,
	});
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

export function buildActiveComponentsFromModules(props: {
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

export function resolveErrorBoundaryComponentFromModules(props: {
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
	const snapshot = getRuntimeRouteSnapshot();
	const newActiveComponents = buildActiveComponentsFromModules({
		importURLs: props.importURLs ?? [],
		exportKeys: props.exportKeys ?? [],
		modulesMap: props.modulesMap,
	});

	if (!jsonDeepEquals(newActiveComponents, snapshot.activeComponents)) {
		updateRuntimeRouteSnapshot({
			updater: (previousSnapshot) => ({
				...previousSnapshot,
				activeComponents: newActiveComponents,
			}),
		});
	}
}

export function setActiveErrorBoundaryFromModules(props: {
	importURLs: Array<string> | undefined;
	errorExportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
}): void {
	const snapshot = getRuntimeRouteSnapshot();
	const errorIdx = getEffectiveErrorDataFromSnapshot({
		outermostServerErrorIdx: snapshot.outermostServerErrorIdx,
		outermostClientErrorIdx: snapshot.outermostClientErrorIdx,
		outermostServerError: snapshot.outermostServerError,
		outermostClientError: snapshot.outermostClientError,
	}).index;
	if (errorIdx == null) {
		return;
	}

	const newErrorBoundary = resolveErrorBoundaryComponentFromModules({
		errorIdx,
		importURLs: props.importURLs ?? [],
		errorExportKeys: props.errorExportKeys,
		modulesMap: props.modulesMap,
		defaultErrorBoundary: __vormaClientGlobal.get("defaultErrorBoundary"),
	});

	if (snapshot.activeErrorBoundary !== newErrorBoundary) {
		updateRuntimeRouteSnapshot({
			updater: (previousSnapshot) => ({
				...previousSnapshot,
				activeErrorBoundary: newErrorBoundary,
			}),
		});
	}
}

export async function handleComponents(
	importURLs?: string[],
): Promise<ComponentModulesMap> {
	const modulesMap = await loadComponents(importURLs);
	const snapshot = getRuntimeRouteSnapshot();
	setActiveComponentsFromModules({
		importURLs: snapshot.importURLs,
		exportKeys: snapshot.exportKeys,
		modulesMap,
	});
	return modulesMap;
}

export async function handleErrorBoundaryComponent(
	importURLs?: string[],
	modulesMapOverride?: ComponentModulesMap,
): Promise<void> {
	const modulesMap = modulesMapOverride ?? (await loadComponents(importURLs));
	const snapshot = getRuntimeRouteSnapshot();
	setActiveErrorBoundaryFromModules({
		importURLs: snapshot.importURLs,
		errorExportKeys: snapshot.errorExportKeys,
		modulesMap,
	});
}

export const ComponentLoader = {
	loadComponents,
	handleComponents,
	handleErrorBoundaryComponent,
};

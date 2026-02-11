import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { registerPattern } from "vorma/kit/matcher/register";
import { resolvePublicHref } from "../platform/url.ts";
import { isAbortError, logError } from "../platform/safety.ts";
import { dispatchRouteChangeEvent } from "../platform/events.ts";
import {
	hashFragmentFromHref,
	isSameDocumentLocation,
} from "../platform/url.ts";
import { HistoryManager } from "../platform/history.ts";
import type { VormaNavigationType } from "./navigation/types.ts";
import type { ScrollState } from "../platform/scroll.ts";
import { updateHeadEls } from "../ui/head.ts";
import {
	__vormaClientGlobal,
	type ClientLoaderAwaitedServerData,
	type GetRouteDataOutput,
	type VormaClientGlobal,
} from "../app/context.ts";

function preloadModule(url: string): void {
	const href = resolvePublicHref(url);
	if (
		document.querySelector(
			`link[rel="modulepreload"][href="${CSS.escape(href)}"]`,
		)
	) {
		return;
	}

	const link = document.createElement("link");
	link.rel = "modulepreload";
	link.href = href;
	document.head.appendChild(link);
}

function preloadCSS(url: string): Promise<void> {
	const href = resolvePublicHref(url);

	if (
		document.querySelector(
			`link[rel="preload"][href="${CSS.escape(href)}"]`,
		)
	) {
		return Promise.resolve();
	}

	const link = document.createElement("link");
	link.rel = "preload";
	link.setAttribute("as", "style");
	link.href = href;

	document.head.appendChild(link);

	return new Promise((resolve, reject) => {
		link.onload = () => resolve();
		link.onerror = reject;
	});
}

function applyCSS(bundles: string[]): void {
	window.requestAnimationFrame(() => {
		for (const bundle of bundles) {
			if (
				document.querySelector(
					`link[data-vorma-css-bundle="${bundle}"]`,
				)
			) {
				continue;
			}

			const link = document.createElement("link");
			link.rel = "stylesheet";
			link.href = resolvePublicHref(bundle);
			link.setAttribute("data-vorma-css-bundle", bundle);
			document.head.appendChild(link);
		}
	});
}

export const AssetManager = {
	preloadModule,
	preloadCSS,
	applyCSS,
};

function getEffectiveErrorData(): {
	index: number | undefined;
	error: string | undefined;
} {
	const serverErrorIdx = __vormaClientGlobal.get("outermostServerErrorIdx");
	const clientErrorIdx = __vormaClientGlobal.get("outermostClientErrorIdx");
	let errorIdx: number | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		errorIdx = Math.min(serverErrorIdx, clientErrorIdx);
	} else {
		errorIdx = serverErrorIdx ?? clientErrorIdx;
	}
	return {
		index: errorIdx,
		error:
			errorIdx === serverErrorIdx
				? __vormaClientGlobal.get("outermostServerError")
				: errorIdx === clientErrorIdx
					? __vormaClientGlobal.get("outermostClientError")
					: undefined,
	};
}

async function loadComponentModules(
	importURLs: string[],
): Promise<Map<string, any>> {
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
	modulesMap: Map<string, any>;
}): Array<any> {
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
	modulesMap: Map<string, any>;
	defaultErrorBoundary: any;
}): any {
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

export type PartialWaitFnJSON = Pick<
	GetRouteDataOutput,
	| "matchedPatterns"
	| "splatValues"
	| "params"
	| "hasRootData"
	| "loadersData"
	| "importURLs"
>;

export type ClientLoadersResult = {
	data: Array<any>;
	errorMessage?: string;
};

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

	if (patternRequiresServerData(pattern) && loaderData === undefined) {
		return null;
	}

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

type ClientLoaderWorkItems = {
	loaderPromises: Array<Promise<any>>;
	abortControllers: Array<AbortController | null>;
};

function buildClientLoaderWorkItems(props: {
	matchedPatterns: Array<string>;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	outermostServerErrorIdx: number | undefined;
	runningLoaders?: Map<string, Promise<any>>;
	signal: AbortSignal;
	json: PartialWaitFnJSON;
	buildID: string;
}): ClientLoaderWorkItems {
	const {
		matchedPatterns,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		json,
		buildID,
	} = props;

	const loaderPromises: Array<Promise<any>> = [];
	const abortControllers: Array<AbortController | null> = [];

	let i = 0;
	for (const pattern of matchedPatterns) {
		if (
			outermostServerErrorIdx !== undefined &&
			i === outermostServerErrorIdx
		) {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
			i++;
			continue;
		}

		if (runningLoaders?.has(pattern)) {
			loaderPromises.push(runningLoaders.get(pattern)!);
			abortControllers.push(null);
		} else if (patternToWaitFnMap[pattern]) {
			const controller = new AbortController();
			abortControllers.push(controller);

			if (signal.aborted) {
				controller.abort();
			} else {
				signal.addEventListener("abort", () => controller.abort(), {
					once: true,
				});
			}

			const serverData = buildClientLoaderServerData({
				pattern,
				matchedPatterns: json.matchedPatterns ?? [],
				loadersData: json.loadersData ?? [],
				hasRootData: !!json.hasRootData,
				buildID,
			});
			const serverDataPromise = serverData
				? Promise.resolve(serverData)
				: Promise.reject(createUnavailableServerDataError());

			const loaderPromise = patternToWaitFnMap[pattern]({
				params: json.params || {},
				splatValues: json.splatValues || [],
				serverDataPromise,
				signal: controller.signal,
			});
			loaderPromises.push(loaderPromise);
		} else {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
		}
		i++;
	}

	return { loaderPromises, abortControllers };
}

function wrapLoaderPromisesWithChildAbort(props: {
	loaderPromises: Array<Promise<any>>;
	abortControllers: Array<AbortController | null>;
}): Array<Promise<any>> {
	const { loaderPromises, abortControllers } = props;
	return loaderPromises.map(async (promise, index) => {
		return promise.catch((error) => {
			if (!isAbortError(error)) {
				for (let j = index + 1; j < abortControllers.length; j++) {
					abortControllers[j]?.abort();
				}
			}
			throw error;
		});
	});
}

function processSettledClientLoaderResults(props: {
	results: Array<PromiseSettledResult<any>>;
	matchedPatterns: Array<string>;
}): {
	data: Array<any>;
	errorMessage: string | undefined;
} {
	const { results, matchedPatterns } = props;
	const data: Array<any> = [];
	let errorMessage: string | undefined;

	for (let i = 0; i < results.length; i++) {
		const result = results[i];
		if (!result) {
			data.push(undefined);
			continue;
		}

		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			if (!isAbortError(result.reason)) {
				const pattern = matchedPatterns[i];
				logError(
					`Client loader error for pattern ${pattern}:`,
					result.reason,
				);
				errorMessage =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);
			}
			data.push(undefined);
			break;
		}
	}

	return { data, errorMessage };
}

async function executeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
	runningLoaders?: Map<string, Promise<any>>,
): Promise<ClientLoadersResult> {
	await ComponentLoader.loadComponents(json.importURLs);

	const matchedPatterns = json.matchedPatterns ?? [];
	const patternToWaitFnMap = __vormaClientGlobal.get("patternToWaitFnMap");
	const outermostServerErrorIdx = __vormaClientGlobal.get(
		"outermostServerErrorIdx",
	);

	const { loaderPromises, abortControllers } = buildClientLoaderWorkItems({
		matchedPatterns,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		json,
		buildID,
	});

	const wrappedPromises = wrapLoaderPromisesWithChildAbort({
		loaderPromises,
		abortControllers,
	});
	const results = await Promise.allSettled(wrappedPromises);

	const { data, errorMessage } = processSettledClientLoaderResults({
		results,
		matchedPatterns,
	});

	return { data, errorMessage };
}

function buildClientLoaderSnapshotFromGlobal(): PartialWaitFnJSON {
	return {
		hasRootData: __vormaClientGlobal.get("hasRootData"),
		importURLs: __vormaClientGlobal.get("importURLs"),
		loadersData: __vormaClientGlobal.get("loadersData"),
		matchedPatterns: __vormaClientGlobal.get("matchedPatterns"),
		params: __vormaClientGlobal.get("params"),
		splatValues: __vormaClientGlobal.get("splatValues"),
	};
}

export function setClientLoadersState(
	clientLoadersResult: ClientLoadersResult | undefined,
): void {
	if (!clientLoadersResult) {
		return;
	}

	__vormaClientGlobal.set(
		"clientLoadersData",
		clientLoadersResult.data ?? [],
	);
	__vormaClientGlobal.set(
		"outermostClientErrorIdx",
		clientLoadersResult.errorMessage
			? clientLoadersResult.data.length - 1
			: undefined,
	);
	__vormaClientGlobal.set(
		"outermostClientError",
		clientLoadersResult.errorMessage,
	);
}

export function deriveAndSetErrorState(): void {
	const effectiveErrData = getEffectiveErrorData();
	__vormaClientGlobal.set("outermostErrorIdx", effectiveErrData.index);
	__vormaClientGlobal.set("outermostError", effectiveErrData.error);
}

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await executeClientLoaders(
		buildClientLoaderSnapshotFromGlobal(),
		__vormaClientGlobal.get("buildID"),
		new AbortController().signal,
	);

	setClientLoadersState(clientLoadersResult);
	deriveAndSetErrorState();
}

export async function __registerClientLoaderPattern(
	pattern: string,
): Promise<void> {
	registerPattern(__vormaClientGlobal.get("patternRegistry"), pattern);
}

export async function findPartialMatchesOnClient(pathname: string) {
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const patternToWaitFnMap = __vormaClientGlobal.get("patternToWaitFnMap");

	if (Object.keys(patternToWaitFnMap).length === 0) {
		return null;
	}

	const fullResult = findNestedMatches(patternRegistry, pathname);
	if (fullResult) {
		return fullResult;
	}

	const segments = pathname.split("/").filter(Boolean);
	for (let i = segments.length; i >= 0; i--) {
		const partialPath =
			i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
		const result = findNestedMatches(patternRegistry, partialPath);
		if (result) {
			return result;
		}
	}

	return null;
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<any>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

type RenderingHistoryOptions = {
	href: string;
	scrollStateToRestore?: ScrollState;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
};

function runHistoryAndDeriveScrollState(props: {
	navigationType: VormaNavigationType;
	runHistoryOptions?: RenderingHistoryOptions;
}): ScrollState | undefined {
	const { navigationType, runHistoryOptions } = props;
	let scrollStateToDispatch: ScrollState | undefined;

	if (runHistoryOptions) {
		const { href, scrollStateToRestore, replace, scrollToTop } =
			runHistoryOptions;
		const hash = hashFragmentFromHref(href);
		const history = HistoryManager.getInstance();

		if (
			navigationType === "userNavigation" ||
			navigationType === "redirect"
		) {
			const currentHref = window.location.href;
			const isSameLocation = isSameDocumentLocation(href, currentHref);

			if (!isSameLocation && !replace) {
				history.push(href, runHistoryOptions.state);
			} else {
				history.replace(href, runHistoryOptions.state);
			}

			scrollStateToDispatch = hash
				? { hash }
				: scrollToTop !== false
					? { x: 0, y: 0 }
					: undefined;
		}

		if (navigationType === "browserHistory") {
			scrollStateToDispatch =
				scrollStateToRestore ?? (hash ? { hash } : undefined);
		}
	}

	return scrollStateToDispatch;
}

function applyRouteDataToGlobalState(json: GetRouteDataOutput): void {
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

function applyRouteDocumentTitle(title: GetRouteDataOutput["title"]): void {
	if (title === undefined) {
		return;
	}

	const tempTxt = document.createElement("textarea");
	tempTxt.innerHTML = title?.dangerousInnerHTML || "";
	if (document.title !== tempTxt.value) {
		document.title = tempTxt.value;
	}
}

function applyRouteHeadElements(json: GetRouteDataOutput): void {
	if (json.metaHeadEls !== undefined) {
		updateHeadEls("meta", json.metaHeadEls ?? []);
	}
	if (json.restHeadEls !== undefined) {
		updateHeadEls("rest", json.restHeadEls ?? []);
	}
}

type RerenderAppProps = {
	json: GetRouteDataOutput;
	navigationType: VormaNavigationType;
	runHistoryOptions?: RenderingHistoryOptions;
	onFinish: () => void;
};

export async function __reRenderApp(props: RerenderAppProps): Promise<void> {
	const shouldUseViewTransitions =
		__vormaClientGlobal.get("useViewTransitions") &&
		!!document.startViewTransition &&
		props.navigationType !== "prefetch" &&
		props.navigationType !== "revalidation";

	if (shouldUseViewTransitions) {
		const transition = document.startViewTransition(async () => {
			await __reRenderAppInner(props);
		});
		await transition.finished;
	} else {
		await __reRenderAppInner(props);
	}
}

async function __reRenderAppInner(props: RerenderAppProps): Promise<void> {
	const { json, navigationType, runHistoryOptions } = props;

	applyRouteDataToGlobalState(json);
	deriveAndSetErrorState();

	await ComponentLoader.handleComponents(json.importURLs);
	await ComponentLoader.handleErrorBoundaryComponent(json.importURLs);

	const scrollStateToDispatch: ScrollState | undefined =
		runHistoryAndDeriveScrollState({ navigationType, runHistoryOptions });

	applyRouteDocumentTitle(json.title);

	if (json.cssBundles) {
		AssetManager.applyCSS(json.cssBundles);
	}

	dispatchRouteChangeEvent({ __scrollState: scrollStateToDispatch });
	applyRouteHeadElements(json);
	props.onFinish();
}

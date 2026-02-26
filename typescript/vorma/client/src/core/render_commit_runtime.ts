import {
	__vormaClientGlobal,
	getRuntimeRouteSnapshot,
	setRuntimeRouteSnapshot,
	type GetRouteDataOutput,
	type RuntimeRouteSnapshot,
} from "../app/context.ts";
import { dispatchRouteChangeEvent } from "../platform/events.ts";
import { HistoryManager } from "../platform/history.ts";
import type { ScrollState } from "../platform/scroll.ts";
import {
	hashFragmentFromHref,
	isSameDocumentLocation,
	resolvePublicHref,
} from "../platform/url.ts";
import { updateHeadEls } from "../ui/head.ts";
import type { VormaNavigationType } from "./navigation/types.ts";
import type { ClientLoadersResult } from "./render_client_loader_runtime.ts";
import {
	buildActiveComponentsFromModules,
	ComponentLoader,
	getEffectiveErrorDataFromSnapshot,
	resolveErrorBoundaryComponentFromModules,
	type ComponentModulesMap,
} from "./render_component_runtime.ts";

const inFlightCSSPreloadPromiseByHref = new Map<string, Promise<void>>();

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
	const existingInFlightPromise = inFlightCSSPreloadPromiseByHref.get(href);
	if (existingInFlightPromise) {
		return existingInFlightPromise;
	}

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

	const preloadPromise = new Promise<void>((resolve, reject) => {
		link.onload = () => {
			inFlightCSSPreloadPromiseByHref.delete(href);
			resolve();
		};
		link.onerror = (event) => {
			inFlightCSSPreloadPromiseByHref.delete(href);
			reject(event);
		};
	});
	inFlightCSSPreloadPromiseByHref.set(href, preloadPromise);
	document.head.appendChild(link);

	return preloadPromise;
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
			const isSameLocation = isSameDocumentLocation({
				targetHref: href,
				currentHref: currentHref,
			});

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
	clientLoadersResult?: ClientLoadersResult;
	runHistoryOptions?: RenderingHistoryOptions;
	shouldCommit?: () => boolean;
	onFinish: () => void;
};

function canCommitRender(props: {
	shouldCommit: RerenderAppProps["shouldCommit"];
}): boolean {
	if (!props.shouldCommit) {
		return true;
	}
	return props.shouldCommit();
}

function buildSnapshotWithCommittedClientLoaders(props: {
	previousSnapshot: RuntimeRouteSnapshot;
	clientLoadersResult: ClientLoadersResult | undefined;
}): RuntimeRouteSnapshot {
	const { previousSnapshot, clientLoadersResult } = props;
	if (!clientLoadersResult) {
		return previousSnapshot;
	}

	const normalizedClientLoaderData = clientLoadersResult.data ?? [];
	const outermostClientErrorIdx = clientLoadersResult.errorMessage
		? normalizedClientLoaderData.length > 0
			? normalizedClientLoaderData.length - 1
			: undefined
		: undefined;

	return {
		...previousSnapshot,
		clientLoadersData: normalizedClientLoaderData,
		outermostClientErrorIdx,
		outermostClientError: clientLoadersResult.errorMessage,
	};
}

function buildCommittedRuntimeRouteSnapshot(props: {
	previousSnapshot: RuntimeRouteSnapshot;
	json: GetRouteDataOutput;
	modulesMap: ComponentModulesMap;
	clientLoadersResult: ClientLoadersResult | undefined;
}): RuntimeRouteSnapshot {
	const snapshotWithCommittedClientLoaders =
		buildSnapshotWithCommittedClientLoaders({
			previousSnapshot: props.previousSnapshot,
			clientLoadersResult: props.clientLoadersResult,
		});

	const baseCommittedSnapshot: RuntimeRouteSnapshot = {
		...snapshotWithCommittedClientLoaders,
		buildID: __vormaClientGlobal.get("buildID") || "",
		outermostServerError: props.json.outermostServerError,
		outermostServerErrorIdx: props.json.outermostServerErrorIdx,
		errorExportKeys: props.json.errorExportKeys,
		matchedPatterns: props.json.matchedPatterns,
		loadersData: props.json.loadersData,
		importURLs: props.json.importURLs,
		exportKeys: props.json.exportKeys,
		hasRootData: props.json.hasRootData,
		params: props.json.params,
		splatValues: props.json.splatValues,
		activeErrorBoundary: undefined,
		activeComponents: buildActiveComponentsFromModules({
			importURLs: props.json.importURLs ?? [],
			exportKeys: props.json.exportKeys ?? [],
			modulesMap: props.modulesMap,
		}),
	};

	const effectiveErrData = getEffectiveErrorDataFromSnapshot({
		outermostServerErrorIdx: baseCommittedSnapshot.outermostServerErrorIdx,
		outermostClientErrorIdx: baseCommittedSnapshot.outermostClientErrorIdx,
		outermostServerError: baseCommittedSnapshot.outermostServerError,
		outermostClientError: baseCommittedSnapshot.outermostClientError,
	});

	if (effectiveErrData.index == null) {
		return {
			...baseCommittedSnapshot,
			outermostErrorIdx: undefined,
			outermostError: undefined,
			activeErrorBoundary: undefined,
		};
	}

	return {
		...baseCommittedSnapshot,
		outermostErrorIdx: effectiveErrData.index,
		outermostError: effectiveErrData.error,
		activeErrorBoundary: resolveErrorBoundaryComponentFromModules({
			errorIdx: effectiveErrData.index,
			importURLs: props.json.importURLs ?? [],
			errorExportKeys: props.json.errorExportKeys,
			modulesMap: props.modulesMap,
			defaultErrorBoundary: __vormaClientGlobal.get(
				"defaultErrorBoundary",
			),
		}),
	};
}

function executeRenderCommitPipeline(props: {
	json: GetRouteDataOutput;
	navigationType: VormaNavigationType;
	clientLoadersResult: ClientLoadersResult | undefined;
	runHistoryOptions?: RenderingHistoryOptions;
	modulesMap: ComponentModulesMap;
	onFinish: () => void;
}): void {
	const previousSnapshot = getRuntimeRouteSnapshot();
	const committedSnapshot = buildCommittedRuntimeRouteSnapshot({
		previousSnapshot,
		json: props.json,
		modulesMap: props.modulesMap,
		clientLoadersResult: props.clientLoadersResult,
	});
	setRuntimeRouteSnapshot(committedSnapshot);
	const scrollStateToDispatch = runHistoryAndDeriveScrollState({
		navigationType: props.navigationType,
		runHistoryOptions: props.runHistoryOptions,
	});
	applyRouteDocumentTitle(props.json.title);
	if (props.json.cssBundles) {
		AssetManager.applyCSS(props.json.cssBundles);
	}
	dispatchRouteChangeEvent({
		__scrollState: scrollStateToDispatch,
	});
	applyRouteHeadElements(props.json);
	props.onFinish();
}

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
	const {
		json,
		navigationType,
		runHistoryOptions,
		shouldCommit,
		clientLoadersResult,
	} = props;

	if (
		!canCommitRender({
			shouldCommit,
		})
	) {
		return;
	}

	const modulesMap = await ComponentLoader.loadComponents(json.importURLs);

	if (
		!canCommitRender({
			shouldCommit,
		})
	) {
		return;
	}

	executeRenderCommitPipeline({
		json,
		navigationType,
		clientLoadersResult,
		runHistoryOptions,
		modulesMap,
		onFinish: props.onFinish,
	});
}

import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
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
import { deriveAndSetErrorState } from "./render_client_loader_runtime.ts";
import {
	ComponentLoader,
	type ComponentModulesMap,
	setActiveComponentsFromModules,
	setActiveErrorBoundaryFromModules,
} from "./render_component_runtime.ts";

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

function executeRenderCommitPipeline(props: {
	json: GetRouteDataOutput;
	navigationType: VormaNavigationType;
	runHistoryOptions?: RenderingHistoryOptions;
	modulesMap: ComponentModulesMap;
	onFinish: () => void;
}): void {
	applyRouteDataToGlobalState(props.json);
	deriveAndSetErrorState();
	setActiveComponentsFromModules({
		importURLs: props.json.importURLs,
		exportKeys: props.json.exportKeys,
		modulesMap: props.modulesMap,
	});
	setActiveErrorBoundaryFromModules({
		importURLs: props.json.importURLs,
		errorExportKeys: props.json.errorExportKeys,
		modulesMap: props.modulesMap,
	});
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
	const { json, navigationType, runHistoryOptions, shouldCommit } = props;

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
		runHistoryOptions,
		modulesMap,
		onFinish: props.onFinish,
	});
}

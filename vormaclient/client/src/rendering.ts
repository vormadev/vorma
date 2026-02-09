import { AssetManager } from "./asset_manager.ts";
import type { VormaNavigationType } from "./client.ts";
import { deriveAndSetErrorState } from "./client_loaders.ts";
import { ComponentLoader } from "./component_loader.ts";
import { dispatchRouteChangeEvent } from "./events.ts";
import { updateHeadEls } from "./head_elements/head_elements.ts";
import { HistoryManager } from "./history/history.ts";
import type { ScrollState } from "./scroll_state_manager.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

type RerenderAppProps = {
	json: GetRouteDataOutput;
	navigationType: VormaNavigationType;
	runHistoryOptions?: {
		href: string;
		scrollStateToRestore?: ScrollState;
		replace?: boolean;
		scrollToTop?: boolean;
		state?: unknown;
	};
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

	// Update global state
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

	deriveAndSetErrorState();

	// Load components and error boundary
	await ComponentLoader.handleComponents(json.importURLs);
	await ComponentLoader.handleErrorBoundaryComponent(json.importURLs);

	// Handle history and scroll
	let scrollStateToDispatch: ScrollState | undefined;

	if (runHistoryOptions) {
		const { href, scrollStateToRestore, replace, scrollToTop } =
			runHistoryOptions;
		const hash = href.split("#")[1];
		const history = HistoryManager.getInstance();

		if (
			navigationType === "userNavigation" ||
			navigationType === "redirect"
		) {
			const target = new URL(href, window.location.href).href;
			const current = new URL(window.location.href).href;

			if (target !== current && !replace) {
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

	if (json.title !== undefined) {
		// Changing the title instantly makes it feel faster
		// The temp textarea trick is to decode any HTML entities in the title.
		// This should come after pushing to history though, so that the title is
		// correct in the history entry.
		const tempTxt = document.createElement("textarea");
		tempTxt.innerHTML = json.title?.dangerousInnerHTML || "";
		if (document.title !== tempTxt.value) {
			document.title = tempTxt.value;
		}
	}

	// Apply CSS
	if (json.cssBundles) {
		AssetManager.applyCSS(json.cssBundles);
	}

	// Dispatch route change event -- this triggers the actual UI update
	dispatchRouteChangeEvent({ __scrollState: scrollStateToDispatch });

	// Only update head elements if provided (not undefined)
	if (json.metaHeadEls !== undefined) {
		updateHeadEls("meta", json.metaHeadEls ?? []);
	}
	if (json.restHeadEls !== undefined) {
		updateHeadEls("rest", json.restHeadEls ?? []);
	}

	props.onFinish();
}

import { AssetManager } from "./asset_manager.ts";
import type { VormaNavigationType } from "./navigation_runtime/types.ts";
import { deriveAndSetErrorState } from "./client_loaders.ts";
import { ComponentLoader } from "./component_loader.ts";
import { dispatchRouteChangeEvent } from "./events.ts";
import {
	applyRouteDocumentTitle,
	applyRouteHeadElements,
} from "./rendering_document_updates.ts";
import type { ScrollState } from "./scroll_state_manager.ts";
import {
	runHistoryAndDeriveScrollState,
	type RenderingHistoryOptions,
} from "./rendering_history_scroll.ts";
import { applyRouteDataToGlobalState } from "./rendering_state_apply.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

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

	// Update global state
	applyRouteDataToGlobalState(json);

	deriveAndSetErrorState();

	// Load components and error boundary
	await ComponentLoader.handleComponents(json.importURLs);
	await ComponentLoader.handleErrorBoundaryComponent(json.importURLs);

	// Handle history and scroll
	const scrollStateToDispatch: ScrollState | undefined =
		runHistoryAndDeriveScrollState({ navigationType, runHistoryOptions });

	applyRouteDocumentTitle(json.title);

	// Apply CSS
	if (json.cssBundles) {
		AssetManager.applyCSS(json.cssBundles);
	}

	// Dispatch route change event -- this triggers the actual UI update
	dispatchRouteChangeEvent({ __scrollState: scrollStateToDispatch });

	// Only update head elements if provided (not undefined)
	applyRouteHeadElements(json);

	props.onFinish();
}

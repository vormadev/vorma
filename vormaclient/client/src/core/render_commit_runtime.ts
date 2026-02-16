import type { ScrollState } from "../platform/scroll.ts";
import { dispatchRouteChangeEvent } from "../platform/events.ts";
import {
	hashFragmentFromHref,
	isSameDocumentLocation,
} from "../platform/url.ts";
import { HistoryManager } from "../platform/history.ts";
import { updateHeadEls } from "../ui/head.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../app/context.ts";
import type { VormaNavigationType } from "./navigation/types.ts";
import {
	buildRenderCommitCommands,
	type RenderCommitCommand,
} from "./render_runtime_commit_commands.ts";
import {
	decideRenderCommitCheckpointExecutionPlan,
	type RenderCommitCheckpoint,
} from "./render_runtime_commit_state_machine.ts";
import { AssetManager } from "./render_asset_runtime.ts";
import {
	ComponentLoader,
	type ComponentModulesMap,
	setActiveComponentsFromModules,
	setActiveErrorBoundaryFromModules,
} from "./render_component_runtime.ts";
import { deriveAndSetErrorState } from "./render_client_loader_runtime.ts";

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

function shouldContinueRenderCommitAtCheckpoint(props: {
	checkpoint: RenderCommitCheckpoint;
	shouldCommit: RerenderAppProps["shouldCommit"];
}): boolean {
	const checkpointExecutionPlan = decideRenderCommitCheckpointExecutionPlan({
		checkpoint: props.checkpoint,
		shouldCommitRender: canCommitRender({
			shouldCommit: props.shouldCommit,
		}),
	});

	return checkpointExecutionPlan.type === "continue";
}

function executeRenderCommitCommands(props: {
	commands: RenderCommitCommand[];
	json: GetRouteDataOutput;
	navigationType: VormaNavigationType;
	runHistoryOptions?: RenderingHistoryOptions;
	modulesMap: ComponentModulesMap;
	onFinish: () => void;
}): void {
	let scrollStateToDispatch: ScrollState | undefined;

	for (const command of props.commands) {
		switch (command.type) {
			case "apply_route_data_to_global_state":
				applyRouteDataToGlobalState(props.json);
				break;
			case "derive_and_set_error_state":
				deriveAndSetErrorState();
				break;
			case "set_active_components_from_modules":
				setActiveComponentsFromModules({
					importURLs: props.json.importURLs,
					exportKeys: props.json.exportKeys,
					modulesMap: props.modulesMap,
				});
				break;
			case "set_active_error_boundary_from_modules":
				setActiveErrorBoundaryFromModules({
					importURLs: props.json.importURLs,
					errorExportKeys: props.json.errorExportKeys,
					modulesMap: props.modulesMap,
				});
				break;
			case "run_history_and_capture_scroll_state":
				scrollStateToDispatch = runHistoryAndDeriveScrollState({
					navigationType: props.navigationType,
					runHistoryOptions: props.runHistoryOptions,
				});
				break;
			case "apply_route_document_title":
				applyRouteDocumentTitle(props.json.title);
				break;
			case "apply_css_bundles":
				AssetManager.applyCSS(command.cssBundles);
				break;
			case "dispatch_route_change_event":
				dispatchRouteChangeEvent({
					__scrollState: scrollStateToDispatch,
				});
				break;
			case "apply_route_head_elements":
				applyRouteHeadElements(props.json);
				break;
			case "finish":
				props.onFinish();
				return;
		}
	}

	throw new Error(
		"Render commit command plan ended without a finish command.",
	);
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
		!shouldContinueRenderCommitAtCheckpoint({
			checkpoint: "pre_module_load",
			shouldCommit,
		})
	) {
		return;
	}

	const modulesMap = await ComponentLoader.loadComponents(json.importURLs);

	if (
		!shouldContinueRenderCommitAtCheckpoint({
			checkpoint: "post_module_load",
			shouldCommit,
		})
	) {
		return;
	}

	executeRenderCommitCommands({
		commands: buildRenderCommitCommands({
			json,
		}),
		json,
		navigationType,
		runHistoryOptions,
		modulesMap,
		onFinish: props.onFinish,
	});
}

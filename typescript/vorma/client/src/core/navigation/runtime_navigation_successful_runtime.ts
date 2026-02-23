import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
	type VormaClientGlobal,
} from "../../app/context.ts";
import { dispatchBuildIDEvent } from "../../platform/events.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import { getBuildIDFromResponse } from "../redirects.ts";
import {
	AssetManager,
	__reRenderApp,
	setClientLoadersState,
} from "../render_runtime.ts";
import {
	decideBuildIDSyncTimingForSuccessfulEntry,
	decideSuccessfulNavigationCleanupExecutionPlan,
	decideSuccessfulNavigationPostAssetExecutionPlan,
	decideSuccessfulNavigationPostAssetSideEffectPlan,
	decideSuccessfulNavigationPostWaitingExecutionPlan,
	decideSuccessfulNavigationPreAssetWaitExecutionPlan,
	decideSuccessfulNavigationPreWaitingExecutionPlan,
	type BuildIDSyncTiming,
} from "./runtime_navigation_outcome_state_machine.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";
import { hasNavigationOperationOwnership } from "./types.ts";

const successfulNavigationPhaseReason = {
	waiting: "process_successful_navigation_waiting",
	rendering: "process_successful_navigation_rendering",
	complete: "process_successful_navigation_complete",
} as const;

export type RouteModuleMetadataInput = {
	matchedPatterns?: Array<string>;
	importURLs?: Array<string>;
	exportKeys?: Array<string>;
	errorExportKeys?: Array<string>;
};

export function mergeClientModuleMapWithRouteModuleMetadata(props: {
	currentClientModuleMap: VormaClientGlobal["clientModuleMap"] | undefined;
	routeModuleMetadata: RouteModuleMetadataInput;
}): VormaClientGlobal["clientModuleMap"] {
	const { currentClientModuleMap, routeModuleMetadata } = props;
	const nextClientModuleMap: VormaClientGlobal["clientModuleMap"] = {
		...currentClientModuleMap,
	};
	const matchedPatterns = routeModuleMetadata.matchedPatterns ?? [];
	const importURLs = routeModuleMetadata.importURLs ?? [];
	const exportKeys = routeModuleMetadata.exportKeys ?? [];
	const errorExportKeys = routeModuleMetadata.errorExportKeys ?? [];

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

export type ProcessSuccessfulNavigationContext = {
	transitionPhase: (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	onSuccessfulNavigationCommitted?: (props: {
		entry: NavigationEntry;
		outcome: Extract<NavigationOutcome, { type: "success" }>;
	}) => void;
};

function isCurrentNavigationEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): boolean {
	const { context, entry } = props;
	return hasNavigationOperationOwnership({
		entry: context.findNavigationEntry(entry.targetUrl),
		expectedOperationID: entry.operationID,
	});
}

function transitionPhaseForCurrentEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
	phase: NavigationPhase;
	reason: string;
}): void {
	const { context, entry, phase, reason } = props;
	if (!isCurrentNavigationEntry({ context, entry })) {
		return;
	}

	context.transitionPhase({
		targetUrl: entry.targetUrl,
		phase,
		reason,
	});
}

function applyResponseArtifactsWhenBuildMatches(props: {
	response: Response;
	json: GetRouteDataOutput;
	expectedBuildID: string;
}): void {
	const { response, json, expectedBuildID } = props;
	const responseBuildID = getBuildIDFromResponse(response);

	if (responseBuildID !== expectedBuildID) {
		return;
	}

	const clientModuleMap = mergeClientModuleMapWithRouteModuleMetadata({
		currentClientModuleMap: __vormaClientGlobal.get("clientModuleMap"),
		routeModuleMetadata: json,
	});

	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);
}

export function syncBuildIDFromResponse(response: Response): void {
	const oldID = __vormaClientGlobal.get("buildID");
	const newID = getBuildIDFromResponse(response);
	if (!newID || newID === oldID) {
		return;
	}

	__vormaClientGlobal.set("buildID", newID);
	dispatchBuildIDEvent({ newID, oldID });
}

async function waitForSuccessfulNavigationAssets(
	outcome: Extract<NavigationOutcome, { type: "success" }>,
): Promise<
	| Awaited<Extract<NavigationOutcome, { type: "success" }>["waitFnPromise"]>
	| undefined
> {
	const { waitFnPromise, preloadPlan } = outcome;
	const cssBundlePromises: Array<Promise<unknown>> = [];
	for (const dependency of preloadPlan.moduleDependencies) {
		if (dependency) {
			AssetManager.preloadModule(dependency);
		}
	}
	for (const cssBundle of preloadPlan.cssBundles) {
		cssBundlePromises.push(AssetManager.preloadCSS(cssBundle));
	}

	const clientLoadersResult = await waitFnPromise;

	if (cssBundlePromises.length > 0) {
		try {
			await Promise.all(cssBundlePromises);
		} catch (error) {
			logError("Error preloading CSS bundles:", error);
		}
	}

	return clientLoadersResult;
}

function buildRunHistoryOptions(
	entry: NavigationEntry,
	props: Extract<NavigationOutcome, { type: "success" }>["props"],
) {
	if (entry.intent !== "navigate") {
		return undefined;
	}

	return {
		href: entry.targetUrl,
		scrollStateToRestore: props.scrollStateToRestore,
		replace: entry.replace || props.replace,
		scrollToTop: entry.scrollToTop,
		state: entry.state,
	};
}

async function renderSuccessfulNavigation(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	transitionPhaseForCurrentEntry({
		context,
		entry,
		phase: "rendering",
		reason: successfulNavigationPhaseReason.rendering,
	});

	try {
		await __reRenderApp({
			json: outcome.json,
			navigationType: entry.type,
			runHistoryOptions: buildRunHistoryOptions(entry, outcome.props),
			shouldCommit: () =>
				isCurrentNavigationEntry({
					context,
					entry,
				}),
			onFinish: () => {
				transitionPhaseForCurrentEntry({
					context,
					entry,
					phase: "complete",
					reason: successfulNavigationPhaseReason.complete,
				});
			},
		});
	} catch (error) {
		transitionPhaseForCurrentEntry({
			context,
			entry,
			phase: "complete",
			reason: successfulNavigationPhaseReason.complete,
		});
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}

type SuccessfulNavigationOutcome = Extract<
	NavigationOutcome,
	{ type: "success" }
>;
type SuccessfulNavigationClientLoadersResult =
	| Awaited<SuccessfulNavigationOutcome["waitFnPromise"]>
	| undefined;

type SuccessfulNavigationLifecycleCheckpointExecutionInputBase = {
	context: ProcessSuccessfulNavigationContext;
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
};

type SuccessfulNavigationLifecycleCheckpointExecutionInput =
	| (SuccessfulNavigationLifecycleCheckpointExecutionInputBase & {
			checkpoint: "pre_waiting";
	  })
	| (SuccessfulNavigationLifecycleCheckpointExecutionInputBase & {
			checkpoint: "post_waiting";
	  })
	| (SuccessfulNavigationLifecycleCheckpointExecutionInputBase & {
			checkpoint: "pre_asset_wait";
			buildIDSyncTiming: BuildIDSyncTiming;
	  })
	| (SuccessfulNavigationLifecycleCheckpointExecutionInputBase & {
			checkpoint: "post_asset";
			buildIDSyncTiming: BuildIDSyncTiming;
			expectedBuildID: string;
			clientLoadersResult: SuccessfulNavigationClientLoadersResult;
	  })
	| (SuccessfulNavigationLifecycleCheckpointExecutionInputBase & {
			checkpoint: "cleanup";
	  });

async function executeSuccessfulNavigationLifecycleCheckpoint(
	props: SuccessfulNavigationLifecycleCheckpointExecutionInput,
): Promise<boolean> {
	const { context, outcome, entry } = props;
	const isCurrentEntry = isCurrentNavigationEntry({ context, entry });
	const currentHref = window.location.href;

	switch (props.checkpoint) {
		case "pre_waiting": {
			const preWaitingExecutionPlan =
				decideSuccessfulNavigationPreWaitingExecutionPlan({
					entry,
					isCurrentEntry,
					currentHref,
				});
			switch (preWaitingExecutionPlan.type) {
				case "stop":
					return true;
				case "deleteAndStop":
					context.deleteNavigation({
						targetUrl: preWaitingExecutionPlan.targetUrl,
						reason: preWaitingExecutionPlan.reason,
					});
					return true;
				case "continue":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "waiting",
						reason: successfulNavigationPhaseReason.waiting,
					});
					return false;
			}
		}
		case "post_waiting": {
			const postWaitingExecutionPlan =
				decideSuccessfulNavigationPostWaitingExecutionPlan({
					isCurrentEntry,
				});
			return postWaitingExecutionPlan.type === "stop";
		}
		case "pre_asset_wait": {
			const preAssetWaitExecutionPlan =
				decideSuccessfulNavigationPreAssetWaitExecutionPlan({
					buildIDSyncTiming: props.buildIDSyncTiming,
				});
			if (preAssetWaitExecutionPlan.shouldSyncBuildIDBeforeAssetWait) {
				syncBuildIDFromResponse(outcome.response);
			}
			return false;
		}
		case "post_asset": {
			const postAssetExecutionPlan =
				decideSuccessfulNavigationPostAssetExecutionPlan({
					entry,
					isCurrentEntry,
					currentHref,
				});
			const postAssetSideEffectPlan =
				decideSuccessfulNavigationPostAssetSideEffectPlan({
					postAssetExecutionPlan,
					buildIDSyncTiming: props.buildIDSyncTiming,
				});

			if (postAssetSideEffectPlan.shouldCommitClientLoadersState) {
				setClientLoadersState(props.clientLoadersResult);
			}
			if (postAssetSideEffectPlan.shouldSyncBuildIDAfterAssetWait) {
				syncBuildIDFromResponse(outcome.response);
			}
			if (postAssetSideEffectPlan.shouldApplyResponseArtifacts) {
				applyResponseArtifactsWhenBuildMatches({
					response: outcome.response,
					json: outcome.json,
					expectedBuildID: props.expectedBuildID,
				});
			}

			switch (postAssetExecutionPlan.type) {
				case "stop":
					return true;
				case "completeWithoutRender":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "complete",
						reason: successfulNavigationPhaseReason.complete,
					});
					return false;
				case "render":
					await renderSuccessfulNavigation(context, outcome, entry);
					return false;
			}
		}
		case "cleanup": {
			const cleanupExecutionPlan =
				decideSuccessfulNavigationCleanupExecutionPlan({
					entry,
					isCurrentEntry,
				});
			if (cleanupExecutionPlan.type === "skip") {
				return false;
			}
			context.deleteNavigation({
				targetUrl: cleanupExecutionPlan.targetUrl,
				reason: cleanupExecutionPlan.reason,
			});
			return false;
		}
	}
}

export async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
): Promise<void> {
	let didCommitSuccessfulNavigation = false;

	try {
		const expectedBuildIDForResponseArtifacts =
			__vormaClientGlobal.get("buildID");

		for (const checkpoint of ["pre_waiting", "post_waiting"] as const) {
			const shouldStop =
				await executeSuccessfulNavigationLifecycleCheckpoint({
					checkpoint,
					context,
					outcome,
					entry,
				});
			if (shouldStop) {
				return;
			}
		}

		const buildIDSyncTiming = decideBuildIDSyncTimingForSuccessfulEntry({
			entry,
		});
		const shouldStopAfterPreAssetWait =
			await executeSuccessfulNavigationLifecycleCheckpoint({
				checkpoint: "pre_asset_wait",
				context,
				outcome,
				entry,
				buildIDSyncTiming,
			});
		if (shouldStopAfterPreAssetWait) {
			return;
		}

		const clientLoadersResult =
			await waitForSuccessfulNavigationAssets(outcome);

		const shouldStopAfterPostAsset =
			await executeSuccessfulNavigationLifecycleCheckpoint({
				checkpoint: "post_asset",
				context,
				outcome,
				entry,
				buildIDSyncTiming,
				expectedBuildID: expectedBuildIDForResponseArtifacts,
				clientLoadersResult,
			});
		if (shouldStopAfterPostAsset) {
			return;
		}

		didCommitSuccessfulNavigation = true;
	} finally {
		await executeSuccessfulNavigationLifecycleCheckpoint({
			checkpoint: "cleanup",
			context,
			outcome,
			entry,
		});
	}

	if (didCommitSuccessfulNavigation) {
		context.onSuccessfulNavigationCommitted?.({
			entry,
			outcome,
		});
	}
}

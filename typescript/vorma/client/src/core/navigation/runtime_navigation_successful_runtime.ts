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
	decideSuccessfulNavigationLifecycleCheckpointExecutionPlan,
	type BuildIDSyncTiming,
	type SuccessfulNavigationLifecycleCheckpoint,
	type SuccessfulNavigationLifecycleCheckpointExecutionPlan,
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

function toRouteModuleMetadataArrays(props: RouteModuleMetadataInput): {
	matchedPatterns: Array<string>;
	importURLs: Array<string>;
	exportKeys: Array<string>;
	errorExportKeys: Array<string>;
} {
	return {
		matchedPatterns: props.matchedPatterns || [],
		importURLs: props.importURLs || [],
		exportKeys: props.exportKeys || [],
		errorExportKeys: props.errorExportKeys || [],
	};
}

export function mergeClientModuleMapWithRouteModuleMetadata(props: {
	currentClientModuleMap: VormaClientGlobal["clientModuleMap"] | undefined;
	routeModuleMetadata: RouteModuleMetadataInput;
}): VormaClientGlobal["clientModuleMap"] {
	const { currentClientModuleMap, routeModuleMetadata } = props;
	const nextClientModuleMap: VormaClientGlobal["clientModuleMap"] = {
		...currentClientModuleMap,
	};
	const { matchedPatterns, importURLs, exportKeys, errorExportKeys } =
		toRouteModuleMetadataArrays(routeModuleMetadata);

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

export function buildClientModuleMapFromRouteModuleMetadata(props: {
	routeModuleMetadata: RouteModuleMetadataInput;
}): VormaClientGlobal["clientModuleMap"] {
	return mergeClientModuleMapWithRouteModuleMetadata({
		currentClientModuleMap: undefined,
		routeModuleMetadata: props.routeModuleMetadata,
	});
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
): Promise<{
	clientLoadersResult:
		| Awaited<
				Extract<NavigationOutcome, { type: "success" }>["waitFnPromise"]
		  >
		| undefined;
}> {
	const { waitFnPromise, preloadCommands } = outcome;
	const cssBundlePromises: Array<Promise<unknown>> = [];
	for (const preloadCommand of preloadCommands) {
		switch (preloadCommand.type) {
			case "preload_module_dependency":
				if (preloadCommand.dependency) {
					AssetManager.preloadModule(preloadCommand.dependency);
				}
				break;
			case "preload_css_bundle":
				cssBundlePromises.push(
					AssetManager.preloadCSS(preloadCommand.bundle),
				);
				break;
		}
	}

	const clientLoadersResult = await waitFnPromise;

	if (cssBundlePromises.length > 0) {
		try {
			await Promise.all(cssBundlePromises);
		} catch (error) {
			logError("Error preloading CSS bundles:", error);
		}
	}

	return {
		clientLoadersResult,
	};
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

type DecideAndExecuteSuccessfulNavigationLifecycleCheckpointProps = {
	checkpoint: SuccessfulNavigationLifecycleCheckpoint;
	context: ProcessSuccessfulNavigationContext;
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
	buildIDSyncTiming?: BuildIDSyncTiming;
	expectedBuildID?: string;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
};

function requireSuccessfulNavigationRuntimeCheckpointInputValue<T>(props: {
	value: T | undefined;
	checkpoint: SuccessfulNavigationLifecycleCheckpoint;
	field: string;
}): T {
	if (props.value === undefined) {
		throw new Error(
			`Missing '${props.field}' for successful navigation runtime checkpoint '${props.checkpoint}'.`,
		);
	}

	return props.value;
}

async function executeSuccessfulNavigationLifecycleCheckpointExecutionPlan(props: {
	checkpointExecutionPlan: SuccessfulNavigationLifecycleCheckpointExecutionPlan;
	context: ProcessSuccessfulNavigationContext;
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
	expectedBuildID?: string;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
}): Promise<{ shouldStop: boolean }> {
	const { checkpointExecutionPlan, context, outcome, entry } = props;
	switch (checkpointExecutionPlan.checkpoint) {
		case "pre_waiting":
			switch (checkpointExecutionPlan.preWaitingExecutionPlan.type) {
				case "stop":
					return { shouldStop: true };
				case "deleteAndStop":
					context.deleteNavigation({
						targetUrl:
							checkpointExecutionPlan.preWaitingExecutionPlan
								.targetUrl,
						reason: checkpointExecutionPlan.preWaitingExecutionPlan
							.reason,
					});
					return { shouldStop: true };
				case "continue":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "waiting",
						reason: successfulNavigationPhaseReason.waiting,
					});
					return { shouldStop: false };
			}
		case "post_waiting":
			return {
				shouldStop:
					checkpointExecutionPlan.postWaitingExecutionPlan.type ===
					"stop",
			};
		case "pre_asset_wait":
			if (
				checkpointExecutionPlan.preAssetWaitExecutionPlan
					.shouldSyncBuildIDBeforeAssetWait
			) {
				syncBuildIDFromResponse(outcome.response);
			}
			return { shouldStop: false };
		case "post_asset": {
			const { postAssetExecutionPlan, postAssetSideEffectPlan } =
				checkpointExecutionPlan.postAssetLifecycleExecutionPlan;

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
					expectedBuildID:
						requireSuccessfulNavigationRuntimeCheckpointInputValue({
							value: props.expectedBuildID,
							checkpoint: checkpointExecutionPlan.checkpoint,
							field: "expectedBuildID",
						}),
				});
			}

			switch (postAssetExecutionPlan.type) {
				case "stop":
					return { shouldStop: true };
				case "completeWithoutRender":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "complete",
						reason: successfulNavigationPhaseReason.complete,
					});
					return { shouldStop: false };
				case "render":
					await renderSuccessfulNavigation(context, outcome, entry);
					return { shouldStop: false };
			}
		}
		case "cleanup":
			if (checkpointExecutionPlan.cleanupExecutionPlan.type === "skip") {
				return { shouldStop: false };
			}
			context.deleteNavigation({
				targetUrl:
					checkpointExecutionPlan.cleanupExecutionPlan.targetUrl,
				reason: checkpointExecutionPlan.cleanupExecutionPlan.reason,
			});
			return { shouldStop: false };
	}
}

async function decideAndExecuteSuccessfulNavigationLifecycleCheckpoint(
	props: DecideAndExecuteSuccessfulNavigationLifecycleCheckpointProps,
): Promise<{
	shouldStop: boolean;
}> {
	const { context, outcome, entry } = props;
	const checkpointExecutionPlan =
		decideSuccessfulNavigationLifecycleCheckpointExecutionPlan({
			checkpoint: props.checkpoint,
			entry,
			isCurrentEntry: isCurrentNavigationEntry({
				context,
				entry,
			}),
			currentHref: window.location.href,
			buildIDSyncTiming: props.buildIDSyncTiming,
		});
	return executeSuccessfulNavigationLifecycleCheckpointExecutionPlan({
		checkpointExecutionPlan,
		context,
		outcome,
		entry,
		expectedBuildID: props.expectedBuildID,
		clientLoadersResult: props.clientLoadersResult,
	});
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
			const checkpointResult =
				await decideAndExecuteSuccessfulNavigationLifecycleCheckpoint({
					checkpoint,
					context,
					outcome,
					entry,
				});
			if (checkpointResult.shouldStop) {
				return;
			}
		}

		const buildIDSyncTiming = decideBuildIDSyncTimingForSuccessfulEntry({
			entry,
		});
		const preAssetWaitCommandExecutionResult =
			await decideAndExecuteSuccessfulNavigationLifecycleCheckpoint({
				checkpoint: "pre_asset_wait",
				context,
				outcome,
				entry,
				buildIDSyncTiming,
			});
		if (preAssetWaitCommandExecutionResult.shouldStop) {
			return;
		}

		const assetWaitResult =
			await waitForSuccessfulNavigationAssets(outcome);

		if (
			(
				await decideAndExecuteSuccessfulNavigationLifecycleCheckpoint({
					checkpoint: "post_asset",
					context,
					outcome,
					entry,
					buildIDSyncTiming,
					expectedBuildID: expectedBuildIDForResponseArtifacts,
					clientLoadersResult: assetWaitResult.clientLoadersResult,
				})
			).shouldStop
		) {
			return;
		}

		didCommitSuccessfulNavigation = true;
	} finally {
		await decideAndExecuteSuccessfulNavigationLifecycleCheckpoint({
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

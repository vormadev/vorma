import { dispatchBuildIDEvent } from "../../platform/events.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
	type VormaClientGlobal,
} from "../../app/context.ts";
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
} from "./runtime_navigation_outcome_state_machine.ts";
import {
	buildSuccessfulNavigationLifecycleCheckpointCommands,
	type SuccessfulNavigationLifecycleCommand,
} from "./runtime_navigation_successful_commands.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";
import { hasNavigationOperationOwnership } from "./types.ts";

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
	shouldApplyCSSBundles: boolean;
}): void {
	const { response, json, expectedBuildID, shouldApplyCSSBundles } = props;
	const responseBuildID = getBuildIDFromResponse(response);

	if (responseBuildID !== expectedBuildID) {
		return;
	}

	const clientModuleMap = mergeClientModuleMapWithRouteModuleMetadata({
		currentClientModuleMap: __vormaClientGlobal.get("clientModuleMap"),
		routeModuleMetadata: json,
	});

	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);

	if (
		shouldApplyCSSBundles &&
		json.cssBundles &&
		json.cssBundles.length > 0
	) {
		AssetManager.applyCSS(json.cssBundles);
	}
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
		reason: "process_successful_navigation_rendering",
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
					reason: "process_successful_navigation_complete",
				});
			},
		});
	} catch (error) {
		transitionPhaseForCurrentEntry({
			context,
			entry,
			phase: "complete",
			reason: "process_successful_navigation_complete",
		});
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}

async function executeSuccessfulNavigationLifecycleCommands(props: {
	commands: SuccessfulNavigationLifecycleCommand[];
	context: ProcessSuccessfulNavigationContext;
	outcome: Extract<NavigationOutcome, { type: "success" }>;
	entry: NavigationEntry;
}): Promise<{ shouldStop: boolean }> {
	const { commands, context, outcome, entry } = props;
	for (const command of commands) {
		switch (command.type) {
			case "stop":
				return { shouldStop: true };
			case "delete_navigation":
				context.deleteNavigation({
					targetUrl: command.targetUrl,
					reason: command.reason,
				});
				break;
			case "transition_phase":
				transitionPhaseForCurrentEntry({
					context,
					entry,
					phase: command.phase,
					reason: command.reason,
				});
				break;
			case "complete_without_render":
				transitionPhaseForCurrentEntry({
					context,
					entry,
					phase: "complete",
					reason: "process_successful_navigation_complete",
				});
				break;
			case "render":
				await renderSuccessfulNavigation(context, outcome, entry);
				break;
			case "commit_client_loaders_state":
				setClientLoadersState(command.clientLoadersResult);
				break;
			case "sync_build_id_from_response":
				syncBuildIDFromResponse(command.response);
				break;
			case "apply_response_artifacts_when_build_matches":
				applyResponseArtifactsWhenBuildMatches({
					response: command.response,
					json: command.json,
					expectedBuildID: command.expectedBuildID,
					shouldApplyCSSBundles: command.shouldApplyCSSBundles,
				});
				break;
		}
	}

	return { shouldStop: false };
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

type SuccessfulNavigationCheckpointDecisionInput = {
	checkpoint: SuccessfulNavigationLifecycleCheckpoint;
	entry?: NavigationEntry;
	isCurrentEntry?: boolean;
	currentHref?: string;
	buildIDSyncTiming?: BuildIDSyncTiming;
};

type SuccessfulNavigationCheckpointCommandInput = {
	response?: SuccessfulNavigationOutcome["response"];
	json?: SuccessfulNavigationOutcome["json"];
	expectedBuildID?: string;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
};

type SuccessfulNavigationCheckpointExecutionRuntimeContext = {
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
	buildIDSyncTiming?: BuildIDSyncTiming;
	expectedBuildID?: string;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
};

type SuccessfulNavigationCheckpointExecutionDefinition = {
	buildDecisionInput: (props: {
		checkpoint: SuccessfulNavigationLifecycleCheckpoint;
		runtimeContext: SuccessfulNavigationCheckpointExecutionRuntimeContext;
	}) => SuccessfulNavigationCheckpointDecisionInput;
	buildCommandInput: (props: {
		checkpoint: SuccessfulNavigationLifecycleCheckpoint;
		runtimeContext: SuccessfulNavigationCheckpointExecutionRuntimeContext;
	}) => SuccessfulNavigationCheckpointCommandInput;
};

function requireSuccessfulNavigationCheckpointRuntimeContextValue<T>(props: {
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

const successfulNavigationCheckpointExecutionDefinitionByCheckpoint: Record<
	SuccessfulNavigationLifecycleCheckpoint,
	SuccessfulNavigationCheckpointExecutionDefinition
> = {
	pre_waiting: {
		buildDecisionInput: ({ checkpoint, runtimeContext }) => ({
			checkpoint,
			entry: runtimeContext.entry,
			isCurrentEntry: runtimeContext.isCurrentEntry,
			currentHref: runtimeContext.currentHref,
		}),
		buildCommandInput: () => ({}),
	},
	post_waiting: {
		buildDecisionInput: ({ checkpoint, runtimeContext }) => ({
			checkpoint,
			isCurrentEntry: runtimeContext.isCurrentEntry,
		}),
		buildCommandInput: () => ({}),
	},
	pre_asset_wait: {
		buildDecisionInput: ({ checkpoint, runtimeContext }) => ({
			checkpoint,
			buildIDSyncTiming:
				requireSuccessfulNavigationCheckpointRuntimeContextValue({
					value: runtimeContext.buildIDSyncTiming,
					checkpoint,
					field: "buildIDSyncTiming",
				}),
		}),
		buildCommandInput: ({ runtimeContext }) => ({
			response: runtimeContext.outcome.response,
		}),
	},
	post_asset: {
		buildDecisionInput: ({ checkpoint, runtimeContext }) => ({
			checkpoint,
			entry: runtimeContext.entry,
			isCurrentEntry: runtimeContext.isCurrentEntry,
			currentHref: runtimeContext.currentHref,
			buildIDSyncTiming:
				requireSuccessfulNavigationCheckpointRuntimeContextValue({
					value: runtimeContext.buildIDSyncTiming,
					checkpoint,
					field: "buildIDSyncTiming",
				}),
		}),
		buildCommandInput: ({ checkpoint, runtimeContext }) => ({
			response: runtimeContext.outcome.response,
			json: runtimeContext.outcome.json,
			expectedBuildID:
				requireSuccessfulNavigationCheckpointRuntimeContextValue({
					value: runtimeContext.expectedBuildID,
					checkpoint,
					field: "expectedBuildID",
				}),
			clientLoadersResult: runtimeContext.clientLoadersResult,
		}),
	},
	cleanup: {
		buildDecisionInput: ({ checkpoint, runtimeContext }) => ({
			checkpoint,
			entry: runtimeContext.entry,
			isCurrentEntry: runtimeContext.isCurrentEntry,
		}),
		buildCommandInput: () => ({}),
	},
};

async function decideAndExecuteSuccessfulNavigationLifecycleCheckpoint(
	props: DecideAndExecuteSuccessfulNavigationLifecycleCheckpointProps,
): Promise<{
	shouldStop: boolean;
}> {
	const { context, outcome, entry } = props;
	const runtimeContext: SuccessfulNavigationCheckpointExecutionRuntimeContext =
		{
			outcome,
			entry,
			isCurrentEntry: isCurrentNavigationEntry({
				context,
				entry,
			}),
			currentHref: window.location.href,
			buildIDSyncTiming: props.buildIDSyncTiming,
			expectedBuildID: props.expectedBuildID,
			clientLoadersResult: props.clientLoadersResult,
		};
	const checkpointExecutionDefinition =
		successfulNavigationCheckpointExecutionDefinitionByCheckpoint[
			props.checkpoint
		];
	const checkpointDecisionInput =
		checkpointExecutionDefinition.buildDecisionInput({
			checkpoint: props.checkpoint,
			runtimeContext,
		});
	const checkpointCommandInput =
		checkpointExecutionDefinition.buildCommandInput({
			checkpoint: props.checkpoint,
			runtimeContext,
		});

	const checkpointExecutionPlan =
		decideSuccessfulNavigationLifecycleCheckpointExecutionPlan(
			checkpointDecisionInput,
		);
	return executeSuccessfulNavigationLifecycleCommands({
		commands: buildSuccessfulNavigationLifecycleCheckpointCommands({
			checkpointExecutionPlan,
			response: checkpointCommandInput.response,
			json: checkpointCommandInput.json,
			expectedBuildID: checkpointCommandInput.expectedBuildID,
			clientLoadersResult: checkpointCommandInput.clientLoadersResult,
		}),
		context,
		outcome,
		entry,
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

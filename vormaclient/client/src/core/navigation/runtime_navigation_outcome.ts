import { resolveAbsoluteHref } from "vorma/kit/url";
import { dispatchBuildIDEvent } from "../../platform/events.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import {
	effectuateRedirectDataResult,
	getBuildIDFromResponse,
	syncBuildIDFromRedirectData,
} from "../redirects.ts";
import {
	AssetManager,
	__reRenderApp,
	setClientLoadersState,
} from "../render_runtime.ts";
import {
	decideBuildIDSyncTimingForSuccessfulEntry,
	decideNavigationOutcomeExecutionPlan,
	decideSuccessfulNavigationPostAssetSideEffectPlan,
	decideSuccessfulNavigationLifecycleStageExecutionPlan,
	isIdlePrefetchNavigationEntry,
	toPublicNavigateResult,
	type InternalNavigateResult,
	type NavigationOutcomeExecutionPlan,
	type SuccessfulNavigationLifecycleStage,
	type SuccessfulNavigationLifecycleStageExecutionPlan,
} from "./runtime_navigation_outcome_state_machine.ts";
import {
	buildSuccessfulNavigationLifecycleStageCommands,
	buildSuccessfulNavigationPostAssetLifecycleCommands,
	buildSuccessfulNavigationPreAssetWaitCommands,
	type SuccessfulNavigationLifecycleCommand,
} from "./runtime_navigation_successful_commands.ts";
import { mergeClientModuleMapWithRouteModuleMetadata } from "./route_metadata.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";

export async function handleNavigationOutcome(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	controlPromise: Promise<NavigationOutcome>;
}): Promise<{ didNavigate: boolean }> {
	const internalResult =
		await handleNavigationOutcomeWithInternalResult(props);

	return toPublicNavigateResult({
		internalResult,
	});
}

export async function handleNavigationOutcomeWithInternalResult(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	controlPromise: Promise<NavigationOutcome>;
}): Promise<InternalNavigateResult> {
	const {
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
		outcome,
		controlPromise,
	} = props;
	const targetUrl = resolveAbsoluteHref({ href: navigationProps.href });
	const entry = findNavigationEntry(targetUrl);
	const executionPlan = decideNavigationOutcomeExecutionPlan({
		outcome,
		targetUrl,
		entry,
		controlPromise,
		currentHref: window.location.href,
	});

	const internalResult = await executeNavigationOutcomeExecutionPlan({
		executionPlan,
		targetUrl,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
	});
	return internalResult;
}

async function executeNavigationOutcomeExecutionPlan(props: {
	executionPlan: NavigationOutcomeExecutionPlan;
	targetUrl: string;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
}): Promise<InternalNavigateResult> {
	const {
		executionPlan,
		targetUrl,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
	} = props;

	switch (executionPlan.type) {
		case "deleteAndStop":
			deleteNavigation({
				targetUrl: executionPlan.targetUrl,
				reason: executionPlan.reason,
			});
			return {
				type: "cancelled",
				reason: executionPlan.reason,
			};
		case "stop":
			return {
				type: "cancelled",
				reason: executionPlan.reason,
			};
		case "redirect":
			return await handleRedirectOutcomeExecutionPlan({
				executionPlan,
				deleteNavigation,
				targetUrl,
				navigationProps,
			});
		case "success":
			if (executionPlan.shouldResolveIntent) {
				onNavigationIntentResolved?.();
			}

			await processSuccessfulNavigation(
				executionPlan.outcome,
				executionPlan.entry,
			);
			return {
				type: "committed",
				didNavigate: executionPlan.didNavigate,
			};
	}
}

async function handleRedirectOutcomeExecutionPlan(props: {
	executionPlan: Extract<
		NavigationOutcomeExecutionPlan,
		{ type: "redirect" }
	>;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	targetUrl: string;
	navigationProps: NavigateProps;
}): Promise<InternalNavigateResult> {
	const { executionPlan, deleteNavigation, targetUrl, navigationProps } =
		props;

	if (executionPlan.shouldSyncBuildIDBeforeRedirect) {
		syncBuildIDFromRedirectData(executionPlan.outcome.redirectData);
	}

	deleteNavigation({
		targetUrl,
		reason: executionPlan.reason,
	});
	const redirectResult = await effectuateRedirectDataResult(
		executionPlan.outcome.redirectData,
		navigationProps.redirectCount || 0,
		navigationProps,
	);
	return {
		type: "committed",
		didNavigate: redirectResult?.status === "did",
	};
}

export type ProcessSuccessfulNavigationContext = {
	transitionPhase: (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
};

function isCurrentNavigationEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): boolean {
	const { context, entry } = props;
	return context.findNavigationEntry(entry.targetUrl) === entry;
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

	if (json.cssBundles && json.cssBundles.length > 0) {
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
	const { waitFnPromise, cssBundlePromises } = outcome;

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
				});
				break;
		}
	}

	return { shouldStop: false };
}

function decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry(props: {
	stage: "pre_waiting";
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): Extract<
	SuccessfulNavigationLifecycleStageExecutionPlan,
	{ stage: "pre_waiting" }
>;
function decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry(props: {
	stage: "post_waiting";
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): Extract<
	SuccessfulNavigationLifecycleStageExecutionPlan,
	{ stage: "post_waiting" }
>;
function decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry(props: {
	stage: "post_asset";
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): Extract<
	SuccessfulNavigationLifecycleStageExecutionPlan,
	{ stage: "post_asset" }
>;
function decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry(props: {
	stage: SuccessfulNavigationLifecycleStage;
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): SuccessfulNavigationLifecycleStageExecutionPlan {
	return decideSuccessfulNavigationLifecycleStageExecutionPlan({
		stage: props.stage,
		entry: props.entry,
		isCurrentEntry: isCurrentNavigationEntry({
			context: props.context,
			entry: props.entry,
		}),
		currentHref: window.location.href,
	});
}

async function executeSuccessfulNavigationLifecycleStage(props: {
	stageExecutionPlan: SuccessfulNavigationLifecycleStageExecutionPlan;
	context: ProcessSuccessfulNavigationContext;
	outcome: Extract<NavigationOutcome, { type: "success" }>;
	entry: NavigationEntry;
}): Promise<{ shouldStop: boolean }> {
	const { stageExecutionPlan, context, outcome, entry } = props;
	return executeSuccessfulNavigationLifecycleCommands({
		commands: buildSuccessfulNavigationLifecycleStageCommands({
			stageExecutionPlan,
		}),
		context,
		outcome,
		entry,
	});
}

async function decideAndExecuteSuccessfulNavigationLifecycleStage(props: {
	stage: Exclude<SuccessfulNavigationLifecycleStage, "post_asset">;
	context: ProcessSuccessfulNavigationContext;
	outcome: Extract<NavigationOutcome, { type: "success" }>;
	entry: NavigationEntry;
}): Promise<{
	shouldStop: boolean;
}> {
	const stageExecutionPlan =
		decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry({
			stage: props.stage,
			context: props.context,
			entry: props.entry,
		});

	const stageExecutionResult =
		await executeSuccessfulNavigationLifecycleStage({
			stageExecutionPlan,
			context: props.context,
			outcome: props.outcome,
			entry: props.entry,
		});

	return {
		shouldStop: stageExecutionResult.shouldStop,
	};
}

export async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	try {
		const { response, json } = outcome;
		const expectedBuildIDForResponseArtifacts =
			__vormaClientGlobal.get("buildID");

		for (const stage of ["pre_waiting", "post_waiting"] as const) {
			const stageResult =
				await decideAndExecuteSuccessfulNavigationLifecycleStage({
					stage,
					context,
					outcome,
					entry,
				});
			if (stageResult.shouldStop) {
				return;
			}
		}

		const buildIDSyncTiming = decideBuildIDSyncTimingForSuccessfulEntry({
			entry,
		});
		const preAssetWaitCommandExecutionResult =
			await executeSuccessfulNavigationLifecycleCommands({
				commands: buildSuccessfulNavigationPreAssetWaitCommands({
					buildIDSyncTiming,
					response,
				}),
				context,
				outcome,
				entry,
			});
		if (preAssetWaitCommandExecutionResult.shouldStop) {
			return;
		}

		const assetWaitResult =
			await waitForSuccessfulNavigationAssets(outcome);

		const postAssetExecutionPlan =
			decideSuccessfulNavigationLifecycleStageExecutionPlanForEntry({
				stage: "post_asset",
				context,
				entry,
			});

		const postAssetSideEffectPlan =
			decideSuccessfulNavigationPostAssetSideEffectPlan({
				postAssetStageExecutionPlan: postAssetExecutionPlan,
				buildIDSyncTiming,
			});

		if (
			(
				await executeSuccessfulNavigationLifecycleCommands({
					commands:
						buildSuccessfulNavigationPostAssetLifecycleCommands({
							postAssetExecutionPlan: postAssetExecutionPlan.plan,
							postAssetSideEffectPlan,
							response,
							json,
							expectedBuildID:
								expectedBuildIDForResponseArtifacts,
							clientLoadersResult:
								assetWaitResult.clientLoadersResult,
						}),
					context,
					outcome,
					entry,
				})
			).shouldStop
		) {
			return;
		}
	} finally {
		if (
			!isIdlePrefetchNavigationEntry(entry) &&
			isCurrentNavigationEntry({ context, entry })
		) {
			context.deleteNavigation({
				targetUrl: entry.targetUrl,
				reason: "successful_navigation_cleanup",
			});
		}
	}
}

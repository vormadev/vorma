import { __vormaClientGlobal, setRuntimeBuildID } from "../../app/context.ts";
import { dispatchBuildIDEvent } from "../../platform/events.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import { getBuildIDFromResponse } from "../redirects.ts";
import { AssetManager, __reRenderApp } from "../render_runtime.ts";
import {
	decideBuildIDSyncTimingForSuccessfulEntry,
	decideSuccessfulNavigationCheckpointExecutionPlan,
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

type SuccessfulNavigationOutcome = Extract<
	NavigationOutcome,
	{ type: "success" }
>;
type SuccessfulNavigationClientLoadersResult =
	| Awaited<SuccessfulNavigationOutcome["waitFnPromise"]>
	| undefined;

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

export function syncBuildIDFromResponse(response: Response): void {
	const oldID = __vormaClientGlobal.get("buildID");
	const newID = getBuildIDFromResponse(response);
	if (!newID || newID === oldID) {
		return;
	}

	setRuntimeBuildID({
		buildID: newID,
	});
	dispatchBuildIDEvent({ newID, oldID });
}

async function waitForSuccessfulNavigationAssets(
	outcome: SuccessfulNavigationOutcome,
): Promise<SuccessfulNavigationClientLoadersResult> {
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
	props: SuccessfulNavigationOutcome["props"],
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
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
	clientLoadersResult: SuccessfulNavigationClientLoadersResult,
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
			clientLoadersResult,
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

type SuccessfulNavigationCheckpointStateEnvelope = {
	context: ProcessSuccessfulNavigationContext;
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
};

function readSuccessfulNavigationCheckpointEnvelope(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): {
	isCurrentEntry: boolean;
	currentHref: string;
} {
	const { context, entry } = props;
	return {
		isCurrentEntry: isCurrentNavigationEntry({ context, entry }),
		currentHref: window.location.href,
	};
}

function runSuccessfulNavigationPreWaitingCheckpoint(
	envelope: SuccessfulNavigationCheckpointStateEnvelope,
): boolean {
	const { context, entry } = envelope;
	const checkpointEnvelope = readSuccessfulNavigationCheckpointEnvelope({
		context,
		entry,
	});
	const preWaitingExecutionPlan =
		decideSuccessfulNavigationCheckpointExecutionPlan({
			checkpoint: "pre_waiting",
			entry,
			isCurrentEntry: checkpointEnvelope.isCurrentEntry,
			currentHref: checkpointEnvelope.currentHref,
		}).plan;

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

function runSuccessfulNavigationPostWaitingCheckpoint(
	envelope: SuccessfulNavigationCheckpointStateEnvelope,
): boolean {
	const checkpointEnvelope = readSuccessfulNavigationCheckpointEnvelope({
		context: envelope.context,
		entry: envelope.entry,
	});
	const postWaitingExecutionPlan =
		decideSuccessfulNavigationCheckpointExecutionPlan({
			checkpoint: "post_waiting",
			entry: envelope.entry,
			isCurrentEntry: checkpointEnvelope.isCurrentEntry,
			currentHref: checkpointEnvelope.currentHref,
		}).plan;
	return postWaitingExecutionPlan.type === "stop";
}

function runSuccessfulNavigationPreAssetWaitCheckpoint(props: {
	outcome: SuccessfulNavigationOutcome;
	buildIDSyncTiming: BuildIDSyncTiming;
}): void {
	const { outcome, buildIDSyncTiming } = props;
	if (buildIDSyncTiming === "before_asset_wait") {
		syncBuildIDFromResponse(outcome.response);
	}
}

async function runSuccessfulNavigationPostAssetCheckpoint(props: {
	envelope: SuccessfulNavigationCheckpointStateEnvelope;
	buildIDSyncTiming: BuildIDSyncTiming;
	clientLoadersResult: SuccessfulNavigationClientLoadersResult;
}): Promise<boolean> {
	const { envelope, buildIDSyncTiming, clientLoadersResult } = props;
	const { context, outcome, entry } = envelope;
	const checkpointEnvelope = readSuccessfulNavigationCheckpointEnvelope({
		context,
		entry,
	});
	const postAssetExecutionPlanEnvelope =
		decideSuccessfulNavigationCheckpointExecutionPlan({
			checkpoint: "post_asset",
			entry,
			isCurrentEntry: checkpointEnvelope.isCurrentEntry,
			currentHref: checkpointEnvelope.currentHref,
		});
	const postAssetExecutionPlan = postAssetExecutionPlanEnvelope.plan;
	const shouldSyncBuildIDAfterAssetWait =
		buildIDSyncTiming === "after_asset_wait_if_not_stopped" &&
		postAssetExecutionPlan.type !== "stop";
	if (shouldSyncBuildIDAfterAssetWait) {
		syncBuildIDFromResponse(outcome.response);
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
			await renderSuccessfulNavigation(
				context,
				outcome,
				entry,
				clientLoadersResult,
			);
			return false;
	}
}

function runSuccessfulNavigationCleanupCheckpoint(
	envelope: SuccessfulNavigationCheckpointStateEnvelope,
): void {
	const { context, entry } = envelope;
	const checkpointEnvelope = readSuccessfulNavigationCheckpointEnvelope({
		context,
		entry,
	});
	const cleanupExecutionPlan =
		decideSuccessfulNavigationCheckpointExecutionPlan({
			checkpoint: "cleanup",
			entry,
			isCurrentEntry: checkpointEnvelope.isCurrentEntry,
		}).plan;

	if (cleanupExecutionPlan.type === "skip") {
		return;
	}
	context.deleteNavigation({
		targetUrl: cleanupExecutionPlan.targetUrl,
		reason: cleanupExecutionPlan.reason,
	});
}

export async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
): Promise<void> {
	let didCommitSuccessfulNavigation = false;
	const envelope: SuccessfulNavigationCheckpointStateEnvelope = {
		context,
		outcome,
		entry,
	};

	try {
		if (runSuccessfulNavigationPreWaitingCheckpoint(envelope)) {
			return;
		}
		if (runSuccessfulNavigationPostWaitingCheckpoint(envelope)) {
			return;
		}

		const buildIDSyncTiming = decideBuildIDSyncTimingForSuccessfulEntry({
			entry,
		});
		runSuccessfulNavigationPreAssetWaitCheckpoint({
			outcome,
			buildIDSyncTiming,
		});

		const clientLoadersResult =
			await waitForSuccessfulNavigationAssets(outcome);

		const shouldStopAfterPostAsset =
			await runSuccessfulNavigationPostAssetCheckpoint({
				envelope,
				buildIDSyncTiming,
				clientLoadersResult,
			});
		if (shouldStopAfterPostAsset) {
			return;
		}

		didCommitSuccessfulNavigation = true;
	} finally {
		runSuccessfulNavigationCleanupCheckpoint(envelope);
	}

	if (didCommitSuccessfulNavigation) {
		context.onSuccessfulNavigationCommitted?.({
			entry,
			outcome,
		});
	}
}

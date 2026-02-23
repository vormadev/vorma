import { hasSameDataTarget } from "../../platform/url.ts";
import {
	hasNavigationOperationOwnership,
	type NavigationEntry,
	type NavigationOutcome,
} from "./types.ts";

const navigationOutcomeExecutionReason = {
	stop: {
		entryNotFound: "entry_not_found",
		staleControlOwnership: "stale_control_ownership",
		nonCurrentEntry: "non_current_entry",
		postWaitingEntryLost: "post_waiting_entry_lost",
		postAssetEntryLost: "post_asset_entry_lost",
		postAssetStaleRevalidation: "post_asset_stale_revalidation",
	},
	deleteAndStop: {
		outcomeAborted: "outcome_aborted",
		staleRevalidationPreWaiting: "stale_revalidation_pre_waiting",
		redirectIgnoredForPrefetchOrStaleRevalidation:
			"redirect_ignored_for_prefetch_or_stale_revalidation",
	},
	redirect: {
		effectuate: "redirect_effectuate",
	},
	success: {
		process: "success_process",
	},
} as const;

const successfulNavigationLifecycleReason = {
	preWaiting: {
		nonCurrentEntry: navigationOutcomeExecutionReason.stop.nonCurrentEntry,
		staleRevalidationPreWaiting:
			navigationOutcomeExecutionReason.deleteAndStop
				.staleRevalidationPreWaiting,
		entryCurrentAndFresh: "entry_current_and_fresh",
	},
	postWaiting: {
		entryLost: navigationOutcomeExecutionReason.stop.postWaitingEntryLost,
		entryCurrent: "post_waiting_entry_current",
	},
	postAsset: {
		entryLost: navigationOutcomeExecutionReason.stop.postAssetEntryLost,
		staleRevalidation:
			navigationOutcomeExecutionReason.stop.postAssetStaleRevalidation,
		idlePrefetch: "post_asset_idle_prefetch",
		render: "post_asset_render",
	},
	cleanup: {
		successfulNavigation: "successful_navigation_cleanup",
		skippedIdlePrefetchOrNonCurrentEntry:
			"cleanup_skipped_idle_prefetch_or_non_current_entry",
	},
	preAssetWait: {
		syncBuildIDBeforeAssetWait:
			"pre_asset_wait_sync_build_id_before_asset_wait",
		skipSyncBuildIDBeforeAssetWait:
			"pre_asset_wait_skip_sync_build_id_before_asset_wait",
	},
	internalNavigateResult: {
		navigatePromiseRejected: "navigate_promise_rejected",
	},
} as const;

type NavigationOutcomeStopReason =
	(typeof navigationOutcomeExecutionReason.stop)[keyof typeof navigationOutcomeExecutionReason.stop];
type NavigationOutcomeDeleteAndStopReason =
	(typeof navigationOutcomeExecutionReason.deleteAndStop)[keyof typeof navigationOutcomeExecutionReason.deleteAndStop];
type NavigationOutcomeRedirectReason =
	(typeof navigationOutcomeExecutionReason.redirect)[keyof typeof navigationOutcomeExecutionReason.redirect];
type NavigationOutcomeSuccessReason =
	(typeof navigationOutcomeExecutionReason.success)[keyof typeof navigationOutcomeExecutionReason.success];
type SuccessfulNavigationPreWaitingStopReason =
	typeof successfulNavigationLifecycleReason.preWaiting.nonCurrentEntry;
type SuccessfulNavigationPreWaitingDeleteAndStopReason =
	typeof successfulNavigationLifecycleReason.preWaiting.staleRevalidationPreWaiting;
type SuccessfulNavigationPreWaitingContinueReason =
	typeof successfulNavigationLifecycleReason.preWaiting.entryCurrentAndFresh;
type SuccessfulNavigationPostWaitingStopReason =
	typeof successfulNavigationLifecycleReason.postWaiting.entryLost;
type SuccessfulNavigationPostWaitingContinueReason =
	typeof successfulNavigationLifecycleReason.postWaiting.entryCurrent;
type SuccessfulNavigationPostAssetStopReason =
	| typeof successfulNavigationLifecycleReason.postAsset.entryLost
	| typeof successfulNavigationLifecycleReason.postAsset.staleRevalidation;
type SuccessfulNavigationPostAssetCompleteWithoutRenderReason =
	typeof successfulNavigationLifecycleReason.postAsset.idlePrefetch;
type SuccessfulNavigationPostAssetRenderReason =
	typeof successfulNavigationLifecycleReason.postAsset.render;
type SuccessfulNavigationCleanupDeleteReason =
	typeof successfulNavigationLifecycleReason.cleanup.successfulNavigation;
type SuccessfulNavigationCleanupSkipReason =
	typeof successfulNavigationLifecycleReason.cleanup.skippedIdlePrefetchOrNonCurrentEntry;
type SuccessfulNavigationPreAssetWaitSyncReason =
	typeof successfulNavigationLifecycleReason.preAssetWait.syncBuildIDBeforeAssetWait;
type SuccessfulNavigationPreAssetWaitSkipSyncReason =
	typeof successfulNavigationLifecycleReason.preAssetWait.skipSyncBuildIDBeforeAssetWait;

export function isIdlePrefetchNavigationEntry(entry: NavigationEntry): boolean {
	return entry.type === "prefetch" && entry.intent === "none";
}

export function isStaleRevalidationNavigationEntry(props: {
	entry: NavigationEntry;
	currentHref: string;
}): boolean {
	const { entry, currentHref } = props;
	return (
		entry.type === "revalidation" &&
		!hasSameDataTarget({
			firstHref: currentHref,
			secondHref: entry.originUrl,
		})
	);
}

export type NavigationOutcomeExecutionPlan =
	| {
			type: "stop";
			reason: NavigationOutcomeStopReason;
	  }
	| {
			type: "deleteAndStop";
			targetUrl: string;
			reason: NavigationOutcomeDeleteAndStopReason;
	  }
	| {
			type: "redirect";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "redirect" }>;
			reason: NavigationOutcomeRedirectReason;
	  }
	| {
			type: "success";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "success" }>;
			didNavigate: boolean;
			reason: NavigationOutcomeSuccessReason;
	  };

export function decideNavigationOutcomeExecutionPlan(props: {
	outcome: NavigationOutcome;
	targetUrl: string;
	entry: NavigationEntry | undefined;
	expectedOperationID: number | undefined;
	currentHref: string;
}): NavigationOutcomeExecutionPlan {
	const { outcome, targetUrl, entry, expectedOperationID, currentHref } =
		props;

	if (!entry) {
		return {
			type: "stop",
			reason: navigationOutcomeExecutionReason.stop.entryNotFound,
		};
	}

	if (
		!hasNavigationOperationOwnership({
			entry,
			expectedOperationID,
		})
	) {
		return {
			type: "stop",
			reason: navigationOutcomeExecutionReason.stop.staleControlOwnership,
		};
	}

	if (outcome.type === "aborted") {
		return {
			type: "deleteAndStop",
			targetUrl,
			reason: navigationOutcomeExecutionReason.deleteAndStop
				.outcomeAborted,
		};
	}

	if (outcome.type === "redirect") {
		const shouldIgnoreRedirect =
			isIdlePrefetchNavigationEntry(entry) ||
			isStaleRevalidationNavigationEntry({
				entry,
				currentHref,
			});
		if (shouldIgnoreRedirect) {
			return {
				type: "deleteAndStop",
				targetUrl,
				reason: navigationOutcomeExecutionReason.deleteAndStop
					.redirectIgnoredForPrefetchOrStaleRevalidation,
			};
		}

		return {
			type: "redirect",
			entry,
			outcome,
			reason: navigationOutcomeExecutionReason.redirect.effectuate,
		};
	}

	return {
		type: "success",
		entry,
		outcome,
		didNavigate: !isIdlePrefetchNavigationEntry(entry),
		reason: navigationOutcomeExecutionReason.success.process,
	};
}

export type SuccessfulNavigationPreWaitingExecutionPlan =
	| {
			type: "stop";
			reason: SuccessfulNavigationPreWaitingStopReason;
	  }
	| {
			type: "deleteAndStop";
			targetUrl: string;
			reason: SuccessfulNavigationPreWaitingDeleteAndStopReason;
	  }
	| {
			type: "continue";
			reason: SuccessfulNavigationPreWaitingContinueReason;
	  };

export function decideSuccessfulNavigationPreWaitingExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): SuccessfulNavigationPreWaitingExecutionPlan {
	const { entry, isCurrentEntry, currentHref } = props;
	if (!isCurrentEntry) {
		return {
			type: "stop",
			reason: successfulNavigationLifecycleReason.preWaiting
				.nonCurrentEntry,
		};
	}

	if (
		isStaleRevalidationNavigationEntry({
			entry,
			currentHref,
		})
	) {
		return {
			type: "deleteAndStop",
			targetUrl: entry.targetUrl,
			reason: successfulNavigationLifecycleReason.preWaiting
				.staleRevalidationPreWaiting,
		};
	}

	return {
		type: "continue",
		reason: successfulNavigationLifecycleReason.preWaiting
			.entryCurrentAndFresh,
	};
}

export type SuccessfulNavigationPostWaitingExecutionPlan =
	| {
			type: "stop";
			reason: SuccessfulNavigationPostWaitingStopReason;
	  }
	| {
			type: "continue";
			reason: SuccessfulNavigationPostWaitingContinueReason;
	  };

export function decideSuccessfulNavigationPostWaitingExecutionPlan(props: {
	isCurrentEntry: boolean;
}): SuccessfulNavigationPostWaitingExecutionPlan {
	if (!props.isCurrentEntry) {
		return {
			type: "stop",
			reason: successfulNavigationLifecycleReason.postWaiting.entryLost,
		};
	}

	return {
		type: "continue",
		reason: successfulNavigationLifecycleReason.postWaiting.entryCurrent,
	};
}

export type SuccessfulNavigationPostAssetExecutionPlan =
	| {
			type: "stop";
			reason: SuccessfulNavigationPostAssetStopReason;
	  }
	| {
			type: "completeWithoutRender";
			reason: SuccessfulNavigationPostAssetCompleteWithoutRenderReason;
	  }
	| {
			type: "render";
			reason: SuccessfulNavigationPostAssetRenderReason;
	  };

export function decideSuccessfulNavigationPostAssetExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): SuccessfulNavigationPostAssetExecutionPlan {
	const { entry, isCurrentEntry, currentHref } = props;
	if (!isCurrentEntry) {
		return {
			type: "stop",
			reason: successfulNavigationLifecycleReason.postAsset.entryLost,
		};
	}

	if (isIdlePrefetchNavigationEntry(entry)) {
		return {
			type: "completeWithoutRender",
			reason: successfulNavigationLifecycleReason.postAsset.idlePrefetch,
		};
	}

	if (
		isStaleRevalidationNavigationEntry({
			entry,
			currentHref,
		})
	) {
		return {
			type: "stop",
			reason: successfulNavigationLifecycleReason.postAsset
				.staleRevalidation,
		};
	}

	return {
		type: "render",
		reason: successfulNavigationLifecycleReason.postAsset.render,
	};
}

export type SuccessfulNavigationCleanupExecutionPlan =
	| {
			type: "deleteNavigation";
			targetUrl: string;
			reason: SuccessfulNavigationCleanupDeleteReason;
	  }
	| {
			type: "skip";
			reason: SuccessfulNavigationCleanupSkipReason;
	  };

export function decideSuccessfulNavigationCleanupExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
}): SuccessfulNavigationCleanupExecutionPlan {
	const { entry, isCurrentEntry } = props;
	if (!isCurrentEntry || isIdlePrefetchNavigationEntry(entry)) {
		return {
			type: "skip",
			reason: successfulNavigationLifecycleReason.cleanup
				.skippedIdlePrefetchOrNonCurrentEntry,
		};
	}

	return {
		type: "deleteNavigation",
		targetUrl: entry.targetUrl,
		reason: successfulNavigationLifecycleReason.cleanup
			.successfulNavigation,
	};
}

export type InternalNavigateResult =
	| {
			type: "committed";
			didNavigate: boolean;
	  }
	| {
			type: "cancelled";
			reason:
				| NavigationOutcomeExecutionPlan["reason"]
				| SuccessfulNavigationPreWaitingExecutionPlan["reason"]
				| SuccessfulNavigationPostWaitingExecutionPlan["reason"]
				| SuccessfulNavigationPostAssetExecutionPlan["reason"];
	  }
	| {
			type: "failed";
			reason: typeof successfulNavigationLifecycleReason.internalNavigateResult.navigatePromiseRejected;
	  };

export function toPublicNavigateResult(props: {
	internalResult: InternalNavigateResult;
}): { didNavigate: boolean } {
	if (props.internalResult.type === "committed") {
		return { didNavigate: props.internalResult.didNavigate };
	}

	return { didNavigate: false };
}

export type BuildIDSyncTiming =
	| "before_asset_wait"
	| "after_asset_wait_if_not_stopped";

export function decideBuildIDSyncTimingForSuccessfulEntry(props: {
	entry: NavigationEntry;
}): BuildIDSyncTiming {
	return isIdlePrefetchNavigationEntry(props.entry)
		? "before_asset_wait"
		: "after_asset_wait_if_not_stopped";
}

export type SuccessfulNavigationPreAssetWaitExecutionPlan = {
	shouldSyncBuildIDBeforeAssetWait: boolean;
	reason:
		| SuccessfulNavigationPreAssetWaitSyncReason
		| SuccessfulNavigationPreAssetWaitSkipSyncReason;
};

export function decideSuccessfulNavigationPreAssetWaitExecutionPlan(props: {
	buildIDSyncTiming: BuildIDSyncTiming;
}): SuccessfulNavigationPreAssetWaitExecutionPlan {
	if (props.buildIDSyncTiming === "before_asset_wait") {
		return {
			shouldSyncBuildIDBeforeAssetWait: true,
			reason: successfulNavigationLifecycleReason.preAssetWait
				.syncBuildIDBeforeAssetWait,
		};
	}

	return {
		shouldSyncBuildIDBeforeAssetWait: false,
		reason: successfulNavigationLifecycleReason.preAssetWait
			.skipSyncBuildIDBeforeAssetWait,
	};
}

export type SuccessfulNavigationPostAssetSideEffectPlan = {
	shouldCommitClientLoadersState: boolean;
	shouldSyncBuildIDAfterAssetWait: boolean;
	shouldApplyResponseArtifacts: boolean;
};

export function decideSuccessfulNavigationPostAssetSideEffectPlan(props: {
	postAssetExecutionPlan: SuccessfulNavigationPostAssetExecutionPlan;
	buildIDSyncTiming: BuildIDSyncTiming;
}): SuccessfulNavigationPostAssetSideEffectPlan {
	const { postAssetExecutionPlan, buildIDSyncTiming } = props;
	const shouldStop = postAssetExecutionPlan.type === "stop";

	return {
		shouldCommitClientLoadersState:
			postAssetExecutionPlan.type === "render",
		shouldSyncBuildIDAfterAssetWait:
			buildIDSyncTiming === "after_asset_wait_if_not_stopped" &&
			!shouldStop,
		shouldApplyResponseArtifacts: !shouldStop,
	};
}

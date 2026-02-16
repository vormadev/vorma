import { hasSameDataTarget } from "../../platform/url.ts";
import type { NavigationEntry, NavigationOutcome } from "./types.ts";
import { hasNavigationControlPromiseOwnership } from "./types.ts";

export function isIdlePrefetchNavigationEntry(entry: NavigationEntry): boolean {
	return entry.type === "prefetch" && entry.intent === "none";
}

export function shouldResolveNavigationIntentForEntry(
	entry: NavigationEntry,
): boolean {
	return entry.intent === "navigate" || entry.intent === "revalidate";
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
			reason:
				| "entry_not_found"
				| "stale_control_ownership"
				| "non_current_entry"
				| "post_waiting_entry_lost"
				| "post_asset_entry_lost"
				| "post_asset_stale_revalidation";
	  }
	| {
			type: "deleteAndStop";
			targetUrl: string;
			reason:
				| "outcome_aborted"
				| "stale_revalidation_pre_waiting"
				| "redirect_ignored_for_prefetch_or_stale_revalidation";
	  }
	| {
			type: "redirect";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "redirect" }>;
			shouldSyncBuildIDBeforeRedirect: boolean;
			reason: "redirect_effectuate";
	  }
	| {
			type: "success";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "success" }>;
			shouldResolveIntent: boolean;
			didNavigate: boolean;
			reason: "success_process";
	  };

export function decideNavigationOutcomeExecutionPlan(props: {
	outcome: NavigationOutcome;
	targetUrl: string;
	entry: NavigationEntry | undefined;
	controlPromise: Promise<NavigationOutcome>;
	currentHref: string;
}): NavigationOutcomeExecutionPlan {
	const { outcome, targetUrl, entry, controlPromise, currentHref } = props;

	if (!entry) {
		return {
			type: "stop",
			reason: "entry_not_found",
		};
	}

	if (!hasNavigationControlPromiseOwnership(entry, controlPromise)) {
		return {
			type: "stop",
			reason: "stale_control_ownership",
		};
	}

	if (outcome.type === "aborted") {
		return {
			type: "deleteAndStop",
			targetUrl,
			reason: "outcome_aborted",
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
				reason: "redirect_ignored_for_prefetch_or_stale_revalidation",
			};
		}

		return {
			type: "redirect",
			entry,
			outcome,
			shouldSyncBuildIDBeforeRedirect: true,
			reason: "redirect_effectuate",
		};
	}

	return {
		type: "success",
		entry,
		outcome,
		shouldResolveIntent: shouldResolveNavigationIntentForEntry(entry),
		didNavigate: !isIdlePrefetchNavigationEntry(entry),
		reason: "success_process",
	};
}

export type SuccessfulNavigationPreWaitingExecutionPlan =
	| {
			type: "stop";
			reason: "non_current_entry";
	  }
	| {
			type: "deleteAndStop";
			targetUrl: string;
			reason: "stale_revalidation_pre_waiting";
	  }
	| {
			type: "continue";
			reason: "entry_current_and_fresh";
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
			reason: "non_current_entry",
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
			reason: "stale_revalidation_pre_waiting",
		};
	}

	return {
		type: "continue",
		reason: "entry_current_and_fresh",
	};
}

export type SuccessfulNavigationPostWaitingExecutionPlan =
	| {
			type: "stop";
			reason: "post_waiting_entry_lost";
	  }
	| {
			type: "continue";
			reason: "post_waiting_entry_current";
	  };

export function decideSuccessfulNavigationPostWaitingExecutionPlan(props: {
	isCurrentEntry: boolean;
}): SuccessfulNavigationPostWaitingExecutionPlan {
	if (!props.isCurrentEntry) {
		return {
			type: "stop",
			reason: "post_waiting_entry_lost",
		};
	}

	return {
		type: "continue",
		reason: "post_waiting_entry_current",
	};
}

export type SuccessfulNavigationPostAssetExecutionPlan =
	| {
			type: "stop";
			reason: "post_asset_entry_lost" | "post_asset_stale_revalidation";
	  }
	| {
			type: "completeWithoutRender";
			reason: "post_asset_idle_prefetch";
	  }
	| {
			type: "render";
			reason: "post_asset_render";
	  };

export type SuccessfulNavigationLifecycleStage =
	| "pre_waiting"
	| "post_waiting"
	| "post_asset";

export type SuccessfulNavigationLifecycleStageExecutionPlan =
	| {
			stage: "pre_waiting";
			plan: SuccessfulNavigationPreWaitingExecutionPlan;
	  }
	| {
			stage: "post_waiting";
			plan: SuccessfulNavigationPostWaitingExecutionPlan;
	  }
	| {
			stage: "post_asset";
			plan: SuccessfulNavigationPostAssetExecutionPlan;
	  };

type SuccessfulNavigationLifecycleStageDecisionContext = {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
};

export function decideSuccessfulNavigationLifecycleStageExecutionPlan(
	props: {
		stage: "pre_waiting";
	} & SuccessfulNavigationLifecycleStageDecisionContext,
): {
	stage: "pre_waiting";
	plan: SuccessfulNavigationPreWaitingExecutionPlan;
};
export function decideSuccessfulNavigationLifecycleStageExecutionPlan(
	props: {
		stage: "post_waiting";
	} & SuccessfulNavigationLifecycleStageDecisionContext,
): {
	stage: "post_waiting";
	plan: SuccessfulNavigationPostWaitingExecutionPlan;
};
export function decideSuccessfulNavigationLifecycleStageExecutionPlan(
	props: {
		stage: "post_asset";
	} & SuccessfulNavigationLifecycleStageDecisionContext,
): {
	stage: "post_asset";
	plan: SuccessfulNavigationPostAssetExecutionPlan;
};
export function decideSuccessfulNavigationLifecycleStageExecutionPlan(
	props: {
		stage: SuccessfulNavigationLifecycleStage;
	} & SuccessfulNavigationLifecycleStageDecisionContext,
): SuccessfulNavigationLifecycleStageExecutionPlan {
	const { stage, entry, isCurrentEntry, currentHref } = props;

	switch (stage) {
		case "pre_waiting":
			return {
				stage,
				plan: decideSuccessfulNavigationPreWaitingExecutionPlan({
					entry,
					isCurrentEntry,
					currentHref,
				}),
			};
		case "post_waiting":
			return {
				stage,
				plan: decideSuccessfulNavigationPostWaitingExecutionPlan({
					isCurrentEntry,
				}),
			};
		case "post_asset":
			return {
				stage,
				plan: decideSuccessfulNavigationPostAssetExecutionPlan({
					entry,
					isCurrentEntry,
					currentHref,
				}),
			};
	}
}

export function decideSuccessfulNavigationPostAssetExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): SuccessfulNavigationPostAssetExecutionPlan {
	const { entry, isCurrentEntry, currentHref } = props;
	if (!isCurrentEntry) {
		return {
			type: "stop",
			reason: "post_asset_entry_lost",
		};
	}

	if (isIdlePrefetchNavigationEntry(entry)) {
		return {
			type: "completeWithoutRender",
			reason: "post_asset_idle_prefetch",
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
			reason: "post_asset_stale_revalidation",
		};
	}

	return {
		type: "render",
		reason: "post_asset_render",
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
			reason: "navigate_promise_rejected";
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

export type SuccessfulNavigationPostAssetSideEffectPlan = {
	shouldCommitClientLoadersState: boolean;
	shouldSyncBuildIDAfterAssetWait: boolean;
	shouldApplyResponseArtifacts: boolean;
};

export function decideSuccessfulNavigationPostAssetSideEffectPlan(props: {
	postAssetStageExecutionPlan: Extract<
		SuccessfulNavigationLifecycleStageExecutionPlan,
		{ stage: "post_asset" }
	>;
	buildIDSyncTiming: BuildIDSyncTiming;
}): SuccessfulNavigationPostAssetSideEffectPlan {
	const { postAssetStageExecutionPlan, buildIDSyncTiming } = props;
	const shouldStop = postAssetStageExecutionPlan.plan.type === "stop";

	return {
		shouldCommitClientLoadersState: !shouldStop,
		shouldSyncBuildIDAfterAssetWait:
			buildIDSyncTiming === "after_asset_wait_if_not_stopped" &&
			!shouldStop,
		shouldApplyResponseArtifacts: !shouldStop,
	};
}

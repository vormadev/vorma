import { hasSameDataTarget } from "../../platform/url.ts";
import {
	hasNavigationOperationOwnership,
	type NavigationEntry,
	type NavigationOutcome,
} from "./types.ts";

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
			outcome: Extract<NavigationOutcome, { type: "redirect" }>;
			reason: "redirect_effectuate";
	  }
	| {
			type: "success";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "success" }>;
			didNavigate: boolean;
			reason: "success_process";
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
			reason: "entry_not_found",
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
			outcome,
			reason: "redirect_effectuate",
		};
	}

	return {
		type: "success",
		entry,
		outcome,
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

export type SuccessfulNavigationCleanupExecutionPlan =
	| {
			type: "deleteNavigation";
			targetUrl: string;
			reason: "successful_navigation_cleanup";
	  }
	| {
			type: "skip";
			reason: "cleanup_skipped_idle_prefetch_or_non_current_entry";
	  };

export function decideSuccessfulNavigationCleanupExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
}): SuccessfulNavigationCleanupExecutionPlan {
	const { entry, isCurrentEntry } = props;
	if (!isCurrentEntry || isIdlePrefetchNavigationEntry(entry)) {
		return {
			type: "skip",
			reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
		};
	}

	return {
		type: "deleteNavigation",
		targetUrl: entry.targetUrl,
		reason: "successful_navigation_cleanup",
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

export type SuccessfulNavigationPreAssetWaitExecutionPlan = {
	shouldSyncBuildIDBeforeAssetWait: boolean;
	reason:
		| "pre_asset_wait_sync_build_id_before_asset_wait"
		| "pre_asset_wait_skip_sync_build_id_before_asset_wait";
};

export function decideSuccessfulNavigationPreAssetWaitExecutionPlan(props: {
	buildIDSyncTiming: BuildIDSyncTiming;
}): SuccessfulNavigationPreAssetWaitExecutionPlan {
	if (props.buildIDSyncTiming === "before_asset_wait") {
		return {
			shouldSyncBuildIDBeforeAssetWait: true,
			reason: "pre_asset_wait_sync_build_id_before_asset_wait",
		};
	}

	return {
		shouldSyncBuildIDBeforeAssetWait: false,
		reason: "pre_asset_wait_skip_sync_build_id_before_asset_wait",
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
		shouldCommitClientLoadersState: !shouldStop,
		shouldSyncBuildIDAfterAssetWait:
			buildIDSyncTiming === "after_asset_wait_if_not_stopped" &&
			!shouldStop,
		shouldApplyResponseArtifacts: !shouldStop,
	};
}

export type SuccessfulNavigationPostAssetLifecycleExecutionPlan = {
	postAssetExecutionPlan: SuccessfulNavigationPostAssetExecutionPlan;
	postAssetSideEffectPlan: SuccessfulNavigationPostAssetSideEffectPlan;
};

export function decideSuccessfulNavigationPostAssetLifecycleExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
	buildIDSyncTiming: BuildIDSyncTiming;
}): SuccessfulNavigationPostAssetLifecycleExecutionPlan {
	const postAssetExecutionPlan =
		decideSuccessfulNavigationPostAssetExecutionPlan({
			entry: props.entry,
			isCurrentEntry: props.isCurrentEntry,
			currentHref: props.currentHref,
		});

	const postAssetSideEffectPlan =
		decideSuccessfulNavigationPostAssetSideEffectPlan({
			postAssetExecutionPlan,
			buildIDSyncTiming: props.buildIDSyncTiming,
		});

	return {
		postAssetExecutionPlan,
		postAssetSideEffectPlan,
	};
}

export type SuccessfulNavigationLifecycleCheckpoint =
	| "pre_waiting"
	| "post_waiting"
	| "pre_asset_wait"
	| "post_asset"
	| "cleanup";

export type SuccessfulNavigationLifecycleCheckpointExecutionPlan =
	| {
			checkpoint: "pre_waiting";
			preWaitingExecutionPlan: SuccessfulNavigationPreWaitingExecutionPlan;
	  }
	| {
			checkpoint: "post_waiting";
			postWaitingExecutionPlan: SuccessfulNavigationPostWaitingExecutionPlan;
	  }
	| {
			checkpoint: "pre_asset_wait";
			preAssetWaitExecutionPlan: SuccessfulNavigationPreAssetWaitExecutionPlan;
	  }
	| {
			checkpoint: "post_asset";
			postAssetLifecycleExecutionPlan: SuccessfulNavigationPostAssetLifecycleExecutionPlan;
	  }
	| {
			checkpoint: "cleanup";
			cleanupExecutionPlan: SuccessfulNavigationCleanupExecutionPlan;
	  };

function requireSuccessfulNavigationCheckpointContextValue<T>(props: {
	value: T | undefined;
	checkpoint: SuccessfulNavigationLifecycleCheckpoint;
	field: string;
}): T {
	if (props.value === undefined) {
		throw new Error(
			`Missing '${props.field}' for successful navigation checkpoint '${props.checkpoint}'.`,
		);
	}

	return props.value;
}

export function decideSuccessfulNavigationLifecycleCheckpointExecutionPlan(props: {
	checkpoint: SuccessfulNavigationLifecycleCheckpoint;
	entry?: NavigationEntry;
	isCurrentEntry?: boolean;
	currentHref?: string;
	buildIDSyncTiming?: BuildIDSyncTiming;
}): SuccessfulNavigationLifecycleCheckpointExecutionPlan {
	switch (props.checkpoint) {
		case "pre_waiting": {
			const entry = requireSuccessfulNavigationCheckpointContextValue({
				value: props.entry,
				checkpoint: props.checkpoint,
				field: "entry",
			});
			const isCurrentEntry =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.isCurrentEntry,
					checkpoint: props.checkpoint,
					field: "isCurrentEntry",
				});
			const currentHref =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.currentHref,
					checkpoint: props.checkpoint,
					field: "currentHref",
				});

			return {
				checkpoint: props.checkpoint,
				preWaitingExecutionPlan:
					decideSuccessfulNavigationPreWaitingExecutionPlan({
						entry,
						isCurrentEntry,
						currentHref,
					}),
			};
		}
		case "post_waiting": {
			const isCurrentEntry =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.isCurrentEntry,
					checkpoint: props.checkpoint,
					field: "isCurrentEntry",
				});

			return {
				checkpoint: props.checkpoint,
				postWaitingExecutionPlan:
					decideSuccessfulNavigationPostWaitingExecutionPlan({
						isCurrentEntry,
					}),
			};
		}
		case "pre_asset_wait": {
			const buildIDSyncTiming =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.buildIDSyncTiming,
					checkpoint: props.checkpoint,
					field: "buildIDSyncTiming",
				});

			return {
				checkpoint: props.checkpoint,
				preAssetWaitExecutionPlan:
					decideSuccessfulNavigationPreAssetWaitExecutionPlan({
						buildIDSyncTiming,
					}),
			};
		}
		case "post_asset": {
			const entry = requireSuccessfulNavigationCheckpointContextValue({
				value: props.entry,
				checkpoint: props.checkpoint,
				field: "entry",
			});
			const isCurrentEntry =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.isCurrentEntry,
					checkpoint: props.checkpoint,
					field: "isCurrentEntry",
				});
			const currentHref =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.currentHref,
					checkpoint: props.checkpoint,
					field: "currentHref",
				});
			const buildIDSyncTiming =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.buildIDSyncTiming,
					checkpoint: props.checkpoint,
					field: "buildIDSyncTiming",
				});

			return {
				checkpoint: props.checkpoint,
				postAssetLifecycleExecutionPlan:
					decideSuccessfulNavigationPostAssetLifecycleExecutionPlan({
						entry,
						isCurrentEntry,
						currentHref,
						buildIDSyncTiming,
					}),
			};
		}
		case "cleanup": {
			const entry = requireSuccessfulNavigationCheckpointContextValue({
				value: props.entry,
				checkpoint: props.checkpoint,
				field: "entry",
			});
			const isCurrentEntry =
				requireSuccessfulNavigationCheckpointContextValue({
					value: props.isCurrentEntry,
					checkpoint: props.checkpoint,
					field: "isCurrentEntry",
				});

			return {
				checkpoint: props.checkpoint,
				cleanupExecutionPlan:
					decideSuccessfulNavigationCleanupExecutionPlan({
						entry,
						isCurrentEntry,
					}),
			};
		}
	}
}

import { describe, expect, it } from "vitest";
import type {
	NavigationEntry,
	NavigationOutcome,
} from "../../core/navigation/types.ts";
import {
	decideBuildIDSyncTimingForSuccessfulEntry,
	decideNavigationOutcomeExecutionPlan,
	decideSuccessfulNavigationCleanupExecutionPlan,
	decideSuccessfulNavigationLifecycleCheckpointExecutionPlan,
	decideSuccessfulNavigationPostAssetLifecycleExecutionPlan,
	decideSuccessfulNavigationPostAssetSideEffectPlan,
	decideSuccessfulNavigationPostAssetExecutionPlan,
	decideSuccessfulNavigationPostWaitingExecutionPlan,
	decideSuccessfulNavigationPreAssetWaitExecutionPlan,
	decideSuccessfulNavigationPreWaitingExecutionPlan,
	toPublicNavigateResult,
} from "../../core/navigation/runtime_navigation_outcome_state_machine.ts";

function createEntry(props: {
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
	targetUrl?: string;
	originUrl?: string;
}): NavigationEntry {
	return {
		operationID: 1,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl || "http://localhost:3000/target",
		originUrl: props.originUrl || "http://localhost:3000/origin",
	};
}

function createSuccessOutcome(): Extract<
	NavigationOutcome,
	{ type: "success" }
> {
	return {
		type: "success",
		response: new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": "1",
			},
		}),
		json: {
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			deps: [],
			cssBundles: [],
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			title: undefined,
			metaHeadEls: undefined,
			restHeadEls: undefined,
		},
		preloadCommands: [],
		waitFnPromise: Promise.resolve({ data: [] }),
		props: {
			href: "http://localhost:3000/target",
			navigationType: "userNavigation",
		},
	};
}

function createRedirectOutcome(): Extract<
	NavigationOutcome,
	{ type: "redirect" }
> {
	return {
		type: "redirect",
		redirectData: {
			status: "should",
			shouldRedirectStrategy: "soft",
			latestBuildID: "2",
			href: "/redirect-target",
			hrefDetails: {
				url: new URL("http://localhost:3000/redirect-target"),
				isHTTP: true,
				isInternal: true,
				isExternal: false,
				absoluteURL: "http://localhost:3000/redirect-target",
				relativeURL: "/redirect-target",
			},
		},
		props: {
			href: "http://localhost:3000/target",
			navigationType: "userNavigation",
		},
	};
}

describe("navigation outcome state machine", () => {
	it("stops when target entry is missing", () => {
		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: { type: "aborted" },
			targetUrl: "http://localhost:3000/missing",
			entry: undefined,
			expectedOperationID: undefined,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toEqual({
			type: "stop",
			reason: "entry_not_found",
		});
	});

	it("stops when operation-id ownership is stale", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: createSuccessOutcome(),
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID + 1,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toEqual({
			type: "stop",
			reason: "stale_control_ownership",
		});
	});

	it("accepts explicit operation-id ownership", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: createSuccessOutcome(),
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toMatchObject({
			type: "success",
			didNavigate: true,
			reason: "success_process",
		});
	});

	it("ignores redirect outcomes for idle prefetch entries", () => {
		const redirectOutcome = createRedirectOutcome();
		const entry = createEntry({
			type: "prefetch",
			intent: "none",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: redirectOutcome,
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toEqual({
			type: "deleteAndStop",
			targetUrl: entry.targetUrl,
			reason: "redirect_ignored_for_prefetch_or_stale_revalidation",
		});
	});

	it("effectuates redirect outcomes for current navigate entries", () => {
		const redirectOutcome = createRedirectOutcome();
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: redirectOutcome,
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});

		expect(plan.type).toBe("redirect");
		if (plan.type !== "redirect") {
			return;
		}
		expect(plan.reason).toBe("redirect_effectuate");
	});

	it("emits success execution metadata for current non-prefetch entries", () => {
		const successOutcome = createSuccessOutcome();
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: successOutcome,
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toMatchObject({
			type: "success",
			didNavigate: true,
			reason: "success_process",
		});
	});

	it("marks idle prefetch success as non-navigating", () => {
		const successOutcome = createSuccessOutcome();
		const entry = createEntry({
			type: "prefetch",
			intent: "none",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: successOutcome,
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});

		expect(plan).toMatchObject({
			type: "success",
			didNavigate: false,
		});
	});
});

describe("successful outcome stage plans", () => {
	it("pre-waiting stops when entry is no longer current", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const plan = decideSuccessfulNavigationPreWaitingExecutionPlan({
			entry,
			isCurrentEntry: false,
			currentHref: "http://localhost:3000/current",
		});
		expect(plan).toEqual({
			type: "stop",
			reason: "non_current_entry",
		});
	});

	it("pre-waiting deletes stale revalidation entries", () => {
		const entry = createEntry({
			type: "revalidation",
			intent: "revalidate",
			originUrl: "http://localhost:3000/origin",
			targetUrl: "http://localhost:3000/current",
		});
		const plan = decideSuccessfulNavigationPreWaitingExecutionPlan({
			entry,
			isCurrentEntry: true,
			currentHref: "http://localhost:3000/other",
		});
		expect(plan).toEqual({
			type: "deleteAndStop",
			targetUrl: entry.targetUrl,
			reason: "stale_revalidation_pre_waiting",
		});
	});

	it("post-waiting continues only when current entry remains current", () => {
		expect(
			decideSuccessfulNavigationPostWaitingExecutionPlan({
				isCurrentEntry: true,
			}),
		).toEqual({
			type: "continue",
			reason: "post_waiting_entry_current",
		});
		expect(
			decideSuccessfulNavigationPostWaitingExecutionPlan({
				isCurrentEntry: false,
			}),
		).toEqual({
			type: "stop",
			reason: "post_waiting_entry_lost",
		});
	});

	it("post-asset renders only for current non-prefetch fresh entries", () => {
		const activeEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		expect(
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: activeEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			type: "render",
			reason: "post_asset_render",
		});

		const prefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		expect(
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: prefetchEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			type: "completeWithoutRender",
			reason: "post_asset_idle_prefetch",
		});
	});

	it("decides successful-navigation cleanup through one explicit cleanup seam", () => {
		const activeEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/cleanup-target",
		});
		expect(
			decideSuccessfulNavigationCleanupExecutionPlan({
				entry: activeEntry,
				isCurrentEntry: true,
			}),
		).toEqual({
			type: "deleteNavigation",
			targetUrl: "http://localhost:3000/cleanup-target",
			reason: "successful_navigation_cleanup",
		});

		const idlePrefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
			targetUrl: "http://localhost:3000/prefetch-cleanup-target",
		});
		expect(
			decideSuccessfulNavigationCleanupExecutionPlan({
				entry: idlePrefetchEntry,
				isCurrentEntry: true,
			}),
		).toEqual({
			type: "skip",
			reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
		});

		expect(
			decideSuccessfulNavigationCleanupExecutionPlan({
				entry: activeEntry,
				isCurrentEntry: false,
			}),
		).toEqual({
			type: "skip",
			reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
		});
	});

	it("decides post-asset side-effect commits through one reducer seam", () => {
		const stoppedPostAssetExecutionPlan =
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: createEntry({
					type: "userNavigation",
					intent: "navigate",
				}),
				isCurrentEntry: false,
				currentHref: "http://localhost:3000/current",
			});
		expect(
			decideSuccessfulNavigationPostAssetSideEffectPlan({
				postAssetExecutionPlan: stoppedPostAssetExecutionPlan,
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
			}),
		).toEqual({
			shouldCommitClientLoadersState: false,
			shouldSyncBuildIDAfterAssetWait: false,
			shouldApplyResponseArtifacts: false,
		});

		const renderingPostAssetExecutionPlan =
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: createEntry({
					type: "userNavigation",
					intent: "navigate",
				}),
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			});
		expect(
			decideSuccessfulNavigationPostAssetSideEffectPlan({
				postAssetExecutionPlan: renderingPostAssetExecutionPlan,
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
			}),
		).toEqual({
			shouldCommitClientLoadersState: true,
			shouldSyncBuildIDAfterAssetWait: true,
			shouldApplyResponseArtifacts: true,
		});
	});

	it("decides pre-asset wait execution through one reducer seam", () => {
		expect(
			decideSuccessfulNavigationPreAssetWaitExecutionPlan({
				buildIDSyncTiming: "before_asset_wait",
			}),
		).toEqual({
			shouldSyncBuildIDBeforeAssetWait: true,
			reason: "pre_asset_wait_sync_build_id_before_asset_wait",
		});

		expect(
			decideSuccessfulNavigationPreAssetWaitExecutionPlan({
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
			}),
		).toEqual({
			shouldSyncBuildIDBeforeAssetWait: false,
			reason: "pre_asset_wait_skip_sync_build_id_before_asset_wait",
		});
	});

	it("decides post-asset stage plus side effects through one combined reducer seam", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});

		expect(
			decideSuccessfulNavigationPostAssetLifecycleExecutionPlan({
				entry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
			}),
		).toEqual({
			postAssetExecutionPlan: {
				type: "render",
				reason: "post_asset_render",
			},
			postAssetSideEffectPlan: {
				shouldCommitClientLoadersState: true,
				shouldSyncBuildIDAfterAssetWait: true,
				shouldApplyResponseArtifacts: true,
			},
		});

		expect(
			decideSuccessfulNavigationPostAssetLifecycleExecutionPlan({
				entry,
				isCurrentEntry: false,
				currentHref: "http://localhost:3000/current",
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
			}),
		).toEqual({
			postAssetExecutionPlan: {
				type: "stop",
				reason: "post_asset_entry_lost",
			},
			postAssetSideEffectPlan: {
				shouldCommitClientLoadersState: false,
				shouldSyncBuildIDAfterAssetWait: false,
				shouldApplyResponseArtifacts: false,
			},
		});
	});

	it("decides lifecycle checkpoint execution plans through one checkpoint seam", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/checkpoint-target",
		});

		expect(
			decideSuccessfulNavigationLifecycleCheckpointExecutionPlan({
				checkpoint: "pre_waiting",
				entry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			checkpoint: "pre_waiting",
			preWaitingExecutionPlan: {
				type: "continue",
				reason: "entry_current_and_fresh",
			},
		});

		expect(
			decideSuccessfulNavigationLifecycleCheckpointExecutionPlan({
				checkpoint: "pre_asset_wait",
				buildIDSyncTiming: "before_asset_wait",
			}),
		).toEqual({
			checkpoint: "pre_asset_wait",
			preAssetWaitExecutionPlan: {
				shouldSyncBuildIDBeforeAssetWait: true,
				reason: "pre_asset_wait_sync_build_id_before_asset_wait",
			},
		});

		expect(
			decideSuccessfulNavigationLifecycleCheckpointExecutionPlan({
				checkpoint: "cleanup",
				entry,
				isCurrentEntry: false,
			}),
		).toEqual({
			checkpoint: "cleanup",
			cleanupExecutionPlan: {
				type: "skip",
				reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
			},
		});
	});
});

describe("internal navigate result mapping", () => {
	it("maps committed and cancelled internal results to public navigate output", () => {
		expect(
			toPublicNavigateResult({
				internalResult: {
					type: "committed",
					didNavigate: true,
				},
			}),
		).toEqual({ didNavigate: true });
		expect(
			toPublicNavigateResult({
				internalResult: {
					type: "cancelled",
					reason: "entry_not_found",
				},
			}),
		).toEqual({ didNavigate: false });
		expect(
			toPublicNavigateResult({
				internalResult: {
					type: "failed",
					reason: "navigate_promise_rejected",
				},
			}),
		).toEqual({ didNavigate: false });
	});
});

describe("build-id sync timing policy", () => {
	it("syncs idle prefetch build IDs before asset wait and navigations after post-wait ownership checks", () => {
		const idlePrefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		expect(
			decideBuildIDSyncTimingForSuccessfulEntry({
				entry: idlePrefetchEntry,
			}),
		).toBe("before_asset_wait");

		const navigationEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		expect(
			decideBuildIDSyncTimingForSuccessfulEntry({
				entry: navigationEntry,
			}),
		).toBe("after_asset_wait_if_not_stopped");
	});
});

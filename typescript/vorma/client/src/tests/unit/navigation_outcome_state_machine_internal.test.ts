import { describe, expect, it } from "vitest";
import type {
	NavigationEntry,
	NavigationOutcome,
} from "../../../src/runtime.ts";
import {
	buildNavigationOutcomeRuntimeCommandPlan,
	buildNavigationPassRejectionRuntimeCommandPlan,
	buildSuccessfulNavigationCleanupRuntimeCommandPlan,
	buildSuccessfulNavigationPostAssetRuntimeCommandPlan,
	buildSuccessfulNavigationPostWaitingRuntimeCommandPlan,
	buildSuccessfulNavigationPreAssetWaitRuntimeCommandPlan,
	buildSuccessfulNavigationPreWaitingRuntimeCommandPlan,
	decideNavigationOutcomeExecutionPlan,
	decideNavigationPassRejectionExecutionPlan,
	decideSuccessfulNavigationCheckpointExecutionPlan,
	decideSuccessfulNavigationCleanupExecutionPlan,
	decideSuccessfulNavigationPostAssetExecutionPlan,
	decideSuccessfulNavigationPreWaitingExecutionPlan,
	reduceNavigationPassEvent,
	reduceSuccessfulNavigationCheckpointEvent,
	resolveNavigationEntryLifecycleState,
	resolveSuccessfulEntryBuildIDSyncPolicy,
	toPublicNavigateResult,
} from "../../runtime.ts";

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
		preloadPlan: {
			moduleDependencies: [],
			cssBundles: [],
		},
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

	it("ignores redirect outcomes for stale revalidation entries", () => {
		const redirectOutcome = createRedirectOutcome();
		const staleRevalidationEntry = createEntry({
			type: "revalidation",
			intent: "revalidate",
			originUrl: "http://localhost:3000/origin",
			targetUrl: "http://localhost:3000/current",
		});

		const plan = decideNavigationOutcomeExecutionPlan({
			outcome: redirectOutcome,
			targetUrl: staleRevalidationEntry.targetUrl,
			entry: staleRevalidationEntry,
			expectedOperationID: staleRevalidationEntry.operationID,
			currentHref: "http://localhost:3000/other",
		});

		expect(plan).toEqual({
			type: "deleteAndStop",
			targetUrl: staleRevalidationEntry.targetUrl,
			reason: "redirect_ignored_for_prefetch_or_stale_revalidation",
		});
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

	it("builds redirect runtime command plans with explicit command ordering", () => {
		const redirectOutcome = createRedirectOutcome();
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const executionPlan = decideNavigationOutcomeExecutionPlan({
			outcome: redirectOutcome,
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});
		const commandPlan = buildNavigationOutcomeRuntimeCommandPlan({
			executionPlan,
			resolvedTargetUrl: entry.targetUrl,
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
				redirectCount: 2,
			},
		});

		expect(commandPlan.terminalResult).toEqual({
			type: "committed_from_redirect",
		});
		expect(commandPlan.commands.map((command) => command.type)).toEqual([
			"sync_redirect_build_id",
			"delete_navigation",
			"effectuate_redirect",
		]);
		const effectuateRedirectCommand = commandPlan.commands[2];
		if (effectuateRedirectCommand?.type !== "effectuate_redirect") {
			throw new Error("expected effectuate_redirect command");
		}
		expect(effectuateRedirectCommand.redirectCount).toBe(2);
		expect(effectuateRedirectCommand.navigationProps).toMatchObject({
			href: entry.targetUrl,
			navigationType: entry.type,
			scrollToTop: entry.scrollToTop,
			replace: entry.replace,
			state: entry.state,
		});
	});

	it("builds delete-and-stop runtime command plans for aborted outcomes", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const executionPlan = decideNavigationOutcomeExecutionPlan({
			outcome: { type: "aborted" },
			targetUrl: entry.targetUrl,
			entry,
			expectedOperationID: entry.operationID,
			currentHref: "http://localhost:3000/",
		});
		const commandPlan = buildNavigationOutcomeRuntimeCommandPlan({
			executionPlan,
			resolvedTargetUrl: entry.targetUrl,
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
			},
		});

		expect(commandPlan.terminalResult).toEqual({
			type: "cancelled",
			reason: "outcome_aborted",
		});
		expect(commandPlan.commands).toEqual([
			{
				type: "delete_navigation",
				targetUrl: entry.targetUrl,
				reason: "outcome_aborted",
			},
		]);
	});
});

describe("navigation pass rejection plans", () => {
	it("plans delete+report for owned rejected passes", () => {
		const ownedEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/rejected",
		});
		const executionPlan = decideNavigationPassRejectionExecutionPlan({
			targetUrl: ownedEntry.targetUrl,
			candidateEntry: ownedEntry,
			expectedOperationID: ownedEntry.operationID,
		});
		expect(executionPlan).toEqual({
			type: "deleteAndReport",
			targetUrl: ownedEntry.targetUrl,
			ownedEntry,
		});
		expect(
			buildNavigationPassRejectionRuntimeCommandPlan({
				executionPlan,
			}),
		).toEqual({
			commands: [
				{
					type: "delete_navigation",
					targetUrl: ownedEntry.targetUrl,
					reason: "navigate_promise_rejected",
				},
			],
			report: {
				targetUrl: ownedEntry.targetUrl,
				ownedEntry,
			},
		});
	});

	it("plans report-only for stale or missing ownership", () => {
		const staleEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/stale-rejected",
		});
		const staleExecutionPlan = decideNavigationPassRejectionExecutionPlan({
			targetUrl: staleEntry.targetUrl,
			candidateEntry: staleEntry,
			expectedOperationID: staleEntry.operationID + 1,
		});
		expect(staleExecutionPlan).toEqual({
			type: "report",
			targetUrl: staleEntry.targetUrl,
		});
		expect(
			buildNavigationPassRejectionRuntimeCommandPlan({
				executionPlan: staleExecutionPlan,
			}),
		).toEqual({
			commands: [],
			report: {
				targetUrl: staleEntry.targetUrl,
				ownedEntry: undefined,
			},
		});

		const missingExecutionPlan = decideNavigationPassRejectionExecutionPlan(
			{
				targetUrl: "http://localhost:3000/missing",
				candidateEntry: undefined,
				expectedOperationID: 42,
			},
		);
		expect(missingExecutionPlan).toEqual({
			type: "report",
			targetUrl: "http://localhost:3000/missing",
		});
	});
});

describe("navigation pass reducer transitions", () => {
	it("reduces resolved events into resolved command plans", () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/reducer-target",
		});
		const redirectOutcome = createRedirectOutcome();
		const transition = reduceNavigationPassEvent({
			state: {
				targetUrl: entry.targetUrl,
				currentHref: "http://localhost:3000/current",
				entry,
				expectedOperationID: entry.operationID,
			},
			event: {
				type: "outcome_resolved",
				outcome: redirectOutcome,
				navigationProps: {
					href: "/reducer-target",
					navigationType: "userNavigation",
				},
			},
		});

		expect(transition.type).toBe("resolved");
		expect(transition.commandPlan.terminalResult).toEqual({
			type: "committed_from_redirect",
		});
		expect(
			transition.commandPlan.commands.map((command) => command.type),
		).toEqual([
			"sync_redirect_build_id",
			"delete_navigation",
			"effectuate_redirect",
		]);
	});

	it("reduces rejected events into rejected delete-or-report plans", () => {
		const ownedEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/reducer-rejected",
		});

		const ownedTransition = reduceNavigationPassEvent({
			state: {
				targetUrl: ownedEntry.targetUrl,
				currentHref: "http://localhost:3000/current",
				entry: ownedEntry,
				expectedOperationID: ownedEntry.operationID,
			},
			event: {
				type: "outcome_rejected",
			},
		});
		expect(ownedTransition.type).toBe("rejected");
		expect(ownedTransition.commandPlan.commands).toEqual([
			{
				type: "delete_navigation",
				targetUrl: ownedEntry.targetUrl,
				reason: "navigate_promise_rejected",
			},
		]);
		expect(ownedTransition.commandPlan.report).toEqual({
			targetUrl: ownedEntry.targetUrl,
			ownedEntry,
		});

		const staleTransition = reduceNavigationPassEvent({
			state: {
				targetUrl: ownedEntry.targetUrl,
				currentHref: "http://localhost:3000/current",
				entry: ownedEntry,
				expectedOperationID: ownedEntry.operationID + 1,
			},
			event: {
				type: "outcome_rejected",
			},
		});
		expect(staleTransition.type).toBe("rejected");
		expect(staleTransition.commandPlan.commands).toEqual([]);
		expect(staleTransition.commandPlan.report).toEqual({
			targetUrl: ownedEntry.targetUrl,
			ownedEntry: undefined,
		});
	});
});

describe("successful outcome stage plans", () => {
	it("classifies lifecycle state from ownership/prefetch/staleness in one seam", () => {
		const currentEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		expect(
			resolveNavigationEntryLifecycleState({
				entry: currentEntry,
				isCurrentEntry: false,
				currentHref: "http://localhost:3000/current",
			}),
		).toBe("non_current");

		const idlePrefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		expect(
			resolveNavigationEntryLifecycleState({
				entry: idlePrefetchEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}),
		).toBe("idle_prefetch");

		const staleRevalidationEntry = createEntry({
			type: "revalidation",
			intent: "revalidate",
			originUrl: "http://localhost:3000/origin",
		});
		expect(
			resolveNavigationEntryLifecycleState({
				entry: staleRevalidationEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/other",
			}),
		).toBe("stale_revalidation");

		expect(
			resolveNavigationEntryLifecycleState({
				entry: currentEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}),
		).toBe("current_fresh");
	});

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
		expect(
			buildSuccessfulNavigationPreWaitingRuntimeCommandPlan({
				executionPlan: plan,
			}),
		).toEqual({
			shouldStop: true,
			commands: [],
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
		expect(
			buildSuccessfulNavigationPreWaitingRuntimeCommandPlan({
				executionPlan: plan,
			}),
		).toEqual({
			shouldStop: true,
			commands: [
				{
					type: "delete_navigation",
					targetUrl: entry.targetUrl,
					reason: "stale_revalidation_pre_waiting",
				},
			],
		});
	});

	it("pre-waiting continues for idle prefetch entries that remain current", () => {
		const entry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		const plan = decideSuccessfulNavigationPreWaitingExecutionPlan({
			entry,
			isCurrentEntry: true,
			currentHref: "http://localhost:3000/current",
		});
		expect(plan).toEqual({
			type: "continue",
			reason: "entry_current_and_fresh",
		});
		expect(
			buildSuccessfulNavigationPreWaitingRuntimeCommandPlan({
				executionPlan: plan,
			}),
		).toEqual({
			shouldStop: false,
			commands: [{ type: "transition_to_waiting" }],
		});
	});

	it("post-waiting command planner reflects ownership stop/continue states", () => {
		const stopExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "post_waiting",
				entry: createEntry({
					type: "userNavigation",
					intent: "navigate",
				}),
				isCurrentEntry: false,
				currentHref: "http://localhost:3000/current",
			}).plan;
		expect(
			buildSuccessfulNavigationPostWaitingRuntimeCommandPlan({
				executionPlan: stopExecutionPlan,
			}),
		).toEqual({
			shouldStop: true,
			commands: [],
		});

		const continueExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "post_waiting",
				entry: createEntry({
					type: "userNavigation",
					intent: "navigate",
				}),
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			}).plan;
		expect(
			buildSuccessfulNavigationPostWaitingRuntimeCommandPlan({
				executionPlan: continueExecutionPlan,
			}),
		).toEqual({
			shouldStop: false,
			commands: [],
		});
	});

	it("post-asset renders only for current non-prefetch fresh entries", () => {
		const activeEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const renderExecutionPlan =
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: activeEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			});
		expect(renderExecutionPlan).toEqual({
			type: "render",
			reason: "post_asset_render",
		});
		expect(
			buildSuccessfulNavigationPostAssetRuntimeCommandPlan({
				executionPlan: renderExecutionPlan,
				shouldSyncBuildIDAfterAssetWaitIfNotStopped: true,
			}),
		).toEqual({
			shouldStop: false,
			commands: [
				{ type: "sync_build_id_from_response" },
				{ type: "render_navigation" },
			],
		});

		const prefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		const completeWithoutRenderExecutionPlan =
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: prefetchEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			});
		expect(completeWithoutRenderExecutionPlan).toEqual({
			type: "completeWithoutRender",
			reason: "post_asset_idle_prefetch",
		});
		expect(
			buildSuccessfulNavigationPostAssetRuntimeCommandPlan({
				executionPlan: completeWithoutRenderExecutionPlan,
				shouldSyncBuildIDAfterAssetWaitIfNotStopped: false,
			}),
		).toEqual({
			shouldStop: false,
			commands: [{ type: "transition_to_complete" }],
		});
	});

	it("post-asset stops stale revalidation entries after waiting", () => {
		const staleRevalidationEntry = createEntry({
			type: "revalidation",
			intent: "revalidate",
			originUrl: "http://localhost:3000/origin",
		});

		const stopExecutionPlan =
			decideSuccessfulNavigationPostAssetExecutionPlan({
				entry: staleRevalidationEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/other",
			});
		expect(stopExecutionPlan).toEqual({
			type: "stop",
			reason: "post_asset_stale_revalidation",
		});
		expect(
			buildSuccessfulNavigationPostAssetRuntimeCommandPlan({
				executionPlan: stopExecutionPlan,
				shouldSyncBuildIDAfterAssetWaitIfNotStopped: true,
			}),
		).toEqual({
			shouldStop: true,
			commands: [],
		});
	});

	it("decides successful-navigation cleanup through one explicit cleanup seam", () => {
		const activeEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/cleanup-target",
		});
		const deleteExecutionPlan =
			decideSuccessfulNavigationCleanupExecutionPlan({
				entry: activeEntry,
				isCurrentEntry: true,
			});
		expect(deleteExecutionPlan).toEqual({
			type: "deleteNavigation",
			targetUrl: "http://localhost:3000/cleanup-target",
			reason: "successful_navigation_cleanup",
		});
		expect(
			buildSuccessfulNavigationCleanupRuntimeCommandPlan({
				executionPlan: deleteExecutionPlan,
			}),
		).toEqual({
			shouldStop: false,
			commands: [
				{
					type: "delete_navigation",
					targetUrl: "http://localhost:3000/cleanup-target",
					reason: "successful_navigation_cleanup",
				},
			],
		});

		const idlePrefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
			targetUrl: "http://localhost:3000/prefetch-cleanup-target",
		});
		const skipIdlePrefetchExecutionPlan =
			decideSuccessfulNavigationCleanupExecutionPlan({
				entry: idlePrefetchEntry,
				isCurrentEntry: true,
			});
		expect(skipIdlePrefetchExecutionPlan).toEqual({
			type: "skip",
			reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
		});
		expect(
			buildSuccessfulNavigationCleanupRuntimeCommandPlan({
				executionPlan: skipIdlePrefetchExecutionPlan,
			}),
		).toEqual({
			shouldStop: false,
			commands: [],
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

	it("emits checkpoint-specific plans through a single planner seam", () => {
		const currentEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/current",
		});

		const preWaitingPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "pre_waiting",
				entry: currentEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			});
		expect(preWaitingPlan).toEqual({
			checkpoint: "pre_waiting",
			plan: {
				type: "continue",
				reason: "entry_current_and_fresh",
			},
		});

		const postWaitingPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "post_waiting",
				entry: currentEntry,
				isCurrentEntry: false,
				currentHref: "http://localhost:3000/current",
			});
		expect(postWaitingPlan).toEqual({
			checkpoint: "post_waiting",
			plan: {
				type: "stop",
				reason: "post_waiting_non_current_entry",
			},
		});

		const postAssetPlan = decideSuccessfulNavigationCheckpointExecutionPlan(
			{
				checkpoint: "post_asset",
				entry: currentEntry,
				isCurrentEntry: true,
				currentHref: "http://localhost:3000/current",
			},
		);
		expect(postAssetPlan).toEqual({
			checkpoint: "post_asset",
			plan: {
				type: "render",
				reason: "post_asset_render",
			},
		});

		const cleanupPlan = decideSuccessfulNavigationCheckpointExecutionPlan({
			checkpoint: "cleanup",
			entry: currentEntry,
			isCurrentEntry: true,
		});
		expect(cleanupPlan).toEqual({
			checkpoint: "cleanup",
			plan: {
				type: "deleteNavigation",
				targetUrl: currentEntry.targetUrl,
				reason: "successful_navigation_cleanup",
			},
		});
	});

	it("reduces successful-navigation checkpoint events through one reducer seam", () => {
		const currentEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/current",
		});
		expect(
			reduceSuccessfulNavigationCheckpointEvent({
				event: {
					checkpoint: "pre_waiting",
					entry: currentEntry,
					isCurrentEntry: true,
					currentHref: "http://localhost:3000/current",
				},
			}),
		).toEqual({
			shouldStop: false,
			commands: [{ type: "transition_to_waiting" }],
		});

		expect(
			reduceSuccessfulNavigationCheckpointEvent({
				event: {
					checkpoint: "post_waiting",
					entry: currentEntry,
					isCurrentEntry: false,
					currentHref: "http://localhost:3000/current",
				},
			}),
		).toEqual({
			shouldStop: true,
			commands: [],
		});

		expect(
			reduceSuccessfulNavigationCheckpointEvent({
				event: {
					checkpoint: "pre_asset_wait",
					shouldSyncBuildIDBeforeAssetWait: true,
				},
			}),
		).toEqual({
			shouldStop: false,
			commands: [{ type: "sync_build_id_from_response" }],
		});

		expect(
			reduceSuccessfulNavigationCheckpointEvent({
				event: {
					checkpoint: "post_asset",
					entry: currentEntry,
					isCurrentEntry: true,
					currentHref: "http://localhost:3000/current",
					shouldSyncBuildIDAfterAssetWaitIfNotStopped: true,
				},
			}),
		).toEqual({
			shouldStop: false,
			commands: [
				{ type: "sync_build_id_from_response" },
				{ type: "render_navigation" },
			],
		});

		expect(
			reduceSuccessfulNavigationCheckpointEvent({
				event: {
					checkpoint: "cleanup",
					entry: currentEntry,
					isCurrentEntry: true,
				},
			}),
		).toEqual({
			shouldStop: false,
			commands: [
				{
					type: "delete_navigation",
					targetUrl: currentEntry.targetUrl,
					reason: "successful_navigation_cleanup",
				},
			],
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
			resolveSuccessfulEntryBuildIDSyncPolicy({
				entry: idlePrefetchEntry,
			}),
		).toEqual({
			shouldSyncBuildIDBeforeAssetWait: true,
			shouldSyncBuildIDAfterAssetWaitIfNotStopped: false,
		});

		const navigationEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		expect(
			resolveSuccessfulEntryBuildIDSyncPolicy({
				entry: navigationEntry,
			}),
		).toEqual({
			shouldSyncBuildIDBeforeAssetWait: false,
			shouldSyncBuildIDAfterAssetWaitIfNotStopped: true,
		});
		expect(
			buildSuccessfulNavigationPreAssetWaitRuntimeCommandPlan({
				shouldSyncBuildIDBeforeAssetWait: true,
			}),
		).toEqual({
			shouldStop: false,
			commands: [{ type: "sync_build_id_from_response" }],
		});
		expect(
			buildSuccessfulNavigationPreAssetWaitRuntimeCommandPlan({
				shouldSyncBuildIDBeforeAssetWait: false,
			}),
		).toEqual({
			shouldStop: false,
			commands: [],
		});
	});
});

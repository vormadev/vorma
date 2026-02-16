import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import { hasSameNavigationTarget } from "../../platform/url.ts";
import {
	beginNavigation as executeBeginNavigation,
	createNavigationControls,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { resolveBeginNavigationTargetURL } from "./begin_navigation_state_machine.ts";
import { fetchRouteData } from "./fetch_route_data.ts";
import {
	buildNavigationsMapFromNavigationLanes,
	clearRuntimeLanes,
	computeNavigationStatus,
	createRuntimeLanes,
	createStatusSignaler,
	deleteNavigationFromNavigationLanes,
	findNavigationEntryInNavigationLanes,
	getNavigationsSizeFromNavigationLanes,
	transitionNavigationPhaseInNavigationLanes,
	type NavigationLanes,
} from "./runtime_slots.ts";
import {
	handleNavigationOutcomeWithInternalResult,
	processSuccessfulNavigationRuntime,
} from "./runtime_navigation_outcome.ts";
import { toPublicNavigateResult } from "./runtime_navigation_outcome_state_machine.ts";
import { buildNavigationEntriesByOperationIDFromNavigationLanes } from "./runtime_state_machine.ts";
import { createDeterministicRevalidationLane } from "./runtime_revalidation_lane.ts";
import {
	buildNavigationEntriesBeforeClearAll,
	createNavigationLifecycleRuntime,
} from "./runtime_lifecycle_runtime.ts";
import { executeSubmitRuntime } from "./runtime_submit.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
	NavigationStateManager,
	SubmitOptions,
	SubmissionEntry,
} from "./types.ts";
import { hasNavigationControlPromiseOwnership } from "./types.ts";

export {
	deleteNavigationFromNavigationLanes,
	findNavigationEntryInNavigationLanes,
	transitionNavigationPhaseInNavigationLanes,
};
export type { NavigationLanes as NavigationSlots };

export type CreateNavigationRuntimeOptions = {
	onNavigationIntentResolved?: () => void;
};

export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	const lanes = createRuntimeLanes();
	let nextNavigationOperationID = 1;
	let nextSubmissionOperationID = 1;
	let scheduleStatusUpdate: () => void = () => {};

	const getActiveNavigation = (): NavigationEntry | null => lanes.active;
	const setActiveNavigation = (entry: NavigationEntry | null): void => {
		lanes.active = entry;
	};

	const getRevalidationNavigation = (): NavigationEntry | null =>
		lanes.revalidation;
	const setRevalidationNavigation = (entry: NavigationEntry | null): void => {
		lanes.revalidation = entry;
	};

	const navigationLifecycleRuntime = createNavigationLifecycleRuntime({
		lanes,
		getScheduleStatusUpdate: () => scheduleStatusUpdate,
	});
	const findNavigationEntry = navigationLifecycleRuntime.findNavigationEntry;
	const deleteNavigation = (props: {
		targetUrl: string;
		reason: string;
		causedByOperationID?: number | null;
	}): boolean =>
		navigationLifecycleRuntime.deleteNavigation({
			key: props.targetUrl,
			reason: props.reason,
			causedByOperationID: props.causedByOperationID ?? null,
		});

	const deterministicRevalidationLane = createDeterministicRevalidationLane({
		getCurrentHref: () => window.location.href,
		onInFlightTargetMismatch: () => {
			const revalidationNavigation = getRevalidationNavigation();
			if (
				!revalidationNavigation ||
				hasSameNavigationTarget({
					firstHref: revalidationNavigation.targetUrl,
					secondHref: window.location.href,
				})
			) {
				return;
			}

			revalidationNavigation.control.abortController?.abort();
			deleteNavigation({
				targetUrl: revalidationNavigation.targetUrl,
				reason: "revalidation_target_mismatch",
			});
		},
	});

	const removeNavigation = (key: string): void => {
		const entry = findNavigationEntry(key);
		if (!entry) {
			return;
		}
		entry.control.abortController?.abort();
		deleteNavigation({
			targetUrl: key,
			reason: "remove_navigation",
		});
	};

	const getNavigation = (key: string): NavigationEntry | undefined =>
		findNavigationEntry(key);
	const hasNavigation = (key: string): boolean =>
		getNavigation(key) !== undefined;
	const getNavigationsSize = (): number =>
		getNavigationsSizeFromNavigationLanes({
			lanes,
		});
	const getNavigations = (): Map<string, NavigationEntry> =>
		buildNavigationsMapFromNavigationLanes({
			lanes,
		});
	const transitionPhase = (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}): void => navigationLifecycleRuntime.transitionPhase(props);
	const clearNavigationsAndSubmissions = (): void =>
		clearRuntimeLanes({
			lanes,
			onStatusRelevantChange: scheduleStatusUpdate,
		});

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			lanes,
		});
	}

	const statusSignaler = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});
	scheduleStatusUpdate = statusSignaler.scheduleStatusUpdate;

	const prefetchNavigationsByTargetUrl = lanes.prefetch;

	const { createActiveNavigation, createPrefetch, createRevalidation } =
		createNavigationControls({
			fetchRouteData,
			getActiveNavigation,
			setActiveNavigation,
			prefetchNavigationsByTargetUrl,
			getRevalidationNavigation,
			setRevalidationNavigation,
			scheduleStatusUpdate,
			deleteNavigation: ({ targetUrl, reason }) =>
				deleteNavigation({
					targetUrl,
					reason,
				}),
			allocateNavigationOperationID: () => {
				const operationID = nextNavigationOperationID;
				nextNavigationOperationID += 1;
				return operationID;
			},
		});

	const beginNavigationContext: BeginNavigationContext = {
		getActiveNavigation,
		setActiveNavigation,
		getRevalidationNavigation,
		setRevalidationNavigation,
		prefetchNavigationsByTargetUrl,
		scheduleStatusUpdate,
		createActiveNavigation,
		createPrefetch,
		createRevalidation,
	};

	const processSuccessfulNavigation = async (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> =>
		processSuccessfulNavigationRuntime(
			{
				transitionPhase: ({ targetUrl, phase, reason }): void =>
					transitionPhase({
						targetUrl,
						phase,
						reason,
					}),
				findNavigationEntry,
				deleteNavigation: ({ targetUrl, reason }) =>
					deleteNavigation({
						targetUrl,
						reason,
					}),
			},
			outcome,
			entry,
		);

	const beginNavigation = (props: NavigateProps) => {
		const beforeEntriesByOperationID =
			buildNavigationEntriesByOperationIDFromNavigationLanes({
				lanes,
			});
		const targetUrl = resolveBeginNavigationTargetURL({
			navigationProps: props,
			currentHref: window.location.href,
		});
		const control = executeBeginNavigation(beginNavigationContext, props);
		navigationLifecycleRuntime.dispatchBeginNavigationArbitrated({
			navigationType: props.navigationType,
			targetUrl,
			beforeEntriesByOperationID,
		});
		return control;
	};

	const navigateSinglePass = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		const control = beginNavigation(props);

		try {
			const outcome = await control.promise;
			const internalResult =
				await handleNavigationOutcomeWithInternalResult({
					findNavigationEntry,
					deleteNavigation: ({ targetUrl, reason }) =>
						deleteNavigation({
							targetUrl,
							reason,
						}),
					processSuccessfulNavigation,
					onNavigationIntentResolved,
					navigationProps: props,
					outcome,
					controlPromise: control.promise,
				});
			return toPublicNavigateResult({
				internalResult,
			});
		} catch {
			const targetUrl = resolveAbsoluteHref({ href: props.href });
			const candidateEntry = findNavigationEntry(targetUrl);
			const ownedEntry = hasNavigationControlPromiseOwnership(
				candidateEntry,
				control.promise,
			)
				? candidateEntry
				: undefined;
			if (ownedEntry) {
				deleteNavigation({
					targetUrl,
					reason: "navigate_promise_rejected",
				});
			}
			navigationLifecycleRuntime.dispatchNavigationFailure({
				targetUrl,
				entry: ownedEntry,
				reason: "navigate_promise_rejected",
			});
			return toPublicNavigateResult({
				internalResult: {
					type: "failed",
					reason: "navigate_promise_rejected",
				},
			});
		}
	};

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		if (props.navigationType !== "revalidation") {
			deterministicRevalidationLane.clearQueuedTrailingRequest();
			return navigateSinglePass(props);
		}

		return deterministicRevalidationLane.runRevalidation({
			navigateSinglePass,
		});
	};

	const submit = <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	): Promise<
		{ success: true; data: T } | { success: false; error: string }
	> =>
		executeSubmitRuntime(
			{
				submissions: lanes.submissions,
				scheduleStatusUpdate,
				allocateSubmissionOperationID: () => {
					const operationID = nextSubmissionOperationID;
					nextSubmissionOperationID += 1;
					return operationID;
				},
				onSubmissionStateTransition: ({
					submissionEntry,
					fromState,
					toState,
					reason,
					causedByOperationID,
				}) => {
					navigationLifecycleRuntime.dispatchSubmissionStateTransition(
						{
							submissionEntry,
							targetUrl: window.location.href,
							fromState,
							toState,
							reason,
							causedByOperationID,
						},
					);
				},
				navigate,
			},
			url,
			requestInit,
			options,
		);

	function clearAll(): void {
		const entriesBeforeClearAll = buildNavigationEntriesBeforeClearAll({
			lanes,
		});
		const submissionsBeforeClearAll = [...lanes.submissions.values()];

		deterministicRevalidationLane.reset();
		clearNavigationsAndSubmissions();

		navigationLifecycleRuntime.dispatchClearAll({
			navigationEntries: entriesBeforeClearAll,
			submissionEntries: submissionsBeforeClearAll,
			targetUrl: window.location.href,
		});
	}

	return {
		_submissions: lanes.submissions,
		navigate,
		beginNavigation,
		processSuccessfulNavigation,
		submit,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
		getStatus,
		getDebugJournal: navigationLifecycleRuntime.getDebugJournal,
		clearDebugJournal: navigationLifecycleRuntime.clearDebugJournal,
		clearAll,
	};
}

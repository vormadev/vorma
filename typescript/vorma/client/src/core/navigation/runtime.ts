import {
	dispatchRouteChangeEvent,
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import { HistoryManager } from "../../platform/history.ts";
import { saveScrollState } from "../../platform/scroll.ts";
import {
	classifyNavigationTargetAgainstCurrentLocation,
	hasSameNavigationTarget,
	hashFragmentFromHref,
	isSameDocumentLocation,
} from "../../platform/url.ts";
import {
	createNavigationControls,
	beginNavigation as executeBeginNavigation,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { resolveBeginNavigationTargetURL } from "./begin_navigation_state_machine.ts";
import { fetchRouteData } from "./fetch_route_data_server.ts";
import { createNavigationLifecycleRuntime } from "./runtime_lifecycle_runtime.ts";
import { executeNavigationSinglePass } from "./runtime_navigation_pass_runtime.ts";
import {
	processSuccessfulNavigationRuntime,
	syncBuildIDFromResponse,
	type ProcessSuccessfulNavigationContext,
} from "./runtime_navigation_successful_runtime.ts";
import { createDeterministicRevalidationLane } from "./runtime_revalidation_lane.ts";
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
import { buildNavigationEntriesByOperationIDFromNavigationLanes } from "./runtime_state_machine.ts";
import { executeSubmitRuntime } from "./runtime_submit.ts";
import {
	type NavigateProps,
	type NavigationEntry,
	type NavigationOutcome,
	type NavigationStateManager,
	type SubmitOptions,
	type SubmitResult,
} from "./types.ts";

export {
	handleNavigationOutcome,
	handleNavigationOutcomeWithInternalResult,
} from "./runtime_navigation_pass_runtime.ts";
export {
	createDeterministicRevalidationLane,
	deleteNavigationFromNavigationLanes,
	findNavigationEntryInNavigationLanes,
	processSuccessfulNavigationRuntime,
	syncBuildIDFromResponse,
	transitionNavigationPhaseInNavigationLanes,
};
export type { NavigationLanes, ProcessSuccessfulNavigationContext };

export type CreateNavigationRuntimeOptions = {
	// Called after a navigate/revalidate intent commits successfully.
	onNavigationIntentResolved?: () => void;
};

type SameDocumentNoFetchNavigationDecision =
	| "none"
	| "same-document-noop"
	| "hash-change";

// Same-document navigation classification is shared with link handling to keep
// no-fetch behavior consistent across link and programmatic entry points.
function resolveSameDocumentNoFetchNavigationDecision(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
	currentHref: string;
}): SameDocumentNoFetchNavigationDecision {
	const { navigationProps, targetUrl, currentHref } = props;
	if (
		navigationProps.navigationType === "prefetch" ||
		navigationProps.navigationType === "revalidation"
	) {
		return "none";
	}

	const targetClassification = classifyNavigationTargetAgainstCurrentLocation(
		{
			targetHref: targetUrl,
			currentHref,
		},
	);
	if (targetClassification === "hash-change") {
		return "hash-change";
	}
	if (targetClassification === "same-document-noop") {
		return "same-document-noop";
	}

	return "none";
}

function commitSameDocumentHashNavigationWithoutServerFetch(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
}): { didNavigate: boolean } {
	const { navigationProps, targetUrl } = props;
	const history = HistoryManager.getInstance();
	saveScrollState();
	const isSameLocation = isSameDocumentLocation({
		targetHref: targetUrl,
		currentHref: window.location.href,
	});
	if (!isSameLocation && !navigationProps.replace) {
		history.push(targetUrl, navigationProps.state);
	} else {
		history.replace(targetUrl, navigationProps.state);
	}

	const hash = hashFragmentFromHref(targetUrl);
	dispatchRouteChangeEvent({
		__scrollState: hash
			? { hash }
			: navigationProps.scrollToTop !== false
				? { x: 0, y: 0 }
				: undefined,
	});

	return { didNavigate: true };
}

/**
 * Creates the client navigation runtime that coordinates active navigation,
 * prefetch, revalidation, and submissions over shared slot state.
 */
export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	const lanes = createRuntimeLanes();
	let nextNavigationOperationID = 1;
	let nextSubmissionOperationID = 1;

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			lanes,
		});
	}
	const scheduleStatusUpdate = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});

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
		scheduleStatusUpdate,
	});
	const findNavigationEntry = navigationLifecycleRuntime.findNavigationEntry;
	const deleteNavigation = navigationLifecycleRuntime.deleteNavigation;

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

	const removeNavigation = (targetUrl: string): void => {
		const entry = findNavigationEntry(targetUrl);
		if (!entry) {
			return;
		}
		entry.control.abortController?.abort();
		deleteNavigation({
			targetUrl,
			reason: "remove_navigation",
		});
	};

	const getNavigation = findNavigationEntry;
	const hasNavigation = (targetUrl: string): boolean =>
		findNavigationEntry(targetUrl) !== undefined;
	const getNavigationsSize = (): number =>
		getNavigationsSizeFromNavigationLanes({
			lanes,
		});
	const getNavigations = (): Map<string, NavigationEntry> =>
		buildNavigationsMapFromNavigationLanes({
			lanes,
		});
	const transitionPhase = navigationLifecycleRuntime.transitionPhase;

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
			deleteNavigation,
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
				transitionPhase,
				findNavigationEntry,
				deleteNavigation,
				onSuccessfulNavigationCommitted: ({
					entry: committedEntry,
				}): void => {
					if (
						committedEntry.intent === "navigate" ||
						committedEntry.intent === "revalidate"
					) {
						onNavigationIntentResolved?.();
					}
				},
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
	): Promise<{ didNavigate: boolean }> =>
		executeNavigationSinglePass({
			navigationProps: props,
			beginNavigation,
			findNavigationEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			onNavigationPromiseRejected: ({ targetUrl, ownedEntry }) => {
				navigationLifecycleRuntime.dispatchNavigationFailure({
					targetUrl,
					entry: ownedEntry,
					reason: "navigate_promise_rejected",
				});
			},
		});

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		// Revalidation lane sequencing only applies to revalidation requests.
		if (props.navigationType !== "revalidation") {
			deterministicRevalidationLane.clearQueuedTrailingRequest();

			const targetUrl = resolveBeginNavigationTargetURL({
				navigationProps: props,
				currentHref: window.location.href,
			});
			const sameDocumentDecision =
				resolveSameDocumentNoFetchNavigationDecision({
					navigationProps: props,
					targetUrl,
					currentHref: window.location.href,
				});
			if (sameDocumentDecision === "hash-change") {
				const noFetchNavigationResult =
					commitSameDocumentHashNavigationWithoutServerFetch({
						navigationProps: props,
						targetUrl,
					});
				return noFetchNavigationResult;
			}
			if (sameDocumentDecision === "same-document-noop") {
				return { didNavigate: false };
			}

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
	): Promise<SubmitResult<T>> =>
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
		const entriesBeforeClearAll = [
			...buildNavigationEntriesByOperationIDFromNavigationLanes({
				lanes,
			}).values(),
		];
		const submissionsBeforeClearAll = [...lanes.submissions.values()];

		deterministicRevalidationLane.reset();
		clearRuntimeLanes({
			lanes,
			onStatusRelevantChange: scheduleStatusUpdate,
		});

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

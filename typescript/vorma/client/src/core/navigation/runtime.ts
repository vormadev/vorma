import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import { hasSameNavigationTarget } from "../../platform/url.ts";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "../redirects.ts";
import {
	createNavigationControls,
	beginNavigation as executeBeginNavigation,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { resolveBeginNavigationTargetURL } from "./begin_navigation_state_machine.ts";
import { fetchRouteData } from "./fetch_route_data_server.ts";
import { createNavigationLifecycleRuntime } from "./runtime_lifecycle_runtime.ts";
import {
	decideNavigationOutcomeExecutionPlan,
	toPublicNavigateResult,
	type InternalNavigateResult,
} from "./runtime_navigation_outcome_state_machine.ts";
import {
	processSuccessfulNavigationRuntime,
	syncBuildIDFromResponse,
	type ProcessSuccessfulNavigationContext,
} from "./runtime_navigation_successful_runtime.ts";
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
	hasNavigationOperationOwnership,
	type NavigateProps,
	type NavigationEntry,
	type NavigationOutcome,
	type NavigationPhase,
	type NavigationStateManager,
	type SubmitOptions,
} from "./types.ts";

export {
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

type RevalidationNavigateResult = Promise<{ didNavigate: boolean }>;

type RevalidationLaneState = {
	inFlightPromise: RevalidationNavigateResult | null;
	inFlightTargetUrl: string | null;
	isTrailingEligible: boolean;
	shouldRunTrailingPass: boolean;
	trailingPromise: RevalidationNavigateResult | null;
	resolveTrailingPromise: ((result: { didNavigate: boolean }) => void) | null;
};

export type DeterministicRevalidationLane = {
	runRevalidation: (props: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}) => RevalidationNavigateResult;
	clearQueuedTrailingRequest: () => void;
	reset: () => void;
};

export function createDeterministicRevalidationLane(props: {
	getCurrentHref: () => string;
	onInFlightTargetMismatch: () => void;
}): DeterministicRevalidationLane {
	const state: RevalidationLaneState = {
		inFlightPromise: null,
		inFlightTargetUrl: null,
		isTrailingEligible: false,
		shouldRunTrailingPass: false,
		trailingPromise: null,
		resolveTrailingPromise: null,
	};

	function resolveAndClearTrailingPromise(props?: {
		result?: { didNavigate: boolean };
	}): void {
		const result = props?.result || { didNavigate: false };
		state.resolveTrailingPromise?.(result);
		state.trailingPromise = null;
		state.resolveTrailingPromise = null;
	}

	function clearQueuedTrailingRequest(): void {
		state.shouldRunTrailingPass = false;
		resolveAndClearTrailingPromise();
	}

	function startPass(startNavigateProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const { navigateSinglePass } = startNavigateProps;
		const revalidationHref = props.getCurrentHref();
		const revalidationProps: NavigateProps = {
			href: revalidationHref,
			navigationType: "revalidation",
		};
		const passPromise = navigateSinglePass(revalidationProps);
		state.inFlightPromise = passPromise;
		state.inFlightTargetUrl = resolveAbsoluteHref({
			href: revalidationHref,
		});
		state.isTrailingEligible = false;

		queueMicrotask(() => {
			if (state.inFlightPromise === passPromise) {
				state.isTrailingEligible = true;
			}
		});

		void passPromise.finally(() => {
			if (state.inFlightPromise !== passPromise) {
				return;
			}

			state.inFlightPromise = null;
			state.inFlightTargetUrl = null;
			state.isTrailingEligible = false;

			if (!state.shouldRunTrailingPass) {
				clearQueuedTrailingRequest();
				return;
			}

			state.shouldRunTrailingPass = false;
			const resolveTrailingPromise = state.resolveTrailingPromise;
			state.trailingPromise = null;
			state.resolveTrailingPromise = null;

			void startPass({ navigateSinglePass }).then(
				(result) => resolveTrailingPromise?.(result),
				() => resolveTrailingPromise?.({ didNavigate: false }),
			);
		});

		return passPromise;
	}

	function scheduleTrailingPass(): RevalidationNavigateResult {
		state.shouldRunTrailingPass = true;

		if (state.trailingPromise) {
			return state.trailingPromise;
		}

		state.trailingPromise = new Promise((resolve) => {
			state.resolveTrailingPromise = resolve;
		});
		return state.trailingPromise;
	}

	function hasInFlightTargetMismatch(): boolean {
		if (!state.inFlightTargetUrl) {
			return false;
		}

		return !hasSameNavigationTarget({
			firstHref: state.inFlightTargetUrl,
			secondHref: props.getCurrentHref(),
		});
	}

	function runRevalidation(runProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const { navigateSinglePass } = runProps;
		if (!state.inFlightPromise) {
			return startPass({ navigateSinglePass });
		}

		if (hasInFlightTargetMismatch()) {
			props.onInFlightTargetMismatch();
			clearQueuedTrailingRequest();
			return startPass({ navigateSinglePass });
		}

		if (!state.isTrailingEligible) {
			return state.inFlightPromise;
		}

		return scheduleTrailingPass();
	}

	function reset(): void {
		state.inFlightPromise = null;
		state.inFlightTargetUrl = null;
		state.isTrailingEligible = false;
		clearQueuedTrailingRequest();
	}

	return {
		runRevalidation,
		clearQueuedTrailingRequest,
		reset,
	};
}

type HandleNavigationOutcomeProps = {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	expectedOperationID: number | undefined;
};

/**
 * Applies a completed navigation outcome and returns whether navigation committed.
 */
export async function handleNavigationOutcome(
	props: HandleNavigationOutcomeProps,
): Promise<{ didNavigate: boolean }> {
	const internalResult =
		await handleNavigationOutcomeWithInternalResult(props);

	return toPublicNavigateResult({
		internalResult,
	});
}

/**
 * Internal outcome handler that returns a richer result for runtime state machines.
 */
export async function handleNavigationOutcomeWithInternalResult(
	props: HandleNavigationOutcomeProps,
): Promise<InternalNavigateResult> {
	const {
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		navigationProps,
		outcome,
		expectedOperationID,
	} = props;
	const targetUrl = resolveAbsoluteHref({ href: navigationProps.href });
	const entry = findNavigationEntry(targetUrl);
	const executionPlan = decideNavigationOutcomeExecutionPlan({
		outcome,
		targetUrl,
		entry,
		expectedOperationID,
		currentHref: window.location.href,
	});

	switch (executionPlan.type) {
		case "stop":
			return {
				type: "cancelled",
				reason: executionPlan.reason,
			};
		case "deleteAndStop":
			deleteNavigation({
				targetUrl: executionPlan.targetUrl,
				reason: executionPlan.reason,
			});
			return {
				type: "cancelled",
				reason: executionPlan.reason,
			};
		case "redirect": {
			syncBuildIDFromRedirectData(executionPlan.outcome.redirectData);
			deleteNavigation({
				targetUrl,
				reason: executionPlan.reason,
			});
			const redirectResult = await effectuateRedirectDataResult(
				executionPlan.outcome.redirectData,
				navigationProps.redirectCount || 0,
				{
					...navigationProps,
					href: executionPlan.entry.targetUrl,
					navigationType: executionPlan.entry.type,
					scrollToTop: executionPlan.entry.scrollToTop,
					replace: executionPlan.entry.replace,
					state: executionPlan.entry.state,
				},
			);
			return {
				type: "committed",
				didNavigate: redirectResult?.status === "did",
			};
		}
		case "success":
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
	): Promise<{ didNavigate: boolean }> => {
		const control = beginNavigation(props);

		try {
			const outcome = await control.promise;
			return handleNavigationOutcome({
				findNavigationEntry,
				deleteNavigation,
				processSuccessfulNavigation,
				navigationProps: props,
				outcome,
				expectedOperationID: control.operationID,
			});
		} catch {
			const targetUrl = resolveAbsoluteHref({ href: props.href });
			const candidateEntry = findNavigationEntry(targetUrl);
			const ownedEntry = hasNavigationOperationOwnership({
				entry: candidateEntry,
				expectedOperationID: control.operationID,
			})
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
			return { didNavigate: false };
		}
	};

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		// Revalidation lane sequencing only applies to revalidation requests.
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

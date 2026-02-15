import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import {
	beginNavigation as executeBeginNavigation,
	createNavigationControls,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { fetchRouteData } from "./fetch_route_data.ts";
import {
	buildNavigationsMapFromSlots,
	clearSlotsAndSubmissions,
	computeNavigationStatus,
	createStatusSignaler,
	deleteNavigationFromSlots,
	findNavigationEntryInSlots,
	getNavigationsSizeFromSlots,
	transitionNavigationPhaseInSlots,
	type NavigationSlots,
} from "./runtime_slots.ts";
import {
	handleNavigationOutcome,
	processSuccessfulNavigationRuntime,
} from "./runtime_navigation_outcome.ts";
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

const REVALIDATION_COALESCE_MS = 8;

export {
	deleteNavigationFromSlots,
	findNavigationEntryInSlots,
	transitionNavigationPhaseInSlots,
};
export type { NavigationSlots };

export type CreateNavigationRuntimeOptions = {
	onNavigationIntentResolved?: () => void;
};

export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	const submissions = new Map<string | symbol, SubmissionEntry>();
	const slots: NavigationSlots = {
		activeNavigation: null,
		prefetchCache: new Map<string, NavigationEntry>(),
		pendingRevalidation: null,
	};
	let scheduleStatusUpdate: () => void = () => {};

	const getActiveNavigation = (): NavigationEntry | null =>
		slots.activeNavigation;
	const setActiveNavigation = (entry: NavigationEntry | null): void => {
		slots.activeNavigation = entry;
	};

	const getPendingRevalidation = (): NavigationEntry | null =>
		slots.pendingRevalidation;
	const setPendingRevalidation = (entry: NavigationEntry | null): void => {
		slots.pendingRevalidation = entry;
	};

	const findNavigationEntry = (
		targetUrl: string,
	): NavigationEntry | undefined =>
		findNavigationEntryInSlots(slots, targetUrl);

	const deleteNavigation = (key: string): boolean =>
		deleteNavigationFromSlots(slots, key, scheduleStatusUpdate);

	const removeNavigation = (key: string): void => {
		const entry = findNavigationEntry(key);
		if (!entry) {
			return;
		}
		entry.control.abortController?.abort();
		deleteNavigation(key);
	};

	const getNavigation = (key: string): NavigationEntry | undefined =>
		findNavigationEntry(key);
	const hasNavigation = (key: string): boolean =>
		getNavigation(key) !== undefined;
	const getNavigationsSize = (): number => getNavigationsSizeFromSlots(slots);
	const getNavigations = (): Map<string, NavigationEntry> =>
		buildNavigationsMapFromSlots(slots);
	const transitionPhase = (targetUrl: string, phase: NavigationPhase): void =>
		transitionNavigationPhaseInSlots(
			slots,
			targetUrl,
			phase,
			scheduleStatusUpdate,
		);
	const clearNavigationsAndSubmissions = (): void =>
		clearSlotsAndSubmissions(slots, submissions, scheduleStatusUpdate);

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			activeNavigation: slots.activeNavigation,
			pendingRevalidation: slots.pendingRevalidation,
			submissions,
		});
	}

	const statusSignaler = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});
	scheduleStatusUpdate = statusSignaler.scheduleStatusUpdate;

	const prefetchCache = slots.prefetchCache;

	const { createActiveNavigation, createPrefetch, createRevalidation } =
		createNavigationControls({
			fetchRouteData,
			getActiveNavigation,
			setActiveNavigation,
			prefetchCache,
			getPendingRevalidation,
			setPendingRevalidation,
			scheduleStatusUpdate,
			deleteNavigation: (key: string) => deleteNavigation(key),
		});

	const beginNavigationContext: BeginNavigationContext = {
		getActiveNavigation,
		setActiveNavigation,
		getPendingRevalidation,
		setPendingRevalidation,
		prefetchCache,
		scheduleStatusUpdate,
		revalidationCoalesceMS: REVALIDATION_COALESCE_MS,
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
				transitionPhase: (
					targetUrl: string,
					phase: NavigationPhase,
				): void => transitionPhase(targetUrl, phase),
				findNavigationEntry,
				deleteNavigation,
			},
			outcome,
			entry,
		);

	const beginNavigation = (props: NavigateProps) =>
		executeBeginNavigation(beginNavigationContext, props);

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		const control = beginNavigation(props);

		try {
			const outcome = await control.promise;
			return await handleNavigationOutcome({
				findNavigationEntry,
				deleteNavigation,
				processSuccessfulNavigation,
				onNavigationIntentResolved,
				navigationProps: props,
				outcome,
				controlPromise: control.promise,
			});
		} catch {
			const targetUrl = resolveAbsoluteHref(props.href);
			const entry = findNavigationEntry(targetUrl);
			if (hasNavigationControlPromiseOwnership(entry, control.promise)) {
				deleteNavigation(targetUrl);
			}
			return { didNavigate: false };
		}
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
				submissions,
				scheduleStatusUpdate,
				navigate,
			},
			url,
			requestInit,
			options,
		);

	function clearAll(): void {
		clearNavigationsAndSubmissions();
	}

	return {
		_submissions: submissions,
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
		clearAll,
	};
}

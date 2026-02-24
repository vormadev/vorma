import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "../redirects.ts";
import {
	decideNavigationOutcomeExecutionPlan,
	toPublicNavigateResult,
	type InternalNavigateResult,
} from "./runtime_navigation_outcome_state_machine.ts";
import {
	hasNavigationOperationOwnership,
	type NavigateProps,
	type NavigationControl,
	type NavigationEntry,
	type NavigationOutcome,
} from "./types.ts";

export type HandleNavigationOutcomeProps = {
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

export type ExecuteNavigationSinglePassProps = {
	navigationProps: NavigateProps;
	beginNavigation: (props: NavigateProps) => NavigationControl;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationPromiseRejected: (props: {
		targetUrl: string;
		ownedEntry: NavigationEntry | undefined;
	}) => void;
};

/**
 * Runs a single navigation pass and centralizes rejected-promise ownership cleanup.
 */
export async function executeNavigationSinglePass(
	props: ExecuteNavigationSinglePassProps,
): Promise<{ didNavigate: boolean }> {
	const {
		navigationProps,
		beginNavigation,
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationPromiseRejected,
	} = props;
	const control = beginNavigation(navigationProps);

	try {
		const outcome = await control.promise;
		return handleNavigationOutcome({
			findNavigationEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps,
			outcome,
			expectedOperationID: control.operationID,
		});
	} catch {
		const targetUrl = resolveAbsoluteHref({ href: navigationProps.href });
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
		onNavigationPromiseRejected({
			targetUrl,
			ownedEntry,
		});
		return { didNavigate: false };
	}
}

import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "../redirects.ts";
import {
	decideNavigationOutcomeExecutionPlan,
	toPublicNavigateResult,
	type InternalNavigateResult,
	type NavigationOutcomeExecutionPlan,
} from "./runtime_navigation_outcome_state_machine.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "./types.ts";

export {
	processSuccessfulNavigationRuntime,
	syncBuildIDFromResponse,
	type ProcessSuccessfulNavigationContext,
} from "./runtime_navigation_successful_runtime.ts";

export async function handleNavigationOutcome(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	expectedOperationID: number | undefined;
}): Promise<{ didNavigate: boolean }> {
	const internalResult =
		await handleNavigationOutcomeWithInternalResult(props);

	return toPublicNavigateResult({
		internalResult,
	});
}

export async function handleNavigationOutcomeWithInternalResult(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	expectedOperationID: number | undefined;
}): Promise<InternalNavigateResult> {
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

	const internalResult = await executeNavigationOutcomeExecutionPlan({
		executionPlan,
		targetUrl,
		deleteNavigation,
		processSuccessfulNavigation,
		navigationProps,
	});
	return internalResult;
}

async function executeNavigationOutcomeExecutionPlan(props: {
	executionPlan: NavigationOutcomeExecutionPlan;
	targetUrl: string;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	navigationProps: NavigateProps;
}): Promise<InternalNavigateResult> {
	switch (props.executionPlan.type) {
		case "stop":
			return {
				type: "cancelled",
				reason: props.executionPlan.reason,
			};
		case "deleteAndStop":
			props.deleteNavigation({
				targetUrl: props.executionPlan.targetUrl,
				reason: props.executionPlan.reason,
			});
			return {
				type: "cancelled",
				reason: props.executionPlan.reason,
			};
		case "redirect": {
			syncBuildIDFromRedirectData(
				props.executionPlan.outcome.redirectData,
			);
			props.deleteNavigation({
				targetUrl: props.targetUrl,
				reason: props.executionPlan.reason,
			});
			const redirectResult = await effectuateRedirectDataResult(
				props.executionPlan.outcome.redirectData,
				props.navigationProps.redirectCount || 0,
				props.navigationProps,
			);
			return {
				type: "committed",
				didNavigate: redirectResult?.status === "did",
			};
		}
		case "success":
			await props.processSuccessfulNavigation(
				props.executionPlan.outcome,
				props.executionPlan.entry,
			);
			return {
				type: "committed",
				didNavigate: props.executionPlan.didNavigate,
			};
	}
}

import { dispatchBuildIDEvent } from "../../platform/events.ts";
import { hasSameDataTarget, resolveAbsoluteHref } from "../../platform/url.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import {
	effectuateRedirectDataResult,
	getBuildIDFromResponse,
	syncBuildIDFromRedirectData,
} from "../redirects.ts";
import {
	AssetManager,
	__reRenderApp,
	setClientLoadersState,
} from "../render_runtime.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";

export async function handleNavigationOutcome(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	controlPromise: Promise<NavigationOutcome>;
}): Promise<{ didNavigate: boolean }> {
	const {
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
		outcome,
		controlPromise,
	} = props;
	const targetUrl = resolveAbsoluteHref(navigationProps.href);
	const action = decideNavigationOutcomeAction({
		outcome,
		targetUrl,
		findNavigationEntry,
		controlPromise,
	});

	return executeNavigationOutcomeAction({
		action,
		targetUrl,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
	});
}

async function executeNavigationOutcomeAction(props: {
	action: NavigationOutcomeAction;
	targetUrl: string;
	deleteNavigation: (key: string) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
}): Promise<{ didNavigate: boolean }> {
	const {
		action,
		targetUrl,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
	} = props;

	switch (action.type) {
		case "deleteAndStop":
			deleteNavigation(action.targetUrl);
			return { didNavigate: false };
		case "stop":
			return { didNavigate: false };
		case "redirect":
			await handleRedirectOutcomeForEntry({
				entry: action.entry,
				outcome: action.outcome,
				deleteNavigation,
				targetUrl,
				navigationProps,
			});
			return { didNavigate: false };
		case "success":
			if (action.shouldResolveIntent) {
				onNavigationIntentResolved?.();
			}

			await processSuccessfulNavigation(action.outcome, action.entry);
			return { didNavigate: action.didNavigate };
	}
}

type NavigationOutcomeAction =
	| {
			type: "deleteAndStop";
			targetUrl: string;
	  }
	| {
			type: "stop";
	  }
	| {
			type: "redirect";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "redirect" }>;
	  }
	| {
			type: "success";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "success" }>;
			shouldResolveIntent: boolean;
			didNavigate: boolean;
	  };

function decideNavigationOutcomeAction(props: {
	outcome: NavigationOutcome;
	targetUrl: string;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	controlPromise: Promise<NavigationOutcome>;
}): NavigationOutcomeAction {
	const { outcome, targetUrl, findNavigationEntry, controlPromise } = props;

	const entry = findNavigationEntry(targetUrl);
	if (!entry) {
		return { type: "stop" };
	}

	if (
		!isNavigationOutcomeCurrentForEntry({
			entry,
			controlPromise,
		})
	) {
		return { type: "stop" };
	}

	if (outcome.type === "aborted") {
		return {
			type: "deleteAndStop",
			targetUrl,
		};
	}

	if (outcome.type === "redirect") {
		return {
			type: "redirect",
			entry,
			outcome,
		};
	}

	return {
		type: "success",
		entry,
		outcome,
		shouldResolveIntent: shouldResolveNavigationIntentForEntry(entry),
		didNavigate: !isIdlePrefetchEntry(entry),
	};
}

function isNavigationOutcomeCurrentForEntry(props: {
	entry: NavigationEntry;
	controlPromise: Promise<NavigationOutcome>;
}): boolean {
	const { entry, controlPromise } = props;
	return entry.control.promise === controlPromise;
}

function isIdlePrefetchEntry(entry: NavigationEntry): boolean {
	return entry.type === "prefetch" && entry.intent === "none";
}

function shouldResolveNavigationIntentForEntry(
	entry: NavigationEntry,
): boolean {
	return entry.intent === "navigate" || entry.intent === "revalidate";
}

function shouldIgnoreRedirectOutcomeForEntry(entry: NavigationEntry): boolean {
	return isStaleRevalidationEntry(entry) || isIdlePrefetchEntry(entry);
}

type RedirectOutcomeStep = "ignore" | "effectuate";

function getRedirectOutcomeStep(entry: NavigationEntry): RedirectOutcomeStep {
	return shouldIgnoreRedirectOutcomeForEntry(entry) ? "ignore" : "effectuate";
}

async function handleRedirectOutcomeForEntry(props: {
	entry: NavigationEntry;
	outcome: Extract<NavigationOutcome, { type: "redirect" }>;
	deleteNavigation: (key: string) => boolean;
	targetUrl: string;
	navigationProps: NavigateProps;
}): Promise<void> {
	const { entry, outcome, deleteNavigation, targetUrl, navigationProps } =
		props;

	switch (getRedirectOutcomeStep(entry)) {
		case "ignore":
			deleteNavigation(targetUrl);
			return;
		case "effectuate":
			syncBuildIDFromRedirectData(outcome.redirectData);
			deleteNavigation(targetUrl);
			await effectuateRedirectDataResult(
				outcome.redirectData,
				navigationProps.redirectCount || 0,
				navigationProps,
			);
			return;
	}
}

export type ProcessSuccessfulNavigationContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
};

function isCurrentNavigationEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): boolean {
	const { context, entry } = props;
	return context.findNavigationEntry(entry.targetUrl) === entry;
}

function transitionPhaseForCurrentEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
	phase: NavigationPhase;
}): void {
	const { context, entry, phase } = props;
	if (!isCurrentNavigationEntry({ context, entry })) {
		return;
	}

	context.transitionPhase(entry.targetUrl, phase);
}

function applyResponseArtifactsWhenBuildMatches(
	response: Response,
	json: GetRouteDataOutput,
): void {
	const currentBuildID = __vormaClientGlobal.get("buildID");
	const responseBuildID = getBuildIDFromResponse(response);

	if (responseBuildID !== currentBuildID) {
		return;
	}

	const clientModuleMap = __vormaClientGlobal.get("clientModuleMap") || {};
	const matchedPatterns = json.matchedPatterns || [];
	const importURLs = json.importURLs || [];
	const exportKeys = json.exportKeys || [];
	const errorExportKeys = json.errorExportKeys || [];

	for (let i = 0; i < matchedPatterns.length; i++) {
		const pattern = matchedPatterns[i];
		const importURL = importURLs[i];
		const exportKey = exportKeys[i];
		const errorExportKey = errorExportKeys[i];

		if (pattern && importURL) {
			clientModuleMap[pattern] = {
				importURL,
				exportKey: exportKey || "default",
				errorExportKey: errorExportKey || "",
			};
		}
	}

	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);

	if (json.cssBundles && json.cssBundles.length > 0) {
		AssetManager.applyCSS(json.cssBundles);
	}
}

export function syncBuildIDFromResponse(response: Response): void {
	const oldID = __vormaClientGlobal.get("buildID");
	const newID = getBuildIDFromResponse(response);
	if (!newID || newID === oldID) {
		return;
	}

	__vormaClientGlobal.set("buildID", newID);
	dispatchBuildIDEvent({ newID, oldID });
}

function isStaleRevalidationEntry(entry: NavigationEntry): boolean {
	return (
		entry.type === "revalidation" &&
		!hasSameDataTarget(window.location.href, entry.originUrl)
	);
}

async function waitForSuccessfulNavigationAssets(
	outcome: Extract<NavigationOutcome, { type: "success" }>,
): Promise<void> {
	const { waitFnPromise, cssBundlePromises } = outcome;

	const clientLoadersResult = await waitFnPromise;
	setClientLoadersState(clientLoadersResult);

	if (cssBundlePromises.length > 0) {
		try {
			await Promise.all(cssBundlePromises);
		} catch (error) {
			logError("Error preloading CSS bundles:", error);
		}
	}
}

function buildRunHistoryOptions(
	entry: NavigationEntry,
	props: Extract<NavigationOutcome, { type: "success" }>["props"],
) {
	if (entry.intent !== "navigate") {
		return undefined;
	}

	return {
		href: entry.targetUrl,
		scrollStateToRestore: props.scrollStateToRestore,
		replace: entry.replace || props.replace,
		scrollToTop: entry.scrollToTop,
		state: entry.state,
	};
}

async function renderSuccessfulNavigation(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	transitionPhaseForCurrentEntry({
		context,
		entry,
		phase: "rendering",
	});

	try {
		await __reRenderApp({
			json: outcome.json,
			navigationType: entry.type,
			runHistoryOptions: buildRunHistoryOptions(entry, outcome.props),
			onFinish: () => {
				transitionPhaseForCurrentEntry({
					context,
					entry,
					phase: "complete",
				});
			},
		});
	} catch (error) {
		transitionPhaseForCurrentEntry({
			context,
			entry,
			phase: "complete",
		});
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}

type SuccessfulNavigationPreWaitingAction =
	| "stopAndDelete"
	| "stop"
	| "continue";

function decideSuccessfulNavigationPreWaitingAction(
	context: ProcessSuccessfulNavigationContext,
	entry: NavigationEntry,
): SuccessfulNavigationPreWaitingAction {
	if (!isCurrentNavigationEntry({ context, entry })) {
		return "stop";
	}

	return isStaleRevalidationEntry(entry) ? "stopAndDelete" : "continue";
}

function executeSuccessfulNavigationPreWaitingAction(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
	action: SuccessfulNavigationPreWaitingAction;
}): { shouldStop: boolean } {
	const { context, entry, action } = props;
	switch (action) {
		case "stopAndDelete":
			context.deleteNavigation(entry.targetUrl);
			return { shouldStop: true };
		case "stop":
			return { shouldStop: true };
		case "continue":
			return { shouldStop: false };
	}
}

type SuccessfulNavigationPostWaitingAction = "stop" | "continue";

function decideSuccessfulNavigationPostWaitingAction(
	context: ProcessSuccessfulNavigationContext,
	entry: NavigationEntry,
): SuccessfulNavigationPostWaitingAction {
	return isCurrentNavigationEntry({ context, entry }) ? "continue" : "stop";
}

function executeSuccessfulNavigationPostWaitingAction(
	action: SuccessfulNavigationPostWaitingAction,
): { shouldStop: boolean } {
	return { shouldStop: action === "stop" };
}

type SuccessfulNavigationPostAssetAction =
	| "completeWithoutRender"
	| "stop"
	| "render";

function decideSuccessfulNavigationPostAssetAction(
	context: ProcessSuccessfulNavigationContext,
	entry: NavigationEntry,
): SuccessfulNavigationPostAssetAction {
	if (!isCurrentNavigationEntry({ context, entry })) {
		return "stop";
	}

	if (entry.intent === "none") {
		return "completeWithoutRender";
	}
	if (isStaleRevalidationEntry(entry)) {
		return "stop";
	}
	return "render";
}

async function executeSuccessfulNavigationPostAssetAction(props: {
	context: ProcessSuccessfulNavigationContext;
	outcome: Extract<NavigationOutcome, { type: "success" }>;
	entry: NavigationEntry;
	action: SuccessfulNavigationPostAssetAction;
}): Promise<void> {
	const { context, outcome, entry, action } = props;
	switch (action) {
		case "completeWithoutRender":
			transitionPhaseForCurrentEntry({
				context,
				entry,
				phase: "complete",
			});
			return;
		case "stop":
			return;
		case "render":
			await renderSuccessfulNavigation(context, outcome, entry);
			return;
	}
}

export async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	try {
		const { response, json } = outcome;

		const preWaitingAction = decideSuccessfulNavigationPreWaitingAction(
			context,
			entry,
		);
		if (
			executeSuccessfulNavigationPreWaitingAction({
				context,
				entry,
				action: preWaitingAction,
			}).shouldStop
		) {
			return;
		}

		applyResponseArtifactsWhenBuildMatches(response, json);

		transitionPhaseForCurrentEntry({
			context,
			entry,
			phase: "waiting",
		});

		const postWaitingAction = decideSuccessfulNavigationPostWaitingAction(
			context,
			entry,
		);
		if (
			executeSuccessfulNavigationPostWaitingAction(postWaitingAction)
				.shouldStop
		) {
			return;
		}

		syncBuildIDFromResponse(response);
		await waitForSuccessfulNavigationAssets(outcome);

		const postAssetAction = decideSuccessfulNavigationPostAssetAction(
			context,
			entry,
		);
		await executeSuccessfulNavigationPostAssetAction({
			context,
			outcome,
			entry,
			action: postAssetAction,
		});
	} finally {
		if (
			!isIdlePrefetchEntry(entry) &&
			isCurrentNavigationEntry({ context, entry })
		) {
			context.deleteNavigation(entry.targetUrl);
		}
	}
}

import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import { getIsGETRequest } from "vorma/kit/url";
import {
	dispatchBuildIDEvent,
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import { hasSameDataTarget } from "../../platform/url.ts";
import { __vormaClientGlobal, type GetRouteDataOutput } from "../../app/context.ts";
import { isAbortError, logError } from "../../platform/safety.ts";
import {
	effectuateRedirectDataResult,
	getBuildIDFromResponse,
	handleRedirects,
	syncBuildIDFromRedirectData,
	type RedirectData,
} from "../redirects.ts";
import {
	AssetManager,
	__reRenderApp,
	setClientLoadersState,
} from "../render_runtime.ts";
import {
	beginPrefetch as executeBeginPrefetch,
	beginRevalidation as executeBeginRevalidation,
	beginUserNavigation as executeBeginUserNavigation,
	createNavigationControls,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { fetchRouteData } from "./fetch_route_data.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
	NavigationPhase,
	NavigationStateManager,
	SubmitOptions,
	SubmissionEntry,
} from "./types.ts";

const REVALIDATION_COALESCE_MS = 8;

function computeNavigationStatus(props: {
	activeNavigation: NavigationEntry | null;
	pendingRevalidation: NavigationEntry | null;
	submissions: Map<string | symbol, SubmissionEntry>;
}): StatusEventDetail {
	const { activeNavigation, pendingRevalidation, submissions } = props;

	const isNavigating =
		activeNavigation !== null &&
		activeNavigation.intent === "navigate" &&
		activeNavigation.phase !== "complete";

	const isRevalidating =
		pendingRevalidation !== null &&
		pendingRevalidation.phase !== "complete";

	const isSubmitting = Array.from(submissions.values()).some(
		(x) => !x.skipGlobalLoadingIndicator,
	);

	return { isNavigating, isSubmitting, isRevalidating };
}

function createStatusSignaler(props: {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
}): { scheduleStatusUpdate: () => void } {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = props;
	let lastDispatchedStatus: StatusEventDetail | null = null;

	function dispatchStatusEventInternal(): void {
		const newStatus = getStatus();
		if (jsonDeepEquals(lastDispatchedStatus, newStatus)) {
			return;
		}
		lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}

	const scheduleStatusUpdate = debounce(() => {
		dispatchStatusEventInternal();
	}, debounceMS);

	return {
		scheduleStatusUpdate,
	};
}

export type NavigationSlots = {
	activeNavigation: NavigationEntry | null;
	prefetchCache: Map<string, NavigationEntry>;
	pendingRevalidation: NavigationEntry | null;
};

type NavigationBookkeeping = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	getPrefetchCache: () => Map<string, NavigationEntry>;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	removeNavigation: (key: string) => void;
	getNavigation: (key: string) => NavigationEntry | undefined;
	hasNavigation: (key: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	clearNavigationsAndSubmissions: (
		submissions: Map<string | symbol, SubmissionEntry>,
	) => void;
};

type CreateNavigationBookkeepingOptions = {
	onStatusRelevantChange: () => void;
};

export function findNavigationEntryInSlots(
	slots: NavigationSlots,
	targetUrl: string,
): NavigationEntry | undefined {
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, targetUrl))
	) {
		return slots.activeNavigation;
	}

	const prefetch = slots.prefetchCache.get(targetUrl);
	if (prefetch) {
		return prefetch;
	}
	for (const [url, entry] of slots.prefetchCache) {
		if (hasSameDataTarget(url, targetUrl)) {
			return entry;
		}
	}

	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, targetUrl))
	) {
		return slots.pendingRevalidation;
	}

	return undefined;
}

function getNavigationsSizeFromSlots(slots: NavigationSlots): number {
	let size = 0;
	if (slots.activeNavigation) size++;
	size += slots.prefetchCache.size;
	if (slots.pendingRevalidation) size++;
	return size;
}

function buildNavigationsMapFromSlots(
	slots: NavigationSlots,
): Map<string, NavigationEntry> {
	const map = new Map<string, NavigationEntry>();
	if (slots.activeNavigation) {
		map.set(slots.activeNavigation.targetUrl, slots.activeNavigation);
	}
	for (const [key, entry] of slots.prefetchCache) {
		map.set(key, entry);
	}
	if (slots.pendingRevalidation) {
		map.set(slots.pendingRevalidation.targetUrl, slots.pendingRevalidation);
	}
	return map;
}

function findMatchingPrefetchKey(
	slots: NavigationSlots,
	key: string,
): string | undefined {
	if (slots.prefetchCache.has(key)) {
		return key;
	}

	for (const url of slots.prefetchCache.keys()) {
		if (hasSameDataTarget(url, key)) {
			return url;
		}
	}

	return undefined;
}

export function deleteNavigationFromSlots(
	slots: NavigationSlots,
	key: string,
	onStatusRelevantChange: () => void,
): boolean {
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === key ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, key))
	) {
		slots.activeNavigation = null;
		onStatusRelevantChange();
		return true;
	}

	const prefetchKey = findMatchingPrefetchKey(slots, key);
	if (prefetchKey) {
		slots.prefetchCache.delete(prefetchKey);
		return true;
	}

	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === key ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, key))
	) {
		slots.pendingRevalidation = null;
		onStatusRelevantChange();
		return true;
	}

	return false;
}

export function transitionNavigationPhaseInSlots(
	slots: NavigationSlots,
	targetUrl: string,
	phase: NavigationPhase,
	onStatusRelevantChange: () => void,
): void {
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, targetUrl))
	) {
		slots.activeNavigation.phase = phase;
		onStatusRelevantChange();
		return;
	}

	const prefetch = slots.prefetchCache.get(targetUrl);
	if (prefetch) {
		prefetch.phase = phase;
		return;
	}
	const prefetchAliasKey = findMatchingPrefetchKey(slots, targetUrl);
	if (prefetchAliasKey) {
		const prefetchAlias = slots.prefetchCache.get(prefetchAliasKey);
		if (prefetchAlias) {
			prefetchAlias.phase = phase;
			return;
		}
	}

	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, targetUrl))
	) {
		slots.pendingRevalidation.phase = phase;
		onStatusRelevantChange();
	}
}

function clearSlotsAndSubmissions(
	slots: NavigationSlots,
	submissions: Map<string | symbol, SubmissionEntry>,
	onStatusRelevantChange: () => void,
): void {
	if (slots.activeNavigation) {
		slots.activeNavigation.control.abortController?.abort();
		slots.activeNavigation = null;
	}

	for (const prefetch of slots.prefetchCache.values()) {
		prefetch.control.abortController?.abort();
	}
	slots.prefetchCache.clear();

	if (slots.pendingRevalidation) {
		slots.pendingRevalidation.control.abortController?.abort();
		slots.pendingRevalidation = null;
	}

	for (const sub of submissions.values()) {
		sub.control.abortController?.abort();
	}
	submissions.clear();

	onStatusRelevantChange();
}

function createNavigationBookkeeping(
	options: CreateNavigationBookkeepingOptions,
): NavigationBookkeeping {
	const { onStatusRelevantChange } = options;

	const slots: NavigationSlots = {
		activeNavigation: null,
		prefetchCache: new Map<string, NavigationEntry>(),
		pendingRevalidation: null,
	};

	function getActiveNavigation(): NavigationEntry | null {
		return slots.activeNavigation;
	}

	function setActiveNavigation(entry: NavigationEntry | null): void {
		slots.activeNavigation = entry;
	}

	function getPendingRevalidation(): NavigationEntry | null {
		return slots.pendingRevalidation;
	}

	function setPendingRevalidation(entry: NavigationEntry | null): void {
		slots.pendingRevalidation = entry;
	}

	function getPrefetchCache(): Map<string, NavigationEntry> {
		return slots.prefetchCache;
	}

	function findNavigationEntry(
		targetUrl: string,
	): NavigationEntry | undefined {
		return findNavigationEntryInSlots(slots, targetUrl);
	}

	function deleteNavigation(key: string): boolean {
		return deleteNavigationFromSlots(slots, key, onStatusRelevantChange);
	}

	function removeNavigation(key: string): void {
		const entry = findNavigationEntry(key);
		if (entry) {
			entry.control.abortController?.abort();
			deleteNavigation(key);
		}
	}

	function getNavigation(key: string): NavigationEntry | undefined {
		return findNavigationEntry(key);
	}

	function hasNavigation(key: string): boolean {
		return findNavigationEntry(key) !== undefined;
	}

	function getNavigationsSize(): number {
		return getNavigationsSizeFromSlots(slots);
	}

	function getNavigations(): Map<string, NavigationEntry> {
		return buildNavigationsMapFromSlots(slots);
	}

	function transitionPhase(targetUrl: string, phase: NavigationPhase): void {
		transitionNavigationPhaseInSlots(
			slots,
			targetUrl,
			phase,
			onStatusRelevantChange,
		);
	}

	function clearNavigationsAndSubmissions(
		submissions: Map<string | symbol, SubmissionEntry>,
	): void {
		clearSlotsAndSubmissions(slots, submissions, onStatusRelevantChange);
	}

	return {
		getActiveNavigation,
		setActiveNavigation,
		getPendingRevalidation,
		setPendingRevalidation,
		getPrefetchCache,
		findNavigationEntry,
		deleteNavigation,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
		transitionPhase,
		clearNavigationsAndSubmissions,
	};
}

function resolveNavigationTargetURL(href: string): string {
	return new URL(href, window.location.href).href;
}

function beginNavigationWithContext(
	beginNavigationContext: BeginNavigationContext,
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl,
	props: NavigateProps,
): NavigationControl {
	const targetUrl = resolveNavigationTargetURL(props.href);

	switch (props.navigationType) {
		case "userNavigation":
			return executeBeginUserNavigation(
				beginNavigationContext,
				props,
				targetUrl,
			);
		case "prefetch":
			return executeBeginPrefetch(beginNavigationContext, props, targetUrl);
		case "revalidation":
			return executeBeginRevalidation(beginNavigationContext, props);
		case "browserHistory":
		case "redirect":
		default:
			return createActiveNavigation(props, "navigate");
	}
}

async function handleNavigationOutcome(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
}): Promise<{ didNavigate: boolean }> {
	const {
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
		navigationProps,
		outcome,
	} = props;
	const targetUrl = resolveNavigationTargetURL(navigationProps.href);

	switch (outcome.type) {
		case "aborted": {
			deleteNavigation(targetUrl);
			return { didNavigate: false };
		}

		case "redirect": {
			const entry = findNavigationEntry(targetUrl);
			if (!entry) {
				return { didNavigate: false };
			}
			if (isStaleRevalidationEntry(entry)) {
				deleteNavigation(targetUrl);
				return { didNavigate: false };
			}

			if (entry.type === "prefetch" && entry.intent === "none") {
				deleteNavigation(targetUrl);
				return { didNavigate: false };
			}

			syncBuildIDFromRedirectData(outcome.redirectData);
			deleteNavigation(targetUrl);
			await effectuateRedirectDataResult(
				outcome.redirectData,
				navigationProps.redirectCount || 0,
				navigationProps,
			);
			return { didNavigate: false };
		}

		case "success": {
			const entry = findNavigationEntry(targetUrl);
			if (!entry) {
				return { didNavigate: false };
			}

			if (entry.intent === "navigate" || entry.intent === "revalidate") {
				onNavigationIntentResolved?.();
			}

			await processSuccessfulNavigation(outcome, entry);

			if (entry.intent === "none" && entry.type === "prefetch") {
				return { didNavigate: false };
			}

			return { didNavigate: true };
		}

		default: {
			const exhaustive: never = outcome;
			throw new Error(
				`Unexpected navigation outcome type: ${(exhaustive as any).type}`,
			);
		}
	}
}

type ProcessSuccessfulNavigationContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
};

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

function syncBuildIDFromResponse(response: Response): void {
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
	context.transitionPhase(entry.targetUrl, "rendering");

	try {
		await __reRenderApp({
			json: outcome.json,
			navigationType: entry.type,
			runHistoryOptions: buildRunHistoryOptions(entry, outcome.props),
			onFinish: () => {
				context.transitionPhase(entry.targetUrl, "complete");
			},
		});
	} catch (error) {
		context.transitionPhase(entry.targetUrl, "complete");
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}

async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	try {
		const { response, json } = outcome;

		if (isStaleRevalidationEntry(entry)) {
			context.deleteNavigation(entry.targetUrl);
			return;
		}

		applyResponseArtifactsWhenBuildMatches(response, json);

		context.transitionPhase(entry.targetUrl, "waiting");

		if (!context.findNavigationEntry(entry.targetUrl)) {
			return;
		}

		syncBuildIDFromResponse(response);
		await waitForSuccessfulNavigationAssets(outcome);

		if (entry.intent === "none") {
			context.transitionPhase(entry.targetUrl, "complete");
			return;
		}

		if (isStaleRevalidationEntry(entry)) {
			return;
		}

		await renderSuccessfulNavigation(context, outcome, entry);
	} finally {
		if (!(entry.type === "prefetch" && entry.intent === "none")) {
			context.deleteNavigation(entry.targetUrl);
		}
	}
}

type ActiveSubmission = {
	abortController: AbortController;
	submissionKey: string | symbol;
	entry: SubmissionEntry;
};

function createActiveSubmission(options?: SubmitOptions): ActiveSubmission {
	const abortController = new AbortController();
	const submissionKey = options?.dedupeKey
		? `submission:${options.dedupeKey}`
		: Symbol("submission");

	const entry: SubmissionEntry = {
		control: {
			abortController,
			promise: Promise.resolve() as any,
		},
		startTime: Date.now(),
		skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
	};

	return { abortController, submissionKey, entry };
}

type SubmitExecutionContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	navigate: (props: NavigateProps) => Promise<{
		didNavigate: boolean;
	}>;
};

function beginSubmissionLifecycle(
	context: SubmitExecutionContext,
	activeSubmission: ActiveSubmission,
): void {
	if (typeof activeSubmission.submissionKey === "string") {
		const existing = context.submissions.get(
			activeSubmission.submissionKey,
		);
		if (existing) {
			existing.control.abortController?.abort("deduped");
		}
	}

	context.submissions.set(
		activeSubmission.submissionKey,
		activeSubmission.entry,
	);
	context.scheduleStatusUpdate();
}

function finishSubmissionLifecycle(
	context: SubmitExecutionContext,
	activeSubmission: ActiveSubmission,
): void {
	if (
		context.submissions.get(activeSubmission.submissionKey) ===
		activeSubmission.entry
	) {
		context.submissions.delete(activeSubmission.submissionKey);
	}

	context.scheduleStatusUpdate();
}

function buildSubmitRequestInit(props: {
	requestInit?: RequestInit;
	signal: AbortSignal;
}): RequestInit {
	const { requestInit, signal } = props;
	const headers = new Headers(requestInit?.headers);
	const deploymentID = __vormaClientGlobal.get("deploymentID");
	if (deploymentID) {
		headers.set("x-deployment-id", deploymentID);
	}

	return {
		...requestInit,
		headers,
		signal,
	};
}

async function executeSubmitRequest(props: {
	abortController: AbortController;
	url: URL;
	requestInit: RequestInit;
}) {
	return handleRedirects({
		abortController: props.abortController,
		url: props.url,
		isPrefetch: false,
		redirectCount: 0,
		requestInit: props.requestInit,
	});
}

type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

async function finalizeSubmitResponse<T>(props: {
	response?: Response;
	redirectData: RedirectData | null;
	requestInit?: RequestInit;
	options?: SubmitOptions;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	isSubmissionCurrent: () => boolean;
}): Promise<SubmitResult<T>> {
	const {
		response,
		redirectData,
		requestInit,
		options,
		navigate,
		isSubmissionCurrent,
	} = props;

	if (!isSubmissionCurrent()) {
		return { success: false, error: "Aborted" };
	}

	if (!response || !response.ok) {
		return {
			success: false,
			error: String(response?.status || "unknown"),
		};
	}

	if (redirectData?.status === "should") {
		if (!isSubmissionCurrent()) {
			return { success: false, error: "Aborted" };
		}
		await effectuateRedirectDataResult(redirectData, 0);
		return { success: true, data: undefined as T };
	}

	const data = await response.json();
	if (!isSubmissionCurrent()) {
		return { success: false, error: "Aborted" };
	}

	const isGET = getIsGETRequest(requestInit);
	const redirected = redirectData?.status === "did";
	if (!isGET && !redirected && options?.revalidate !== false) {
		if (!isSubmissionCurrent()) {
			return { success: false, error: "Aborted" };
		}
		await navigate({
			href: window.location.href,
			navigationType: "revalidation",
		});
	}

	return { success: true, data: data as T };
}

async function executeSubmitRuntime<T = any>(
	context: SubmitExecutionContext,
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<{ success: true; data: T } | { success: false; error: string }> {
	const activeSubmission = createActiveSubmission(options);
	const isSubmissionCurrent = (): boolean =>
		context.submissions.get(activeSubmission.submissionKey) ===
		activeSubmission.entry;
	beginSubmissionLifecycle(context, activeSubmission);

	try {
		const urlToUse = new URL(url, window.location.href);
		const finalRequestInit = buildSubmitRequestInit({
			requestInit,
			signal: activeSubmission.abortController.signal,
		});

		const { redirectData, response } = await executeSubmitRequest({
			abortController: activeSubmission.abortController,
			url: urlToUse,
			requestInit: finalRequestInit,
		});

		if (!isSubmissionCurrent()) {
			return { success: false, error: "Aborted" };
		}

		if (response && isSubmissionCurrent()) {
			syncBuildIDFromResponse(response);
		}

		return await finalizeSubmitResponse<T>({
			response,
			redirectData,
			requestInit,
			options,
			navigate: context.navigate,
			isSubmissionCurrent,
		});
	} catch (error) {
		if (
			isAbortError(error) ||
			activeSubmission.abortController.signal.aborted
		) {
			return { success: false, error: "Aborted" };
		}
		logError(error);
		return {
			success: false,
			error: error instanceof Error ? error.message : "Unknown error",
		};
	} finally {
		finishSubmissionLifecycle(context, activeSubmission);
	}
}

export type CreateNavigationRuntimeOptions = {
	onNavigationIntentResolved?: () => void;
};

export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	const submissions = new Map<string | symbol, SubmissionEntry>();
	let scheduleStatusUpdate: () => void = () => {};
	const navigationBookkeeping = createNavigationBookkeeping({
		onStatusRelevantChange: () => {
			scheduleStatusUpdate();
		},
	});

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			activeNavigation: navigationBookkeeping.getActiveNavigation(),
			pendingRevalidation: navigationBookkeeping.getPendingRevalidation(),
			submissions,
		});
	}

	const statusSignaler = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});
	scheduleStatusUpdate = statusSignaler.scheduleStatusUpdate;

	const prefetchCache = navigationBookkeeping.getPrefetchCache();
	const getActiveNavigation = navigationBookkeeping.getActiveNavigation;
	const setActiveNavigation = navigationBookkeeping.setActiveNavigation;
	const getPendingRevalidation = navigationBookkeeping.getPendingRevalidation;
	const setPendingRevalidation = navigationBookkeeping.setPendingRevalidation;

	const { createActiveNavigation, createPrefetch, createRevalidation } =
		createNavigationControls({
			fetchRouteData,
			setActiveNavigation,
			prefetchCache,
			getPendingRevalidation,
			setPendingRevalidation,
			scheduleStatusUpdate,
			deleteNavigation: (key: string) =>
				navigationBookkeeping.deleteNavigation(key),
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
				): void => navigationBookkeeping.transitionPhase(targetUrl, phase),
				findNavigationEntry: navigationBookkeeping.findNavigationEntry,
				deleteNavigation: navigationBookkeeping.deleteNavigation,
			},
			outcome,
			entry,
		);

	const beginNavigation = (props: NavigateProps): NavigationControl =>
		beginNavigationWithContext(
			beginNavigationContext,
			createActiveNavigation,
			props,
		);

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		const control = beginNavigation(props);

		try {
			const outcome = await control.promise;
			return await handleNavigationOutcome({
				findNavigationEntry: navigationBookkeeping.findNavigationEntry,
				deleteNavigation: navigationBookkeeping.deleteNavigation,
				processSuccessfulNavigation,
				onNavigationIntentResolved,
				navigationProps: props,
				outcome,
			});
		} catch {
			const targetUrl = resolveNavigationTargetURL(props.href);
			navigationBookkeeping.deleteNavigation(targetUrl);
			return { didNavigate: false };
		}
	};

	const submit = <T = any>(
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
		navigationBookkeeping.clearNavigationsAndSubmissions(submissions);
	}

	return {
		_submissions: submissions,
		navigate,
		beginNavigation,
		processSuccessfulNavigation,
		submit,
		removeNavigation: navigationBookkeeping.removeNavigation,
		getNavigation: navigationBookkeeping.getNavigation,
		hasNavigation: navigationBookkeeping.hasNavigation,
		getNavigationsSize: navigationBookkeeping.getNavigationsSize,
		getNavigations: navigationBookkeeping.getNavigations,
		getStatus,
		clearAll,
	};
}

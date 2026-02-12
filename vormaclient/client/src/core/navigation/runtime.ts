import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import { getIsGETRequest } from "vorma/kit/url";
import {
	dispatchBuildIDEvent,
	dispatchStatusEvent,
	type StatusEventDetail,
} from "../../platform/events.ts";
import { hasSameDataTarget, resolveAbsoluteHref } from "../../platform/url.ts";
import {
	__vormaClientGlobal,
	type GetRouteDataOutput,
} from "../../app/context.ts";
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

type NavigationSlotMatch =
	| {
			slot: "active";
			entry: NavigationEntry;
	  }
	| {
			slot: "prefetch";
			key: string;
			entry: NavigationEntry;
	  }
	| {
			slot: "pendingRevalidation";
			entry: NavigationEntry;
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
	return matchSlotByTargetURL(slots, targetUrl)?.entry;
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

function matchSlotByTargetURL(
	slots: NavigationSlots,
	targetUrl: string,
): NavigationSlotMatch | undefined {
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, targetUrl))
	) {
		return {
			slot: "active",
			entry: slots.activeNavigation,
		};
	}

	const prefetchKey = findMatchingPrefetchKey(slots, targetUrl);
	if (prefetchKey) {
		return {
			slot: "prefetch",
			key: prefetchKey,
			entry: slots.prefetchCache.get(prefetchKey)!,
		};
	}

	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, targetUrl))
	) {
		return {
			slot: "pendingRevalidation",
			entry: slots.pendingRevalidation,
		};
	}

	return undefined;
}

export function deleteNavigationFromSlots(
	slots: NavigationSlots,
	key: string,
	onStatusRelevantChange: () => void,
): boolean {
	const matchedSlot = matchSlotByTargetURL(slots, key);
	if (!matchedSlot) {
		return false;
	}

	switch (matchedSlot.slot) {
		case "active":
			slots.activeNavigation = null;
			onStatusRelevantChange();
			return true;

		case "prefetch":
			slots.prefetchCache.delete(matchedSlot.key);
			return true;

		case "pendingRevalidation":
			slots.pendingRevalidation = null;
			onStatusRelevantChange();
			return true;
	}
}

export function transitionNavigationPhaseInSlots(
	slots: NavigationSlots,
	targetUrl: string,
	phase: NavigationPhase,
	onStatusRelevantChange: () => void,
): void {
	const matchedSlot = matchSlotByTargetURL(slots, targetUrl);
	if (!matchedSlot) {
		return;
	}

	matchedSlot.entry.phase = phase;
	if (matchedSlot.slot !== "prefetch") {
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

function beginNavigationWithContext(
	beginNavigationContext: BeginNavigationContext,
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl,
	props: NavigateProps,
): NavigationControl {
	const targetUrl = resolveAbsoluteHref(props.href);

	switch (props.navigationType) {
		case "userNavigation":
			return executeBeginUserNavigation(
				beginNavigationContext,
				props,
				targetUrl,
			);
		case "prefetch":
			return executeBeginPrefetch(
				beginNavigationContext,
				props,
				targetUrl,
			);
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
	const targetUrl = resolveAbsoluteHref(navigationProps.href);

	if (outcome.type === "aborted") {
		deleteNavigation(targetUrl);
		return { didNavigate: false };
	}

	if (outcome.type === "redirect") {
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
			promise: Promise.resolve() as Promise<unknown>,
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
}): Promise<{ redirectData: RedirectData | null; response: Response }> {
	const result = await handleRedirects({
		abortController: props.abortController,
		url: props.url,
		isPrefetch: false,
		redirectCount: 0,
		requestInit: props.requestInit,
	});

	if (!result.response) {
		throw new Error("Submit request completed without a response.");
	}

	return {
		redirectData: result.redirectData,
		response: result.response,
	};
}

type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

function getAbortedSubmitResult<T>(): SubmitResult<T> {
	return { success: false, error: "Aborted" };
}

function getUnknownSubmitErrorResult<T>(): SubmitResult<T> {
	return { success: false, error: "Unknown error" };
}

function getSubmitErrorResult<T>(error: string): SubmitResult<T> {
	return { success: false, error };
}

function getSubmitStaleResultIfAny<T>(
	isSubmissionCurrent: () => boolean,
): SubmitResult<T> | null {
	if (isSubmissionCurrent()) {
		return null;
	}
	return getAbortedSubmitResult<T>();
}

async function finalizeSubmitResponse<T>(props: {
	response: Response;
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

	const staleBeforeResponse =
		getSubmitStaleResultIfAny<T>(isSubmissionCurrent);
	if (staleBeforeResponse) return staleBeforeResponse;

	if (!response.ok) {
		return getSubmitErrorResult<T>(String(response.status));
	}

	if (redirectData?.status === "should") {
		await effectuateRedirectDataResult(redirectData, 0);
		return { success: true, data: undefined as T };
	}

	const data = await response.json();
	const staleBeforeReturn = getSubmitStaleResultIfAny<T>(isSubmissionCurrent);
	if (staleBeforeReturn) return staleBeforeReturn;

	const isGET = getIsGETRequest(requestInit);
	const redirected = redirectData?.status === "did";
	if (!isGET && !redirected && options?.revalidate !== false) {
		const staleBeforeRevalidate =
			getSubmitStaleResultIfAny<T>(isSubmissionCurrent);
		if (staleBeforeRevalidate) return staleBeforeRevalidate;
		await navigate({
			href: window.location.href,
			navigationType: "revalidation",
		});
	}

	return { success: true, data: data as T };
}

async function executeSubmitRuntime<T = unknown>(
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
		const urlToUse = new URL(resolveAbsoluteHref(url));
		const finalRequestInit = buildSubmitRequestInit({
			requestInit,
			signal: activeSubmission.abortController.signal,
		});

		const { redirectData, response } = await executeSubmitRequest({
			abortController: activeSubmission.abortController,
			url: urlToUse,
			requestInit: finalRequestInit,
		});

		const staleAfterRequest =
			getSubmitStaleResultIfAny<T>(isSubmissionCurrent);
		if (staleAfterRequest) return staleAfterRequest;

		syncBuildIDFromResponse(response);

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
			return getAbortedSubmitResult<T>();
		}
		logError(error);
		if (error instanceof Error) {
			return getSubmitErrorResult<T>(error.message);
		}
		return getUnknownSubmitErrorResult<T>();
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
				): void =>
					navigationBookkeeping.transitionPhase(targetUrl, phase),
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
			const targetUrl = resolveAbsoluteHref(props.href);
			navigationBookkeeping.deleteNavigation(targetUrl);
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

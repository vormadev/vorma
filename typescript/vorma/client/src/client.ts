import type { BrowserHistory } from "history";
import { resolveAbsoluteHrefWithOptionalSearchAndHash } from "vorma/kit/url";
import {
	__vormaClientGlobal,
	setNavigationStateAccess,
} from "./app/context.ts";
import { createNavigationRuntime } from "./core/navigation/runtime.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationOutcome,
	NavigationStateManager,
	SubmitOptions,
	SubmitResult,
} from "./core/navigation/types.ts";
import type { StatusEventDetail } from "./platform/events.ts";
import { HistoryManager } from "./platform/history.ts";
import { getRuntimeLocationState } from "./platform/location.ts";

export type {
	NavigateProps,
	NavigationControl,
	NavigationOutcome,
	SubmitOptions,
	SubmitResult,
	VormaNavigationType,
} from "./core/navigation/types.ts";

export type RevalidationTriggerTimestampState = {
	lastTriggeredNavOrRevalidateTimestampMS: number;
};

export type RevalidationTriggerTimestampEvent = {
	type: "navigation_or_revalidation_intent_committed";
	committedTimestampMS: number;
};

export function reduceRevalidationTriggerTimestampState(props: {
	state: RevalidationTriggerTimestampState;
	event: RevalidationTriggerTimestampEvent;
}): RevalidationTriggerTimestampState {
	switch (props.event.type) {
		case "navigation_or_revalidation_intent_committed":
			return {
				...props.state,
				lastTriggeredNavOrRevalidateTimestampMS:
					props.event.committedTimestampMS,
			};
	}
}

export type RevalidationTriggerTimestampRuntime = {
	recordNavigationOrRevalidationIntentCommitted: () => void;
	getLastTriggeredNavOrRevalidateTimestampMS: () => number;
};

export function createRevalidationTriggerTimestampRuntime(props?: {
	getNowTimestampMS?: () => number;
	initialState?: RevalidationTriggerTimestampState;
}): RevalidationTriggerTimestampRuntime {
	const getNowTimestampMS = props?.getNowTimestampMS ?? (() => Date.now());
	let state: RevalidationTriggerTimestampState = props?.initialState ?? {
		lastTriggeredNavOrRevalidateTimestampMS: getNowTimestampMS(),
	};

	return {
		recordNavigationOrRevalidationIntentCommitted: () => {
			state = reduceRevalidationTriggerTimestampState({
				state,
				event: {
					type: "navigation_or_revalidation_intent_committed",
					committedTimestampMS: getNowTimestampMS(),
				},
			});
		},
		getLastTriggeredNavOrRevalidateTimestampMS: () =>
			state.lastTriggeredNavOrRevalidateTimestampMS,
	};
}

const revalidationTriggerTimestampRuntime =
	createRevalidationTriggerTimestampRuntime();

let navigationStateManagerSingleton: NavigationStateManager | null = null;

function getNavigationStateManager(): NavigationStateManager {
	if (navigationStateManagerSingleton) {
		return navigationStateManagerSingleton;
	}

	navigationStateManagerSingleton = createNavigationRuntime({
		onNavigationIntentResolved: () => {
			revalidationTriggerTimestampRuntime.recordNavigationOrRevalidationIntentCommitted();
		},
	});
	setNavigationStateAccess(navigationStateManagerSingleton);
	return navigationStateManagerSingleton;
}

/**
 * Ensures the singleton navigation runtime is initialized.
 */
export function ensureNavigationRuntimeInitialized(): void {
	getNavigationStateManager();
}

// Global singleton runtime proxy that defers concrete runtime construction
// until the first method/property access.
export const navigationStateManager: NavigationStateManager = {
	get _submissions() {
		return getNavigationStateManager()._submissions;
	},
	navigate(props: NavigateProps): Promise<{ didNavigate: boolean }> {
		return getNavigationStateManager().navigate(props);
	},
	beginNavigation(props: NavigateProps): NavigationControl {
		return getNavigationStateManager().beginNavigation(props);
	},
	processSuccessfulNavigation(
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> {
		return getNavigationStateManager().processSuccessfulNavigation(
			outcome,
			entry,
		);
	},
	submit<T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	): Promise<SubmitResult<T>> {
		return getNavigationStateManager().submit<T>(url, requestInit, options);
	},
	removeNavigation(targetUrl: string): void {
		getNavigationStateManager().removeNavigation(targetUrl);
	},
	getNavigation(targetUrl: string): NavigationEntry | undefined {
		return getNavigationStateManager().getNavigation(targetUrl);
	},
	hasNavigation(targetUrl: string): boolean {
		return getNavigationStateManager().hasNavigation(targetUrl);
	},
	getNavigationsSize(): number {
		return getNavigationStateManager().getNavigationsSize();
	},
	getNavigations(): Map<string, NavigationEntry> {
		return getNavigationStateManager().getNavigations();
	},
	getStatus(): StatusEventDetail {
		return getNavigationStateManager().getStatus();
	},
	getDebugJournal() {
		return getNavigationStateManager().getDebugJournal();
	},
	clearDebugJournal(): void {
		getNavigationStateManager().clearDebugJournal();
	},
	clearAll(): void {
		getNavigationStateManager().clearAll();
	},
};

/////////////////////////////////////////////////////////////////////
// PUBLIC API
/////////////////////////////////////////////////////////////////////

/**
 * Navigates to a route and drives the full client navigation lifecycle.
 */
export async function vormaNavigate(
	href: string,
	options?: {
		replace?: boolean;
		scrollToTop?: boolean;
		search?: string;
		hash?: string;
		state?: unknown;
	},
): Promise<void> {
	const targetHref = resolveAbsoluteHrefWithOptionalSearchAndHash({
		href,
		search: options?.search,
		hash: options?.hash,
	});

	await navigationStateManager.navigate({
		href: targetHref,
		navigationType: "userNavigation",
		replace: options?.replace,
		scrollToTop: options?.scrollToTop,
		state: options?.state,
	});
}

/**
 * Returns the last navigation/revalidation trigger timestamp used by client
 * state machines to reason about staleness.
 */
export function getLastTriggeredNavOrRevalidateTimestampMS(): number {
	return revalidationTriggerTimestampRuntime.getLastTriggeredNavOrRevalidateTimestampMS();
}

/**
 * Revalidates the current location without changing history.
 */
export async function revalidate() {
	await navigationStateManager.navigate({
		href: window.location.href,
		navigationType: "revalidation",
	});
}

/**
 * Submits an action request through the shared navigation runtime.
 */
export async function submit<T = unknown>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	return navigationStateManager.submit(url, requestInit, options);
}

/**
 * Starts a controllable navigation lifecycle operation.
 */
export function beginNavigation(props: NavigateProps): NavigationControl {
	return navigationStateManager.beginNavigation(props);
}

/**
 * Returns current navigation status event detail.
 */
export function getStatus(): StatusEventDetail {
	return navigationStateManager.getStatus();
}

/**
 * Returns a normalized browser location snapshot used by adapters.
 */
export function getLocation() {
	return getRuntimeLocationState();
}

/**
 * Returns the current runtime build id.
 */
export function getBuildID(): string {
	return __vormaClientGlobal.get("buildID");
}

function getClientRootElementID(): string {
	const rootElementID = __vormaClientGlobal.get("rootElementID");
	if (typeof rootElementID === "string" && rootElementID.trim().length > 0) {
		return rootElementID;
	}
	return "vorma-root";
}

/**
 * Returns the client root element and validates both existence and element
 * type.
 */
export function getRootEl(): HTMLDivElement {
	const rootElementID = getClientRootElementID();
	const rootEl = document.getElementById(rootElementID);
	if (rootEl === null) {
		throw new Error(`Expected element with id "${rootElementID}" to exist`);
	}
	if (!(rootEl instanceof HTMLDivElement)) {
		throw new Error(
			`Expected element with id "${rootElementID}" to be an HTMLDivElement`,
		);
	}
	return rootEl;
}

/**
 * Returns the singleton history integration instance.
 */
export function getHistoryInstance(): BrowserHistory {
	ensureNavigationRuntimeInitialized();
	return HistoryManager.getInstance();
}

/**
 * Returns the in-memory navigation debug journal.
 */
export function getNavigationDebugJournal() {
	return navigationStateManager.getDebugJournal();
}

/**
 * Clears the in-memory navigation debug journal.
 */
export function clearNavigationDebugJournal(): void {
	navigationStateManager.clearDebugJournal();
}

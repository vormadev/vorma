import { resolveAbsoluteHrefWithOptionalSearchAndHash } from "vorma/kit/url";
import type { StatusEventDetail } from "./platform/events.ts";
import { HistoryManager } from "./platform/history.ts";
import type { historyInstance } from "./platform/history.ts";
import { createNavigationRuntime } from "./core/navigation/runtime.ts";
import type {
	NavigateProps,
	NavigationControl,
	SubmitOptions,
} from "./core/navigation/types.ts";
import { createRevalidationTriggerTimestampRuntime } from "./core/revalidation_trigger_timestamp_state_machine.ts";
import { setNavigationStateAccess } from "./app/context.ts";
import { __vormaClientGlobal } from "./app/context.ts";

export type {
	NavigateProps,
	NavigationControl,
	NavigationOutcome,
	SubmitOptions,
	VormaNavigationType,
} from "./core/navigation/types.ts";

const revalidationTriggerTimestampRuntime =
	createRevalidationTriggerTimestampRuntime();

// Global singleton instance
export const navigationStateManager = createNavigationRuntime({
	onNavigationIntentResolved: () => {
		revalidationTriggerTimestampRuntime.recordNavigationOrRevalidationIntentCommitted();
	},
});
setNavigationStateAccess(navigationStateManager);

/////////////////////////////////////////////////////////////////////
// PUBLIC API
/////////////////////////////////////////////////////////////////////

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

export function getLastTriggeredNavOrRevalidateTimestampMS(): number {
	return revalidationTriggerTimestampRuntime.getLastTriggeredNavOrRevalidateTimestampMS();
}

export async function revalidate() {
	await navigationStateManager.navigate({
		href: window.location.href,
		navigationType: "revalidation",
	});
}

export async function submit<T = unknown>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<{ success: true; data: T } | { success: false; error: string }> {
	return navigationStateManager.submit(url, requestInit, options);
}

export function beginNavigation(props: NavigateProps): NavigationControl {
	return navigationStateManager.beginNavigation(props);
}

export function getStatus(): StatusEventDetail {
	return navigationStateManager.getStatus();
}

export function getLocation() {
	return {
		pathname: window.location.pathname,
		search: window.location.search,
		hash: window.location.hash,
		state: HistoryManager.getInstance().location.state,
	};
}

export function getBuildID(): string {
	return __vormaClientGlobal.get("buildID");
}

export function getRootEl(): HTMLDivElement {
	const rootEl = document.getElementById("vorma-root");
	if (rootEl === null) {
		throw new Error('Expected element with id "vorma-root" to exist');
	}
	if (!(rootEl instanceof HTMLDivElement)) {
		throw new Error(
			'Expected element with id "vorma-root" to be an HTMLDivElement',
		);
	}
	return rootEl;
}

export function getHistoryInstance(): historyInstance {
	return HistoryManager.getInstance();
}

export function getNavigationDebugJournal() {
	return navigationStateManager.getDebugJournal();
}

export function clearNavigationDebugJournal(): void {
	navigationStateManager.clearDebugJournal();
}

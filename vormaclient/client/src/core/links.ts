import { getAnchorDetailsFromEvent, getHrefDetails } from "vorma/kit/url";
import { navigationStateManager, vormaNavigate } from "../client.ts";
import {
	hasSameNavigationTarget,
	isSameDocumentHashChange,
	isSameDocumentLocation,
	resolveAbsoluteHref,
} from "../platform/url.ts";
import type { NavigationOutcome } from "./navigation/types.ts";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "./redirects.ts";
import { saveScrollState } from "../platform/scroll.ts";
import { logError } from "../platform/safety.ts";

type EligibleInternalAnchorDetails = Exclude<
	ReturnType<typeof getAnchorDetailsFromEvent>,
	null
>;

function isJustAHashChange(
	anchorDetails: EligibleInternalAnchorDetails,
): boolean {
	return isSameDocumentHashChange(
		anchorDetails.anchor.href,
		window.location.href,
	);
}

function isSameDocumentNoopNavigationTarget(
	anchorDetails: EligibleInternalAnchorDetails,
): boolean {
	return isSameDocumentLocation(
		anchorDetails.anchor.href,
		window.location.href,
	);
}

function getEligibleInternalAnchorDetails(
	event: Event,
): EligibleInternalAnchorDetails | null {
	if (event.defaultPrevented) return null;

	const anchorDetails = getAnchorDetailsFromEvent(
		event as unknown as MouseEvent,
	);
	if (!anchorDetails) return null;

	if (
		!anchorDetails.isEligibleForDefaultPrevention ||
		!anchorDetails.isInternal
	) {
		return null;
	}

	return anchorDetails;
}

type LinkOnClickCallback<E extends Event> = (event: E) => void | Promise<void>;

export type LinkOnClickCallbacks<E extends Event> = {
	beforeBegin?: LinkOnClickCallback<E>;
	beforeRender?: LinkOnClickCallback<E>;
	afterRender?: LinkOnClickCallback<E>;
};

type LinkLifecycleCallbacks<E extends Event> = {
	beforeRender?: (event: E) => void | Promise<void>;
	afterRender?: (event: E) => void | Promise<void>;
};

function getCurrentNavigationEntryForControl(props: {
	targetUrl: string;
	controlPromise: Promise<NavigationOutcome>;
}): ReturnType<typeof navigationStateManager.getNavigation> {
	const { targetUrl, controlPromise } = props;
	const currentEntry = navigationStateManager.getNavigation(targetUrl);
	if (!currentEntry || currentEntry.control.promise !== controlPromise) {
		return undefined;
	}

	return currentEntry;
}

async function handleLinkNavigationOutcome<E extends Event>(props: {
	event: E;
	outcome: NavigationOutcome;
	targetUrl: string;
	controlPromise: Promise<NavigationOutcome>;
	callbacks: LinkLifecycleCallbacks<E>;
}): Promise<void> {
	const { event, outcome, targetUrl, controlPromise, callbacks } = props;
	const currentEntry = getCurrentNavigationEntryForControl({
		targetUrl,
		controlPromise,
	});
	if (!currentEntry) {
		return;
	}

	if (outcome.type === "aborted") {
		navigationStateManager.removeNavigation(targetUrl);
		return;
	}

	if (outcome.type === "redirect") {
		await callbacks.beforeRender?.(event);
		if (
			!getCurrentNavigationEntryForControl({
				targetUrl,
				controlPromise,
			})
		) {
			return;
		}

		syncBuildIDFromRedirectData(outcome.redirectData);
		navigationStateManager.removeNavigation(targetUrl);
		await effectuateRedirectDataResult(
			outcome.redirectData,
			outcome.props.redirectCount || 0,
			outcome.props,
		);
		await callbacks.afterRender?.(event);
		return;
	}

	await callbacks.beforeRender?.(event);
	const currentEntryAfterBeforeRender = getCurrentNavigationEntryForControl({
		targetUrl,
		controlPromise,
	});
	if (!currentEntryAfterBeforeRender) {
		return;
	}

	await navigationStateManager.processSuccessfulNavigation(
		outcome,
		currentEntryAfterBeforeRender,
	);
	await callbacks.afterRender?.(event);
}

export function createLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return async (event: E) => {
		const anchorDetails = getEligibleInternalAnchorDetails(event);
		if (!anchorDetails) return;

		const { anchor } = anchorDetails;

		if (isSameDocumentNoopNavigationTarget(anchorDetails)) {
			return;
		}

		if (isJustAHashChange(anchorDetails)) {
			saveScrollState();
			return;
		}

		event.preventDefault();
		await callbacks.beforeBegin?.(event);

		const control = navigationStateManager.beginNavigation({
			href: anchor.href,
			navigationType: "userNavigation",
			scrollToTop: callbacks.scrollToTop,
			replace: callbacks.replace,
			state: callbacks.state,
		});
		const controlPromise = control.promise;
		let outcome: NavigationOutcome;
		try {
			outcome = await controlPromise;
		} catch (error) {
			const targetUrl = resolveAbsoluteHref(anchor.href);
			const currentEntry =
				navigationStateManager.getNavigation(targetUrl);
			if (currentEntry?.control.promise === controlPromise) {
				navigationStateManager.removeNavigation(targetUrl);
			}
			logError("Link navigation failed", error);
			return;
		}

		const targetUrl = resolveAbsoluteHref(anchor.href);
		await handleLinkNavigationOutcome({
			event,
			outcome,
			targetUrl,
			controlPromise,
			callbacks: {
				beforeRender: callbacks.beforeRender,
				afterRender: callbacks.afterRender,
			},
		});
	};
}

function findIdlePrefetchNavigationByDataTarget(
	targetHref: string,
): ReturnType<typeof navigationStateManager.getNavigation> {
	const exact = navigationStateManager.getNavigation(targetHref);
	if (exact && exact.type === "prefetch" && exact.intent === "none") {
		return exact;
	}

	for (const nav of navigationStateManager.getNavigations().values()) {
		if (
			nav.type === "prefetch" &&
			nav.intent === "none" &&
			hasSameNavigationTarget(nav.targetUrl, targetHref)
		) {
			return nav;
		}
	}

	return undefined;
}

function hasIdlePrefetchNavigation(targetHref: string): boolean {
	return !!findIdlePrefetchNavigationByDataTarget(targetHref);
}

async function startPrefetchNavigation(props: {
	targetHref: string;
	state?: unknown;
}): Promise<void> {
	await navigationStateManager.navigate({
		href: props.targetHref,
		navigationType: "prefetch",
		state: props.state,
	});
}

function abortIdlePrefetchNavigation(targetHref: string): void {
	const nav = findIdlePrefetchNavigationByDataTarget(targetHref);
	if (!nav) return;

	nav.control.abortController?.abort();
	navigationStateManager.removeNavigation(nav.targetUrl);
}

function buildPrefetchTargetHref(props: {
	relativeURL: string;
	search?: string;
	hash?: string;
}): string {
	const fullUrl = new URL(resolveAbsoluteHref(props.relativeURL));
	if (props.search !== undefined) fullUrl.search = props.search;
	if (props.hash !== undefined) fullUrl.hash = props.hash;
	return fullUrl.href;
}

type ClickNavigationOptions = {
	scrollToTop?: boolean;
	replace?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

async function handlePrefetchClick<E extends Event>(props: {
	event: E;
	relativeURL: string;
	prefetchStarted: boolean;
	clearPendingTimer: () => void;
	callbacks: LinkOnClickCallbacks<E>;
	navigationOptions: ClickNavigationOptions;
}): Promise<void> {
	const {
		event,
		relativeURL,
		prefetchStarted,
		clearPendingTimer,
		callbacks,
		navigationOptions,
	} = props;
	const anchorDetails = getEligibleInternalAnchorDetails(event);
	if (!anchorDetails) return;

	if (isSameDocumentNoopNavigationTarget(anchorDetails)) {
		clearPendingTimer();
		return;
	}

	if (isJustAHashChange(anchorDetails)) {
		clearPendingTimer();
		saveScrollState();
		return;
	}

	event.preventDefault();
	clearPendingTimer();

	if (callbacks.beforeBegin && !prefetchStarted) {
		await callbacks.beforeBegin(event);
	}

	if (callbacks.beforeRender) {
		await callbacks.beforeRender(event);
	}
	await vormaNavigate(relativeURL, navigationOptions);
	if (callbacks.afterRender) {
		await callbacks.afterRender(event);
	}
}

export type CreatePrefetchHandlersInput<E extends Event> =
	LinkOnClickCallbacks<E> & {
		href: string;
		delayMs?: number;
		scrollToTop?: boolean;
		replace?: boolean;
		search?: string;
		hash?: string;
		state?: unknown;
	};

export function createPrefetchHandlers<E extends Event>(
	input: CreatePrefetchHandlersInput<E>,
) {
	const hrefDetails = getHrefDetails(input.href);
	if (!hrefDetails.isHTTP) {
		return;
	}

	const { relativeURL } = hrefDetails;
	if (!relativeURL || hrefDetails.isExternal) {
		return;
	}

	let timer: number | undefined;
	let prefetchStarted = false;
	const delayMs = input.delayMs ?? 100;
	const targetHref = buildPrefetchTargetHref({
		relativeURL,
		search: input.search,
		hash: input.hash,
	});

	function clearPendingTimer(): void {
		if (timer === undefined) {
			return;
		}

		clearTimeout(timer);
		timer = undefined;
	}

	function hasActiveIdlePrefetch(): boolean {
		return hasIdlePrefetchNavigation(targetHref);
	}

	async function prefetch(event: E): Promise<void> {
		prefetchStarted = true;

		try {
			if (input.beforeBegin) {
				await input.beforeBegin(event);
			}
			await startPrefetchNavigation({ targetHref, state: input.state });
			prefetchStarted = hasActiveIdlePrefetch();
		} catch (error) {
			prefetchStarted = false;
			logError(
				"Prefetch start failed; allowing subsequent retries.",
				error,
			);
		}
	}

	function start(event: E): void {
		if (timer !== undefined) return;
		if (prefetchStarted && hasActiveIdlePrefetch()) return;
		prefetchStarted = false;
		timer = window.setTimeout(() => {
			timer = undefined;
			void prefetch(event);
		}, delayMs);
	}

	function stop(): void {
		clearPendingTimer();
		abortIdlePrefetchNavigation(targetHref);
		prefetchStarted = false;
	}

	async function onClick(event: E): Promise<void> {
		await handlePrefetchClick({
			event,
			relativeURL,
			prefetchStarted,
			clearPendingTimer,
			callbacks: {
				beforeBegin: input.beforeBegin,
				beforeRender: input.beforeRender,
				afterRender: input.afterRender,
			},
			navigationOptions: {
				scrollToTop: input.scrollToTop,
				replace: input.replace,
				search: input.search,
				hash: input.hash,
				state: input.state,
			},
		});
	}

	return {
		...hrefDetails,
		start,
		stop,
		onClick,
	};
}

export { createLinkOnClickFn as __makeLinkOnClickFn };
export { createPrefetchHandlers as __getPrefetchHandlers };

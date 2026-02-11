import { getAnchorDetailsFromEvent, getHrefDetails } from "vorma/kit/url";
import { navigationStateManager, vormaNavigate } from "../client.ts";
import {
	hasSameDataTarget,
	isSameDocumentHashChange,
	isSameDocumentLocation,
} from "../platform/url.ts";
import type { NavigationOutcome } from "./navigation/types.ts";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "./redirects.ts";
import { saveScrollState } from "../platform/scroll.ts";
import { logError } from "../platform/safety.ts";

type HashCheckAnchorDetails =
	| {
			anchor: HTMLAnchorElement;
	  }
	| null
	| undefined;

function isJustAHashChange(anchorDetails: HashCheckAnchorDetails): boolean {
	if (!anchorDetails) return false;

	return isSameDocumentHashChange(
		anchorDetails.anchor.href,
		window.location.href,
	);
}

function isSameDocumentNoopNavigationTarget(
	anchorDetails: HashCheckAnchorDetails,
): boolean {
	if (!anchorDetails) return false;

	return isSameDocumentLocation(
		anchorDetails.anchor.href,
		window.location.href,
	);
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

async function handleLinkNavigationOutcome<E extends Event>(props: {
	event: E;
	outcome: NavigationOutcome;
	targetUrl: string;
	callbacks: LinkLifecycleCallbacks<E>;
}): Promise<void> {
	const { event, outcome, targetUrl, callbacks } = props;

	switch (outcome.type) {
		case "aborted":
			navigationStateManager.removeNavigation(targetUrl);
			return;

		case "redirect":
			await callbacks.beforeRender?.(event);
			syncBuildIDFromRedirectData(outcome.redirectData);
			navigationStateManager.removeNavigation(targetUrl);
			await effectuateRedirectDataResult(
				outcome.redirectData,
				outcome.props.redirectCount || 0,
				outcome.props,
			);
			await callbacks.afterRender?.(event);
			return;

		case "success": {
			await callbacks.beforeRender?.(event);
			const entry = navigationStateManager.getNavigation(targetUrl);
			if (entry) {
				await navigationStateManager.processSuccessfulNavigation(
					outcome,
					entry,
				);
			}
			await callbacks.afterRender?.(event);
			return;
		}

		default: {
			const exhaustiveCheck: never = outcome;
			throw new Error(
				`Unexpected outcome type: ${String(exhaustiveCheck)}`,
			);
		}
	}
}

export function createLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return async (event: E) => {
		if (event.defaultPrevented) return;

		const anchorDetails = getAnchorDetailsFromEvent(
			event as unknown as MouseEvent,
		);
		if (!anchorDetails) return;

		const { anchor, isEligibleForDefaultPrevention, isInternal } =
			anchorDetails;
		if (!anchor) return;

		if (!isEligibleForDefaultPrevention || !isInternal) {
			return;
		}

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

		if (!control.promise) return;

		const outcome = await control.promise;
		const targetUrl = new URL(anchor.href, window.location.href).href;
		await handleLinkNavigationOutcome({
			event,
			outcome,
			targetUrl,
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
			hasSameDataTarget(nav.targetUrl, targetHref)
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
	const fullUrl = new URL(props.relativeURL, window.location.href);
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
	if (event.defaultPrevented) return;

	const anchorDetails = getAnchorDetailsFromEvent(
		event as unknown as MouseEvent,
	);
	if (!anchorDetails) return;

	const { isEligibleForDefaultPrevention, isInternal } = anchorDetails;
	if (!isEligibleForDefaultPrevention || !isInternal) return;

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
		if (prefetchStarted && hasActiveIdlePrefetch()) return;
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

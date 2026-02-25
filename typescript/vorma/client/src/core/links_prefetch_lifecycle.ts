import { getHrefDetails } from "vorma/kit/url";
import { navigationStateManager } from "../client.ts";
import { logError } from "../platform/safety.ts";
import { hasSameNavigationTarget } from "../platform/url.ts";
import {
	type ClickNavigationOptions,
	type LinkOnClickCallbacks,
	getEligibleInternalAnchorDetails,
	navigateEligibleInternalAnchorClick,
} from "./links_click_lifecycle.ts";

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
			hasSameNavigationTarget({
				firstHref: nav.targetUrl,
				secondHref: targetHref,
			})
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

async function handlePrefetchClick<E extends Event>(props: {
	event: E;
	prefetchStarted: boolean;
	clearPendingTimer: () => void;
	callbacks: LinkOnClickCallbacks<E>;
	navigationOptions: ClickNavigationOptions;
}): Promise<void> {
	const {
		event,
		prefetchStarted,
		clearPendingTimer,
		callbacks,
		navigationOptions,
	} = props;
	const anchorDetails = getEligibleInternalAnchorDetails(event);
	if (!anchorDetails) return;

	clearPendingTimer();
	await navigateEligibleInternalAnchorClick({
		event,
		anchorDetails,
		beforeBegin: callbacks.beforeBegin,
		beforeRender: callbacks.beforeRender,
		afterRender: callbacks.afterRender,
		scrollToTop: navigationOptions.scrollToTop,
		replace: navigationOptions.replace,
		state: navigationOptions.state,
		shouldRunBeforeBegin: !prefetchStarted,
	});
}

export type CreatePrefetchHandlersInput<E extends Event> =
	LinkOnClickCallbacks<E> & {
		href: string;
		delayMs?: number;
		scrollToTop?: boolean;
		replace?: boolean;
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
	const targetHref = hrefDetails.absoluteURL;

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

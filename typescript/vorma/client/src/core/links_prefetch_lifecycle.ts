import {
	getHrefDetails,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "vorma/kit/url";
import { navigationStateManager, vormaNavigate } from "../client.ts";
import { hasSameNavigationTarget } from "../platform/url.ts";
import { saveScrollState } from "../platform/scroll.ts";
import { logError } from "../platform/safety.ts";
import {
	type ClickNavigationOptions,
	type LinkOnClickCallbacks,
	classifyEligibleAnchorTarget,
	getEligibleInternalAnchorDetails,
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

function buildPrefetchTargetHref(props: {
	relativeURL: string;
	search?: string;
	hash?: string;
}): string {
	return resolveAbsoluteHrefWithOptionalSearchAndHash({
		href: props.relativeURL,
		search: props.search,
		hash: props.hash,
	});
}

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

	const targetType = classifyEligibleAnchorTarget(anchorDetails);
	if (targetType === "same-document-noop") {
		event.preventDefault();
		clearPendingTimer();
		return;
	}

	if (targetType === "hash-change") {
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

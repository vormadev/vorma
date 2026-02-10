import { getAnchorDetailsFromEvent, getHrefDetails } from "vorma/kit/url";
import { navigationStateManager, vormaNavigate } from "./client.ts";
import { isJustAHashChange } from "./link_hash_change.ts";
import { buildPrefetchTargetHref } from "./link_prefetch_target_href.ts";
import { saveScrollState } from "./scroll_state_manager.ts";

type LinkOnClickCallback<E extends Event> = (event: E) => void | Promise<void>;

type LinkOnClickCallbacks<E extends Event> = {
	beforeBegin?: LinkOnClickCallback<E>;
	beforeRender?: LinkOnClickCallback<E>;
	afterRender?: LinkOnClickCallback<E>;
};

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

	// TypeScript type guard -- after this check, we know relativeURL exists
	const { relativeURL } = hrefDetails;
	if (!relativeURL || hrefDetails.isExternal) {
		return;
	}

	let timer: number | undefined;
	let prefetchStarted = false;
	const delayMs = input.delayMs ?? 100;

	function clearPendingTimer(): void {
		if (timer !== undefined) {
			clearTimeout(timer);
			timer = undefined;
		}
	}

	async function prefetch(e: E): Promise<void> {
		if (prefetchStarted) return;
		prefetchStarted = true;

		if (input.beforeBegin) {
			await input.beforeBegin(e);
		}

		const targetHref = buildPrefetchTargetHref({
			relativeURL,
			search: input.search,
			hash: input.hash,
		});

		// Use the navigation system
		await navigationStateManager.navigate({
			href: targetHref,
			navigationType: "prefetch",
			state: input.state,
		});
	}

	function start(e: E): void {
		if (prefetchStarted) return;
		timer = window.setTimeout(() => prefetch(e), delayMs);
	}

	function stop(): void {
		clearPendingTimer();

		// Abort prefetch if it exists and hasn't been upgraded
		const targetUrl = buildPrefetchTargetHref({
			relativeURL,
			search: input.search,
			hash: input.hash,
		});
		const nav = navigationStateManager.getNavigation(targetUrl);
		if (nav && nav.type === "prefetch" && nav.intent === "none") {
			nav.control.abortController?.abort();
			navigationStateManager.removeNavigation(targetUrl);
		}

		prefetchStarted = false;
	}

	async function onClick(e: E): Promise<void> {
		if (e.defaultPrevented) return;

		const anchorDetails = getAnchorDetailsFromEvent(
			e as unknown as MouseEvent,
		);
		if (!anchorDetails) return;

		const { isEligibleForDefaultPrevention, isInternal } = anchorDetails;
		if (!isEligibleForDefaultPrevention || !isInternal) return;

		if (isJustAHashChange(anchorDetails)) {
			saveScrollState();
			return;
		}

		e.preventDefault();

		clearPendingTimer();

		// Execute callbacks
		if (input.beforeBegin && !prefetchStarted) {
			await input.beforeBegin(e);
		}

		if (input.beforeRender) {
			await input.beforeRender(e);
		}

		// Use standard navigation -- it will upgrade the prefetch if it exists
		await vormaNavigate(relativeURL, {
			scrollToTop: input.scrollToTop,
			replace: input.replace,
			search: input.search,
			hash: input.hash,
			state: input.state,
		});

		if (input.afterRender) {
			await input.afterRender(e);
		}
	}

	return {
		...hrefDetails,
		start,
		stop,
		onClick,
	};
}

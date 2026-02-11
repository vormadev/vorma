import { getHrefDetails } from "vorma/kit/url";
import type { LinkOnClickCallbacks } from "./link_prefetch_callbacks.ts";
import { handlePrefetchClick } from "./link_prefetch_click.ts";
import {
	abortIdlePrefetchNavigation,
	startPrefetchNavigation,
} from "./link_prefetch_navigation.ts";
import { buildPrefetchTargetHref } from "./link_prefetch_target_href.ts";

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
	const targetHref = buildPrefetchTargetHref({
		relativeURL,
		search: input.search,
		hash: input.hash,
	});

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

		await startPrefetchNavigation({ targetHref, state: input.state });
	}

	function start(e: E): void {
		if (prefetchStarted) return;
		timer = window.setTimeout(() => prefetch(e), delayMs);
	}

	function stop(): void {
		clearPendingTimer();

		abortIdlePrefetchNavigation(targetHref);

		prefetchStarted = false;
	}

	async function onClick(e: E): Promise<void> {
		await handlePrefetchClick({
			event: e,
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

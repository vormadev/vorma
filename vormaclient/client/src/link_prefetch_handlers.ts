import { getHrefDetails } from "vorma/kit/url";
import type { LinkOnClickCallbacks } from "./link_prefetch_callbacks.ts";
import { handlePrefetchClick } from "./link_prefetch_click.ts";
import {
	abortIdlePrefetchNavigation,
	hasIdlePrefetchNavigation,
	startPrefetchNavigation,
} from "./link_prefetch_navigation.ts";
import { buildPrefetchTargetHref } from "./link_prefetch_target_href.ts";
import { logError } from "./utils/logging.ts";

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

	function hasActiveIdlePrefetch(): boolean {
		return hasIdlePrefetchNavigation(targetHref);
	}

	async function prefetch(e: E): Promise<void> {
		if (prefetchStarted && hasActiveIdlePrefetch()) return;
		prefetchStarted = true;

		try {
			if (input.beforeBegin) {
				await input.beforeBegin(e);
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

	function start(e: E): void {
		if (timer !== undefined) return;
		if (prefetchStarted && hasActiveIdlePrefetch()) return;
		prefetchStarted = false;
		timer = window.setTimeout(() => {
			timer = undefined;
			void prefetch(e);
		}, delayMs);
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

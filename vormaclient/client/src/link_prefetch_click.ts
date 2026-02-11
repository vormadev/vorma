import { getAnchorDetailsFromEvent } from "vorma/kit/url";
import { vormaNavigate } from "./client.ts";
import type { LinkOnClickCallbacks } from "./link_prefetch_callbacks.ts";
import { isJustAHashChange } from "./link_hash_change.ts";
import { saveScrollState } from "./scroll_state_manager.ts";

type ClickNavigationOptions = {
	scrollToTop?: boolean;
	replace?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

export async function handlePrefetchClick<E extends Event>(props: {
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

	if (isJustAHashChange(anchorDetails)) {
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

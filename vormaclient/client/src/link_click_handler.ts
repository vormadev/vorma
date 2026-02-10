import { getAnchorDetailsFromEvent } from "vorma/kit/url";
import { navigationStateManager } from "./client.ts";
import { isJustAHashChange } from "./link_hash_change.ts";
import { handleLinkNavigationOutcome } from "./link_navigation_outcome.ts";
import { saveScrollState } from "./scroll_state_manager.ts";

type LinkOnClickCallback<E extends Event> = (event: E) => void | Promise<void>;

export type LinkOnClickCallbacks<E extends Event> = {
	beforeBegin?: LinkOnClickCallback<E>;
	beforeRender?: LinkOnClickCallback<E>;
	afterRender?: LinkOnClickCallback<E>;
};

export function createLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return async (e: E) => {
		if (e.defaultPrevented) return;

		const anchorDetails = getAnchorDetailsFromEvent(
			e as unknown as MouseEvent,
		);
		if (!anchorDetails) return;

		const { anchor, isEligibleForDefaultPrevention, isInternal } =
			anchorDetails;
		if (!anchor) return;

		if (isJustAHashChange(anchorDetails)) {
			saveScrollState();
			return;
		}

		if (isEligibleForDefaultPrevention && isInternal) {
			e.preventDefault();

			await callbacks.beforeBegin?.(e);

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
				event: e,
				outcome,
				targetUrl,
				callbacks: {
					beforeRender: callbacks.beforeRender,
					afterRender: callbacks.afterRender,
				},
			});
		}
	};
}

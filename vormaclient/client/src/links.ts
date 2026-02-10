import {
	createLinkOnClickFn,
	type LinkOnClickCallbacks,
} from "./link_click_handler.ts";
import {
	createPrefetchHandlers,
	type CreatePrefetchHandlersInput,
} from "./link_prefetch_handlers.ts";

export function __getPrefetchHandlers<E extends Event>(
	input: CreatePrefetchHandlersInput<E>,
) {
	return createPrefetchHandlers(input);
}

export function __makeLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return createLinkOnClickFn(callbacks);
}

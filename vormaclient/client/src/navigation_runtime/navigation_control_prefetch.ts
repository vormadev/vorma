import { createPrefetchNavigationEntry } from "./navigation_entry_factory.ts";
import type { CreateNavigationControlsContext } from "./navigation_controls.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";

export function createPrefetchControl(
	context: CreateNavigationControlsContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	const { fetchRouteData, prefetchCache } = context;

	const entry = createPrefetchNavigationEntry({
		props,
		targetUrl,
		fetchRouteData,
		onFetchError: () => {
			prefetchCache.delete(targetUrl);
		},
	});

	prefetchCache.set(targetUrl, entry);
	// No status update needed - prefetches don't affect status
	return entry.control;
}

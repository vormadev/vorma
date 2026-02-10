import { createRevalidationNavigationEntry } from "./navigation_entry_factory.ts";
import type { CreateNavigationControlsContext } from "./navigation_controls.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";

export function createRevalidationControl(
	context: CreateNavigationControlsContext,
	props: NavigateProps,
): NavigationControl {
	const {
		fetchRouteData,
		getPendingRevalidation,
		setPendingRevalidation,
		scheduleStatusUpdate,
	} = context;

	const targetUrl = new URL(props.href, window.location.href).href;
	const entry = createRevalidationNavigationEntry({
		props,
		fetchRouteData,
		onFetchError: () => {
			const pendingRevalidation = getPendingRevalidation();
			if (pendingRevalidation?.targetUrl === targetUrl) {
				setPendingRevalidation(null);
				scheduleStatusUpdate();
			}
		},
	});

	setPendingRevalidation(entry);
	scheduleStatusUpdate();
	return entry.control;
}

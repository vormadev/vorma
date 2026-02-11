import { createRevalidationNavigationEntry } from "./navigation_entry_factory.ts";
import type { CreateNavigationControlsContext } from "./navigation_controls.ts";
import { resolveNavigationTargetURL } from "./target_url.ts";
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

	const targetUrl = resolveNavigationTargetURL(props.href);
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

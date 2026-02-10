import { createActiveNavigationEntry } from "./navigation_entry_factory.ts";
import type { CreateNavigationControlsContext } from "./navigation_controls.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationIntent,
} from "./types.ts";

export function createActiveNavigationControl(
	context: CreateNavigationControlsContext,
	props: NavigateProps,
	intent: NavigationIntent,
): NavigationControl {
	const {
		fetchRouteData,
		setActiveNavigation,
		scheduleStatusUpdate,
		deleteNavigation,
	} = context;

	const targetUrl = new URL(props.href, window.location.href).href;
	const entry = createActiveNavigationEntry({
		props,
		intent,
		fetchRouteData,
		onFetchError: () => {
			deleteNavigation(targetUrl);
		},
	});

	setActiveNavigation(entry);
	scheduleStatusUpdate();
	return entry.control;
}

import type { RouteChangeEvent } from "../platform/events.ts";
import {
	addLocationListener,
	addRouteChangeListener,
} from "../platform/events.ts";
import { applyScrollState } from "../platform/scroll.ts";

export type RouteOutletRuntimeStoreSyncFunction = () => void;

export type RouteOutletRuntimeListenerInitializer = () => void;

/**
 * Builds a once-only listener initializer for route and location updates.
 *
 * Each adapter keeps its own singleton initializer so remounts don't duplicate
 * global listeners.
 */
export function createRouteOutletRuntimeListenerInitializer(props: {
	syncStoreState: RouteOutletRuntimeStoreSyncFunction;
}): RouteOutletRuntimeListenerInitializer {
	const { syncStoreState } = props;
	let isInitialized = false;

	return () => {
		if (isInitialized) {
			return;
		}
		isInitialized = true;

		addRouteChangeListener((event: RouteChangeEvent) => {
			syncStoreState();
			window.requestAnimationFrame(() => {
				applyScrollState(event.detail.__scrollState);
			});
		});

		addLocationListener(() => {
			syncStoreState();
		});
	};
}

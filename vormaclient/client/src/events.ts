import type { ScrollState } from "./scroll_state_manager.ts";

// Route Change Event
export const VORMA_ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
export type RouteChangeEvent = CustomEvent<RouteChangeEventDetail>;
export type RouteChangeEventDetail = { __scrollState?: ScrollState };
export const addRouteChangeListener = makeListenerAdder<RouteChangeEventDetail>(
	VORMA_ROUTE_CHANGE_EVENT_KEY,
);
export function dispatchRouteChangeEvent(detail: RouteChangeEventDetail): void {
	window.dispatchEvent(
		new CustomEvent(VORMA_ROUTE_CHANGE_EVENT_KEY, { detail }),
	);
}

// Status Event
const STATUS_EVENT_KEY = "vorma:status";
export type StatusEvent = CustomEvent<StatusEventDetail>;
export type StatusEventDetail = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};
export function dispatchStatusEvent(detail: StatusEventDetail): void {
	window.dispatchEvent(new CustomEvent(STATUS_EVENT_KEY, { detail }));
}
export const addStatusListener =
	makeListenerAdder<StatusEventDetail>(STATUS_EVENT_KEY);

// Build ID Event
const BUILD_ID_EVENT_KEY = "vorma:build-id";
type BuildIDEventDetail = { oldID: string; newID: string };
export function dispatchBuildIDEvent(detail: BuildIDEventDetail): void {
	window.dispatchEvent(new CustomEvent(BUILD_ID_EVENT_KEY, { detail }));
}
export const addBuildIDListener =
	makeListenerAdder<BuildIDEventDetail>(BUILD_ID_EVENT_KEY);

// Location Event
const LOCATION_EVENT_KEY = "vorma:location";
export function dispatchLocationEvent(): void {
	window.dispatchEvent(new CustomEvent(LOCATION_EVENT_KEY));
}
export const addLocationListener = makeListenerAdder<void>(LOCATION_EVENT_KEY);

// Helper to create listener adders
function makeListenerAdder<T>(key: string) {
	return function addListener(
		listener: (event: CustomEvent<T>) => void,
	): () => void {
		window.addEventListener(key, listener as any);
		return () => window.removeEventListener(key, listener as any);
	};
}

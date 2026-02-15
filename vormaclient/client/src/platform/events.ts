import type { ScrollState } from "./scroll.ts";

// Route Change Event
export const VORMA_ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
export type RouteChangeEvent = CustomEvent<RouteChangeEventDetail>;
export type RouteChangeEventDetail = { __scrollState?: ScrollState };
export const addRouteChangeListener = makeListenerAdder<RouteChangeEventDetail>(
	VORMA_ROUTE_CHANGE_EVENT_KEY,
);
export function dispatchRouteChangeEvent(detail: RouteChangeEventDetail): void {
	dispatchCustomEventWithDetail(VORMA_ROUTE_CHANGE_EVENT_KEY, detail);
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
	dispatchCustomEventWithDetail(STATUS_EVENT_KEY, detail);
}
export const addStatusListener =
	makeListenerAdder<StatusEventDetail>(STATUS_EVENT_KEY);

// Build ID Event
const BUILD_ID_EVENT_KEY = "vorma:build-id";
type BuildIDEventDetail = { oldID: string; newID: string };
export function dispatchBuildIDEvent(detail: BuildIDEventDetail): void {
	dispatchCustomEventWithDetail(BUILD_ID_EVENT_KEY, detail);
}
export const addBuildIDListener =
	makeListenerAdder<BuildIDEventDetail>(BUILD_ID_EVENT_KEY);

// Location Event
const LOCATION_EVENT_KEY = "vorma:location";
export function dispatchLocationEvent(): void {
	dispatchCustomEventWithoutDetail(LOCATION_EVENT_KEY);
}
export const addLocationListener = makeListenerAdder<void>(LOCATION_EVENT_KEY);

// Helper to create listener adders
function dispatchCustomEventWithDetail<T>(eventKey: string, detail: T): void {
	window.dispatchEvent(new CustomEvent(eventKey, { detail }));
}

function dispatchCustomEventWithoutDetail(eventKey: string): void {
	window.dispatchEvent(new CustomEvent(eventKey));
}

function makeListenerAdder<T>(key: string) {
	return function addListener(
		listener: (event: CustomEvent<T>) => void,
	): () => void {
		const wrappedListener: EventListener = (event) => {
			listener(event as CustomEvent<T>);
		};
		window.addEventListener(key, wrappedListener);
		return () => window.removeEventListener(key, wrappedListener);
	};
}

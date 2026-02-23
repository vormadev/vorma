import type { ScrollState } from "./scroll.ts";

// Route Change Event
export const VORMA_ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
export type RouteChangeEvent = CustomEvent<RouteChangeEventDetail>;
export type RouteChangeEventDetail = { __scrollState?: ScrollState };
export const addRouteChangeListener = makeListenerAdder<RouteChangeEventDetail>(
	VORMA_ROUTE_CHANGE_EVENT_KEY,
);
export function dispatchRouteChangeEvent(detail: RouteChangeEventDetail): void {
	dispatchCustomEvent({
		detail,
		eventKey: VORMA_ROUTE_CHANGE_EVENT_KEY,
	});
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
	dispatchCustomEvent({
		detail,
		eventKey: STATUS_EVENT_KEY,
	});
}
export const addStatusListener =
	makeListenerAdder<StatusEventDetail>(STATUS_EVENT_KEY);

// Build ID Event
const BUILD_ID_EVENT_KEY = "vorma:build-id";
type BuildIDEventDetail = { oldID: string; newID: string };
export function dispatchBuildIDEvent(detail: BuildIDEventDetail): void {
	dispatchCustomEvent({
		detail,
		eventKey: BUILD_ID_EVENT_KEY,
	});
}
export const addBuildIDListener =
	makeListenerAdder<BuildIDEventDetail>(BUILD_ID_EVENT_KEY);

// Location Event
const LOCATION_EVENT_KEY = "vorma:location";
export function dispatchLocationEvent(): void {
	dispatchCustomEvent({ eventKey: LOCATION_EVENT_KEY });
}
export const addLocationListener = makeListenerAdder<void>(LOCATION_EVENT_KEY);

function getWindowEventTargetOrNull(): Window | null {
	return typeof window !== "undefined" &&
		typeof window.dispatchEvent === "function" &&
		typeof window.addEventListener === "function" &&
		typeof window.removeEventListener === "function"
		? window
		: null;
}

// Helper to create listener adders
function dispatchCustomEvent<T>(props: { eventKey: string; detail?: T }): void {
	const eventTarget = getWindowEventTargetOrNull();
	if (!eventTarget) {
		return;
	}

	if (props.detail === undefined) {
		eventTarget.dispatchEvent(new CustomEvent(props.eventKey));
		return;
	}

	eventTarget.dispatchEvent(
		new CustomEvent(props.eventKey, { detail: props.detail }),
	);
}

function makeListenerAdder<T>(key: string) {
	return function addListener(
		listener: (event: CustomEvent<T>) => void,
	): () => void {
		const eventTarget = getWindowEventTargetOrNull();
		if (!eventTarget) {
			return () => {};
		}

		const wrappedListener: EventListener = (event) => {
			listener(event as CustomEvent<T>);
		};
		eventTarget.addEventListener(key, wrappedListener);
		return () => eventTarget.removeEventListener(key, wrappedListener);
	};
}

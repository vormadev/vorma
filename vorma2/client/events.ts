/// <reference types="vite/client" />

import { ensure_global } from "./global_state.ts";
import type { RouteChangeEventDetail, StatusEventDetail } from "./types.ts";

const STATUS_EVENT = "vorma:status";
const ROUTE_CHANGE_EVENT = "vorma:route-change";
const BUILD_ID_EVENT = "vorma:build-id";

function add_listener<E extends Event>(
	name: string,
	listener: (e: E) => void,
): () => void {
	const g = ensure_global();
	const by_name = g.window_listeners ?? new Map<string, Set<EventListener>>();
	g.window_listeners = by_name;
	const fn = listener as EventListener;
	let set = by_name.get(name);
	if (!set) {
		set = new Set();
		by_name.set(name, set);
	}
	set.add(fn);
	window.addEventListener(name, fn);
	return () => {
		window.removeEventListener(name, fn);
		set!.delete(fn);
		if (set!.size === 0) by_name.delete(name);
	};
}

function dispatch<D>(name: string, detail: D): void {
	window.dispatchEvent(new CustomEvent<D>(name, { detail }));
}

export function dispatch_status(detail: StatusEventDetail): void {
	dispatch(STATUS_EVENT, detail);
}
export function addStatusListener(
	listener: (e: CustomEvent<StatusEventDetail>) => void,
): () => void {
	return add_listener(STATUS_EVENT, listener);
}

export function dispatch_route_change(detail: RouteChangeEventDetail): void {
	dispatch(ROUTE_CHANGE_EVENT, detail);
}
export function addRouteChangeListener(
	listener: (e: CustomEvent<RouteChangeEventDetail>) => void,
): () => void {
	return add_listener(ROUTE_CHANGE_EVENT, listener);
}

export function dispatch_build_id(detail: {
	oldID: string;
	newID: string;
}): void {
	dispatch(BUILD_ID_EVENT, detail);
}
export function addBuildIDListener(
	listener: (e: CustomEvent<{ oldID: string; newID: string }>) => void,
): () => void {
	return add_listener(BUILD_ID_EVENT, listener);
}

export function add_window_listener(
	name: string,
	listener: EventListener,
): () => void {
	return add_listener(name, listener);
}

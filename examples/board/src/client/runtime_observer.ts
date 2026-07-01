import type {
	BuildSkewDetectedEvent,
	RouteState,
	RouteUpdateReason,
	WorkState,
} from "vorma/react";

const runtime_observer_event = "board-runtime-observer-update";

/*
The client callbacks configured in `app.tsx` can push route/work/build
events into any app-owned observer. Board keeps a tiny module-level store
so the layout can display those events without making Vorma depend on a
specific React state library.
*/
export type RuntimeObserverSnapshot = {
	build_skew: string | null;
	route_href: string | null;
	route_reason: RouteUpdateReason | null;
	update_count: number;
	work_active: boolean;
};

let snapshot: RuntimeObserverSnapshot = {
	build_skew: null,
	route_href: null,
	route_reason: null,
	update_count: 0,
	work_active: false,
};

function publish_snapshot(next: RuntimeObserverSnapshot): void {
	snapshot = next;
	window.dispatchEvent(new Event(runtime_observer_event));
}

export function record_route_update(
	route: RouteState,
	_previous_route: RouteState | null,
	reason: RouteUpdateReason,
): void {
	publish_snapshot({
		...snapshot,
		route_href: route.href,
		route_reason: reason,
		update_count: snapshot.update_count + 1,
	});
}

export function record_work_update(work: WorkState): void {
	publish_snapshot({
		...snapshot,
		work_active:
			work.navigation !== null ||
			work.revalidation !== null ||
			work.prefetch !== null ||
			work.apiRequests.length > 0,
	});
}

export function record_build_skew(event: BuildSkewDetectedEvent): void {
	publish_snapshot({
		...snapshot,
		build_skew: `${event.activeClientBuildId} -> ${event.serverBuildId}`,
	});
}

export function read_runtime_observer_snapshot(): RuntimeObserverSnapshot {
	return snapshot;
}

export function subscribe_runtime_observer(listener: () => void): () => void {
	window.addEventListener(runtime_observer_event, listener);
	return () => {
		window.removeEventListener(runtime_observer_event, listener);
	};
}

type ProbeRoute = {
	href: string;
	client_build_id: string;
	match_patterns: string[];
	error_source: string | null;
	error_index: number | null;
	params: Record<string, string>;
};

type ProbeWork = {
	navigation_href: string | null;
	revalidation_status: string | null;
	prefetch_href: string | null;
	submission_count: number;
};

type ProbeBuildSkew = {
	active_client_build_id: string;
	server_build_id: string;
	default_behavior: string;
	response_kind: string;
	response_trigger: string | null;
	api_route_kind: string | null;
	revalidation_reason: string | null;
	requested_href: string;
	status: number;
	ok: boolean;
	current_route_href: string;
	current_work_navigation_href: string | null;
	current_work_revalidation_status: string | null;
	current_work_prefetch_href: string | null;
	current_work_submission_count: number;
};

type ProbeNavigationTiming = {
	href: string | null;
	pending_started_at_ms: number | null;
	route_committed_at_ms: number | null;
	dom_settled_at_ms: number | null;
	work_cleared_at_ms: number | null;
	pending_to_route_ms: number | null;
	pending_to_dom_ms: number | null;
	route_to_dom_ms: number | null;
	pending_to_clear_ms: number | null;
	route_to_clear_ms: number | null;
};

type Probe = {
	variant: string;
	route: ProbeRoute | null;
	work: ProbeWork | null;
	navigation_timing: ProbeNavigationTiming;
	route_updates: number;
	work_updates: number;
	build_skew_detections: number;
	api_build_skew_detections: number;
	query_build_skew_detections: number;
	mutation_build_skew_detections: number;
	failed_api_build_skew_detections: number;
	manual_revalidation_build_skew_detections: number;
	last_build_skew_server_id: string | null;
	last_build_skew: ProbeBuildSkew | null;
	last_api_build_skew: ProbeBuildSkew | null;
	last_reason: string | null;
	revalidate: () => Promise<unknown>;
};

type BootResult =
	| {
			ok: true;
	  }
	| {
			ok: false;
			err: string;
	  };

type RouteLike = {
	href: string;
	clientBuildID: string;
	matches: Array<{ pattern: string }>;
	error: null | { source: string; idx: number };
	params: Record<string, string>;
};

type WorkLike = {
	navigation: null | { href: string };
	revalidation: null | { status: string };
	prefetch: null | { href: string };
	apiRequests: unknown[];
};

type BuildSkewEventLike = {
	activeClientBuildID: string;
	serverBuildID: string;
	defaultBehavior: string;
	triggeringResponse:
		| {
				kind: "route";
				trigger: string;
				revalidationReason?: string;
				requestedHref: string;
				status: number;
				ok: boolean;
		  }
		| {
				kind: "apiRoute";
				apiRouteKind: string;
				requestedHref: string;
				status: number;
				ok: boolean;
		  };
	currentRouteState: RouteLike;
	currentWorkState: WorkLike;
};

declare global {
	interface Window {
		__vorma_bombadil?: Probe;
	}
}

let current_probe: Probe | null = null;

export function on_vorma_route_update(
	route: RouteLike,
	_previous_route: RouteLike | null,
	reason: string,
): void {
	if (current_probe == null) {
		return;
	}
	record_route_timing(route);
	current_probe.route = serialize_route(route);
	current_probe.route_updates += 1;
	current_probe.last_reason = reason;
}

export function on_vorma_work_update(work: WorkLike): void {
	if (current_probe == null) {
		return;
	}
	const previous_work = current_probe.work;
	const next_work = serialize_work(work);
	record_work_timing(previous_work, next_work);
	current_probe.work = next_work;
	current_probe.work_updates += 1;
}

export function on_vorma_build_skew_detected(event: BuildSkewEventLike): void {
	if (current_probe == null) {
		return;
	}
	current_probe.build_skew_detections += 1;
	const serialized_event = serialize_build_skew(event);
	if (
		event.triggeringResponse.kind === "route" &&
		event.triggeringResponse.trigger === "revalidation" &&
		event.triggeringResponse.revalidationReason === "manual"
	) {
		current_probe.manual_revalidation_build_skew_detections += 1;
	}
	if (event.triggeringResponse.kind === "apiRoute") {
		current_probe.api_build_skew_detections += 1;
		current_probe.last_api_build_skew = serialized_event;
		if (event.triggeringResponse.apiRouteKind === "query") {
			current_probe.query_build_skew_detections += 1;
		}
		if (event.triggeringResponse.apiRouteKind === "mutation") {
			current_probe.mutation_build_skew_detections += 1;
		}
		if (!event.triggeringResponse.ok) {
			current_probe.failed_api_build_skew_detections += 1;
		}
	}
	current_probe.last_build_skew_server_id = event.serverBuildID;
	current_probe.last_build_skew = serialized_event;
}

export async function install_vorma_probe(input: {
	variant: string;
	app: unknown;
}): Promise<void> {
	const app = input.app as {
		boot: () => Promise<BootResult>;
		revalidate: () => Promise<unknown>;
		getRouteState: () => RouteLike;
		getWorkState: () => WorkLike;
	};
	const probe: Probe = {
		variant: input.variant,
		route: null,
		work: null,
		navigation_timing: empty_navigation_timing(),
		route_updates: 0,
		work_updates: 0,
		build_skew_detections: 0,
		api_build_skew_detections: 0,
		query_build_skew_detections: 0,
		mutation_build_skew_detections: 0,
		failed_api_build_skew_detections: 0,
		manual_revalidation_build_skew_detections: 0,
		last_build_skew_server_id: null,
		last_build_skew: null,
		last_api_build_skew: null,
		last_reason: null,
		revalidate: app.revalidate,
	};
	current_probe = probe;
	window.__vorma_bombadil = probe;

	const result = await app.boot();
	if (!result.ok) {
		throw new Error(result.err);
	}
	probe.route = serialize_route(app.getRouteState());
	probe.work = serialize_work(app.getWorkState());
}

function empty_navigation_timing(): ProbeNavigationTiming {
	return {
		href: null,
		pending_started_at_ms: null,
		route_committed_at_ms: null,
		dom_settled_at_ms: null,
		work_cleared_at_ms: null,
		pending_to_route_ms: null,
		pending_to_dom_ms: null,
		route_to_dom_ms: null,
		pending_to_clear_ms: null,
		route_to_clear_ms: null,
	};
}

function record_route_timing(route: RouteLike): void {
	const probe = current_probe;
	if (probe == null) {
		return;
	}
	const timing = probe.navigation_timing;
	if (timing.href == null || route.href !== timing.href) {
		return;
	}
	const now = performance.now();
	timing.route_committed_at_ms = now;
	if (timing.pending_started_at_ms != null) {
		timing.pending_to_route_ms = now - timing.pending_started_at_ms;
	}
	schedule_dom_settle_timing(route.href);
}

function schedule_dom_settle_timing(href: string): void {
	const probe = current_probe;
	if (probe == null || probe.navigation_timing.href !== href) {
		return;
	}
	requestAnimationFrame(() => {
		const current_probe_after_frame = current_probe;
		if (
			current_probe_after_frame == null ||
			current_probe_after_frame.navigation_timing.href !== href
		) {
			return;
		}

		const current_href_text =
			document.querySelector("[data-bmb-current-href]")?.textContent ??
			"";
		if (window.location.href !== href || current_href_text !== href) {
			return;
		}

		const timing = current_probe_after_frame.navigation_timing;
		const now = performance.now();
		timing.dom_settled_at_ms = now;
		if (timing.pending_started_at_ms != null) {
			timing.pending_to_dom_ms = now - timing.pending_started_at_ms;
		}
		if (timing.route_committed_at_ms != null) {
			timing.route_to_dom_ms = now - timing.route_committed_at_ms;
		}
	});
}

function record_work_timing(
	previous_work: ProbeWork | null,
	next_work: ProbeWork,
): void {
	const probe = current_probe;
	if (probe == null) {
		return;
	}
	const previous_navigation_href = previous_work?.navigation_href ?? null;
	const next_navigation_href = next_work.navigation_href;
	if (previous_navigation_href === next_navigation_href) {
		return;
	}

	const now = performance.now();
	if (next_navigation_href != null) {
		probe.navigation_timing = {
			...empty_navigation_timing(),
			href: next_navigation_href,
			pending_started_at_ms: now,
		};
		return;
	}
	if (
		previous_navigation_href == null ||
		probe.navigation_timing.href !== previous_navigation_href
	) {
		return;
	}

	const timing = probe.navigation_timing;
	timing.work_cleared_at_ms = now;
	if (timing.pending_started_at_ms != null) {
		timing.pending_to_clear_ms = now - timing.pending_started_at_ms;
	}
	if (timing.route_committed_at_ms != null) {
		timing.route_to_clear_ms = now - timing.route_committed_at_ms;
	}
}

function serialize_route(route: RouteLike): ProbeRoute {
	return {
		href: route.href,
		client_build_id: route.clientBuildID,
		match_patterns: route.matches.map((match) => {
			return match.pattern;
		}),
		error_source: route.error?.source ?? null,
		error_index: route.error?.idx ?? null,
		params: route.params,
	};
}

function serialize_build_skew(event: BuildSkewEventLike): ProbeBuildSkew {
	const work = serialize_work(event.currentWorkState);
	return {
		active_client_build_id: event.activeClientBuildID,
		server_build_id: event.serverBuildID,
		default_behavior: event.defaultBehavior,
		response_kind: event.triggeringResponse.kind,
		response_trigger:
			event.triggeringResponse.kind === "route"
				? event.triggeringResponse.trigger
				: null,
		api_route_kind:
			event.triggeringResponse.kind === "apiRoute"
				? event.triggeringResponse.apiRouteKind
				: null,
		revalidation_reason:
			event.triggeringResponse.kind === "route"
				? (event.triggeringResponse.revalidationReason ?? null)
				: null,
		requested_href: event.triggeringResponse.requestedHref,
		status: event.triggeringResponse.status,
		ok: event.triggeringResponse.ok,
		current_route_href: event.currentRouteState.href,
		current_work_navigation_href: work.navigation_href,
		current_work_revalidation_status: work.revalidation_status,
		current_work_prefetch_href: work.prefetch_href,
		current_work_submission_count: work.submission_count,
	};
}

function serialize_work(work: WorkLike): ProbeWork {
	return {
		navigation_href: work.navigation?.href ?? null,
		revalidation_status: work.revalidation?.status ?? null,
		prefetch_href: work.prefetch?.href ?? null,
		submission_count: work.apiRequests.length,
	};
}

import {
	actions,
	always,
	eventually,
	extract,
	now,
	type Action,
} from "@antithesishq/bombadil";

export {
	noConsoleErrors,
	noHttpErrorCodes,
	noUncaughtExceptions,
	noUnhandledPromiseRejections,
} from "@antithesishq/bombadil/defaults/properties";

type SkewState = {
	probe_ready: boolean;
	expected_operation: string;
	expected_deployment: string;
	route_client_build_id: string | null;
	view_deployment: string;
	client_build_tag: string;
	pending_href: string;
	revalidation_status: string;
	submission_count: number;
	manual_revalidation_build_skew_detections: number;
	query_build_skew_detections: number;
	mutation_build_skew_detections: number;
	failed_api_build_skew_detections: number;
	last_build_skew_server_id: string | null;
	last_build_skew_active_client_id: string | null;
	last_build_skew_response_kind: string | null;
	last_build_skew_requested_href: string | null;
	last_build_skew_status: number | null;
	last_build_skew_ok: boolean | null;
	last_api_build_skew_server_id: string | null;
	last_api_build_skew_active_client_id: string | null;
	last_api_build_skew_resource_kind: string | null;
	last_api_build_skew_status: number | null;
	last_api_build_skew_ok: boolean | null;
};

type ClickTarget = {
	name: string;
	content: string;
	x: number;
	y: number;
};

const skew_state = extract((state): SkewState => {
	const probe = state.window.__vorma_bombadil ?? null;
	return {
		probe_ready: probe !== null && probe.route !== null && probe.work !== null,
		expected_operation:
			state.document.querySelector("[data-bmb-expected-operation]")?.textContent ??
			"",
		expected_deployment:
			state.document.querySelector("[data-bmb-expected-deployment]")?.textContent ??
			"",
		route_client_build_id: probe?.route?.client_build_id ?? null,
		view_deployment:
			state.document.querySelector("[data-bmb-view-deployment]")?.textContent ?? "",
		client_build_tag:
			state.document.querySelector("[data-bmb-client-build-tag]")?.textContent ??
			"",
		pending_href: probe?.work?.navigation_href ?? "",
		revalidation_status: probe?.work?.revalidation_status ?? "",
		submission_count: probe?.work?.submission_count ?? 0,
		manual_revalidation_build_skew_detections:
			probe?.manual_revalidation_build_skew_detections ?? 0,
		query_build_skew_detections: probe?.query_build_skew_detections ?? 0,
		mutation_build_skew_detections: probe?.mutation_build_skew_detections ?? 0,
		failed_api_build_skew_detections: probe?.failed_api_build_skew_detections ?? 0,
		last_build_skew_server_id: probe?.last_build_skew_server_id ?? null,
		last_build_skew_active_client_id:
			probe?.last_build_skew?.active_client_build_id ?? null,
		last_build_skew_response_kind: probe?.last_build_skew?.response_kind ?? null,
		last_build_skew_requested_href: probe?.last_build_skew?.requested_href ?? null,
		last_build_skew_status: probe?.last_build_skew?.status ?? null,
		last_build_skew_ok: probe?.last_build_skew?.ok ?? null,
		last_api_build_skew_server_id:
			probe?.last_api_build_skew?.server_build_id ?? null,
		last_api_build_skew_active_client_id:
			probe?.last_api_build_skew?.active_client_build_id ?? null,
		last_api_build_skew_resource_kind:
			probe?.last_api_build_skew?.resource_kind ?? null,
		last_api_build_skew_status: probe?.last_api_build_skew?.status ?? null,
		last_api_build_skew_ok: probe?.last_api_build_skew?.ok ?? null,
	};
});

const click_targets = extract((state): ClickTarget[] => {
	return Array.from(
		state.document.querySelectorAll<HTMLElement>("[data-bmb-action]"),
	).flatMap((el) => {
		const rect = el.getBoundingClientRect();
		if (rect.width <= 0 || rect.height <= 0) {
			return [];
		}
		return [
			{
				name: el.getAttribute("data-bmb-action") ?? "",
				content: el.textContent?.trim() ?? "",
				x: rect.left + rect.width / 2,
				y: rect.top + rect.height / 2,
			},
		];
	});
});

function work_is_busy(): boolean {
	return (
		skew_state.current.pending_href !== "" ||
		skew_state.current.revalidation_status !== "" ||
		skew_state.current.submission_count > 0
	);
}

function click_action(name: string): Action[] {
	const target = click_targets.current.find((candidate) => {
		return candidate.name === name;
	});
	if (target == null) {
		return ["Wait"];
	}
	return [
		{
			Click: {
				name: target.name,
				content: target.content,
				point: { x: target.x, y: target.y },
			},
		},
	];
}

function build_skew_context_is_populated(): boolean {
	return (
		skew_state.current.last_build_skew_server_id !== null &&
		skew_state.current.last_build_skew_active_client_id !== null &&
		skew_state.current.last_build_skew_response_kind !== null &&
		skew_state.current.last_build_skew_requested_href !== null &&
		skew_state.current.last_build_skew_status !== null &&
		skew_state.current.last_build_skew_ok !== null
	);
}

function latest_api_skew_is(resource_kind: string, ok: boolean): boolean {
	return (
		skew_state.current.route_client_build_id !== null &&
		skew_state.current.last_api_build_skew_server_id !== null &&
		skew_state.current.last_api_build_skew_active_client_id !== null &&
		skew_state.current.last_api_build_skew_resource_kind === resource_kind &&
		skew_state.current.last_api_build_skew_ok === ok &&
		skew_state.current.route_client_build_id ===
			skew_state.current.last_api_build_skew_active_client_id &&
		skew_state.current.route_client_build_id !==
			skew_state.current.last_api_build_skew_server_id
	);
}

function skew_smoke_complete(): boolean {
	return (
		skew_state.current.manual_revalidation_build_skew_detections > 0 &&
		skew_state.current.query_build_skew_detections > 0 &&
		skew_state.current.mutation_build_skew_detections > 0 &&
		skew_state.current.failed_api_build_skew_detections > 0 &&
		skew_state.current.expected_operation === "mutation-error-error" &&
		build_skew_context_is_populated() &&
		latest_api_skew_is("mutation", false)
	);
}

export const vorma_build_skew_smoke_reaches_all_stale_client_paths = always(
	now(() => {
		return skew_state.current.probe_ready;
	}).implies(
		eventually(() => {
			return skew_smoke_complete();
		}).within(20, "seconds"),
	),
);

export const vorma_build_skew_smoke_actions = actions(() => {
	if (!skew_state.current.probe_ready || work_is_busy()) {
		return ["Wait"];
	}

	if (skew_smoke_complete()) {
		return ["Wait"];
	}

	if (
		skew_state.current.expected_operation === "" ||
		skew_state.current.expected_operation === "switch"
	) {
		return click_action("switch-revalidate");
	}

	if (skew_state.current.expected_operation === "revalidate-build_skew") {
		return click_action("switch-query");
	}

	if (skew_state.current.expected_operation === "query-ok") {
		return click_action("switch-mutation");
	}

	if (skew_state.current.expected_operation === "mutation-ok") {
		return click_action("switch-mutation-fail");
	}

	return ["Wait"];
});

import {
	actions,
	always,
	eventually,
	extract,
	now,
} from "@antithesishq/bombadil";

export * from "@antithesishq/bombadil/defaults";

type FixtureState = {
	is_dev: boolean;
	title: string;
	root_count: number;
	data_script_count: number;
	data_script_parses: boolean;
	shell_count: number;
	variant: string | null;
	route_deployment: string;
	client_build_tag: string;
	switch_result: string;
	expected_deployment: string;
	expected_operation: string;
	pathname: string;
	search_n: number | null;
	search_delay_ms: number | null;
	current_href_text: string;
	location_href: string;
	probe_href: string | null;
	route_client_build_id: string | null;
	probe_ready: boolean;
	rendered_route: string | null;
	counter_value_text: string;
	count_action_next_text: string;
	slow_delay_text: string;
	item_id: string | null;
	client_id: string | null;
	client_server_id_text: string;
	client_server_stamp_text: string;
	client_loader_id_text: string;
	client_loader_trigger_text: string;
	client_loader_stamp_text: string;
	echo_operation: string;
	nested_layout_count: number;
	nested_section: string | null;
	nested_detail_id: string | null;
	nested_detail_section_text: string;
	echo_output: string;
	error_text: string;
	pending_href: string;
	revalidation_status: string;
	prefetch_href: string;
	submission_count: number;
	last_navigation_timing_href: string | null;
	last_navigation_pending_to_route_ms: number | null;
	last_navigation_pending_to_dom_ms: number | null;
	last_navigation_route_to_dom_ms: number | null;
	last_navigation_pending_to_clear_ms: number | null;
	last_navigation_route_to_clear_ms: number | null;
	build_skew_detections: number;
	api_build_skew_detections: number;
	query_build_skew_detections: number;
	mutation_build_skew_detections: number;
	failed_api_build_skew_detections: number;
	manual_revalidation_build_skew_detections: number;
	last_build_skew_server_id: string | null;
	last_build_skew_active_client_id: string | null;
	last_build_skew_response_kind: string | null;
	last_build_skew_response_trigger: string | null;
	last_build_skew_api_route_kind: string | null;
	last_build_skew_revalidation_reason: string | null;
	last_build_skew_requested_href: string | null;
	last_build_skew_status: number | null;
	last_build_skew_ok: boolean | null;
	last_build_skew_current_route_href: string | null;
	last_build_skew_current_work_navigation_href: string | null;
	last_build_skew_current_work_revalidation_status: string | null;
	last_build_skew_current_work_prefetch_href: string | null;
	last_build_skew_current_work_submission_count: number | null;
	last_api_build_skew_server_id: string | null;
	last_api_build_skew_active_client_id: string | null;
	last_api_build_skew_api_route_kind: string | null;
	last_api_build_skew_status: number | null;
	last_api_build_skew_ok: boolean | null;
	history_back_count: number;
	history_forward_count: number;
	duplicate_css_href: string | null;
	public_css_url_probe_background_image: string;
};

type ClickTarget = {
	name: string;
	content: string;
	x: number;
	y: number;
};

const fixture_state = extract((state): FixtureState => {
	const data_scripts = state.document.querySelectorAll("#vorma-data-json");
	const data_script = data_scripts.item(0);
	let data_script_parses = false;
	let data_payload: Record<string, unknown> | null = null;
	if (data_script?.textContent) {
		try {
			data_payload = JSON.parse(data_script.textContent);
			data_script_parses = true;
		} catch {
			data_script_parses = false;
		}
	}

	const probe = state.window.__vorma_bombadil ?? null;
	const css_hrefs = Array.from(
		state.document.querySelectorAll(
			"link[data-vorma-css-bundle], link[data-vorma-css-preload]",
		),
	).map((el) => {
		return (el as HTMLLinkElement).href;
	});
	const duplicate_css_href =
		css_hrefs.find((href, idx) => {
			return css_hrefs.indexOf(href) !== idx;
		}) ?? null;
	const public_css_url_probe = state.document.querySelector(
		"[data-bmb-public-url-probe]",
	);
	const public_css_url_probe_background_image =
		public_css_url_probe === null
			? ""
			: state.window.getComputedStyle(public_css_url_probe)
					.backgroundImage;
	const url = new URL(state.window.location.href);
	const raw_n = url.searchParams.get("n");
	const search_n = raw_n === null ? null : Number(raw_n);
	const raw_delay_ms = url.searchParams.get("delay_ms");
	const search_delay_ms = raw_delay_ms === null ? null : Number(raw_delay_ms);

	return {
		is_dev: data_payload?.IsDev === true,
		title: state.document.title,
		root_count: state.document.querySelectorAll("#vorma-root").length,
		data_script_count: data_scripts.length,
		data_script_parses: data_script_parses,
		shell_count: state.document.querySelectorAll("[data-bmb-shell]").length,
		variant:
			state.document
				.querySelector("[data-bmb-shell]")
				?.getAttribute("data-bmb-shell") ?? null,
		route_deployment:
			state.document.querySelector("[data-bmb-route-deployment]")
				?.textContent ?? "",
		client_build_tag:
			state.document.querySelector("[data-bmb-client-build-tag]")
				?.textContent ?? "",
		switch_result:
			state.document.querySelector("[data-bmb-switch-result]")
				?.textContent ?? "",
		expected_deployment:
			state.document.querySelector("[data-bmb-expected-deployment]")
				?.textContent ?? "",
		expected_operation:
			state.document.querySelector("[data-bmb-expected-operation]")
				?.textContent ?? "",
		pathname: url.pathname,
		search_n: Number.isFinite(search_n) ? search_n : null,
		search_delay_ms: Number.isFinite(search_delay_ms)
			? search_delay_ms
			: null,
		current_href_text:
			state.document.querySelector("[data-bmb-current-href]")
				?.textContent ?? "",
		location_href: state.window.location.href,
		probe_href: probe?.route?.href ?? null,
		route_client_build_id: probe?.route?.client_build_id ?? null,
		probe_ready:
			probe !== null && probe.route !== null && probe.work !== null,
		rendered_route:
			state.document
				.querySelector("[data-bmb-route]")
				?.getAttribute("data-bmb-route") ?? null,
		counter_value_text:
			state.document.querySelector("[data-bmb-counter-value]")
				?.textContent ?? "",
		count_action_next_text:
			state.document.querySelector("[data-bmb-count-action-next]")
				?.textContent ?? "",
		slow_delay_text:
			state.document.querySelector("[data-bmb-slow-delay]")
				?.textContent ?? "",
		item_id:
			state.document
				.querySelector("[data-bmb-item-id]")
				?.getAttribute("data-bmb-item-id") ?? null,
		client_id:
			state.document
				.querySelector("[data-bmb-client-id]")
				?.getAttribute("data-bmb-client-id") ?? null,
		client_server_id_text:
			state.document.querySelector("[data-bmb-client-server-id]")
				?.textContent ?? "",
		client_server_stamp_text:
			state.document.querySelector("[data-bmb-client-server-stamp]")
				?.textContent ?? "",
		client_loader_id_text:
			state.document.querySelector("[data-bmb-client-loader-id]")
				?.textContent ?? "",
		client_loader_trigger_text:
			state.document.querySelector("[data-bmb-client-loader-trigger]")
				?.textContent ?? "",
		client_loader_stamp_text:
			state.document.querySelector("[data-bmb-client-loader-stamp]")
				?.textContent ?? "",
		echo_operation:
			state.document.querySelector("[data-bmb-echo-operation]")
				?.textContent ?? "",
		nested_layout_count: state.document.querySelectorAll(
			"[data-bmb-nested-layout]",
		).length,
		nested_section:
			state.document
				.querySelector("[data-bmb-nested-section]")
				?.getAttribute("data-bmb-nested-section") ?? null,
		nested_detail_id:
			state.document
				.querySelector("[data-bmb-nested-detail-id]")
				?.getAttribute("data-bmb-nested-detail-id") ?? null,
		nested_detail_section_text:
			state.document.querySelector("[data-bmb-nested-detail-section]")
				?.textContent ?? "",
		echo_output:
			state.document.querySelector("[data-bmb-echo-output]")
				?.textContent ?? "",
		error_text:
			state.document.querySelector("[data-bmb-error]")?.textContent ?? "",
		pending_href: probe?.work?.navigation_href ?? "",
		revalidation_status: probe?.work?.revalidation_status ?? "",
		prefetch_href: probe?.work?.prefetch_href ?? "",
		submission_count: probe?.work?.submission_count ?? 0,
		last_navigation_timing_href: probe?.navigation_timing.href ?? null,
		last_navigation_pending_to_route_ms:
			probe?.navigation_timing.pending_to_route_ms ?? null,
		last_navigation_pending_to_dom_ms:
			probe?.navigation_timing.pending_to_dom_ms ?? null,
		last_navigation_route_to_dom_ms:
			probe?.navigation_timing.route_to_dom_ms ?? null,
		last_navigation_pending_to_clear_ms:
			probe?.navigation_timing.pending_to_clear_ms ?? null,
		last_navigation_route_to_clear_ms:
			probe?.navigation_timing.route_to_clear_ms ?? null,
		build_skew_detections: probe?.build_skew_detections ?? 0,
		api_build_skew_detections: probe?.api_build_skew_detections ?? 0,
		query_build_skew_detections: probe?.query_build_skew_detections ?? 0,
		mutation_build_skew_detections:
			probe?.mutation_build_skew_detections ?? 0,
		failed_api_build_skew_detections:
			probe?.failed_api_build_skew_detections ?? 0,
		manual_revalidation_build_skew_detections:
			probe?.manual_revalidation_build_skew_detections ?? 0,
		last_build_skew_server_id: probe?.last_build_skew_server_id ?? null,
		last_build_skew_active_client_id:
			probe?.last_build_skew?.active_client_build_id ?? null,
		last_build_skew_response_kind:
			probe?.last_build_skew?.response_kind ?? null,
		last_build_skew_response_trigger:
			probe?.last_build_skew?.response_trigger ?? null,
		last_build_skew_api_route_kind:
			probe?.last_build_skew?.api_route_kind ?? null,
		last_build_skew_revalidation_reason:
			probe?.last_build_skew?.revalidation_reason ?? null,
		last_build_skew_requested_href:
			probe?.last_build_skew?.requested_href ?? null,
		last_build_skew_status: probe?.last_build_skew?.status ?? null,
		last_build_skew_ok: probe?.last_build_skew?.ok ?? null,
		last_build_skew_current_route_href:
			probe?.last_build_skew?.current_route_href ?? null,
		last_build_skew_current_work_navigation_href:
			probe?.last_build_skew?.current_work_navigation_href ?? null,
		last_build_skew_current_work_revalidation_status:
			probe?.last_build_skew?.current_work_revalidation_status ?? null,
		last_build_skew_current_work_prefetch_href:
			probe?.last_build_skew?.current_work_prefetch_href ?? null,
		last_build_skew_current_work_submission_count:
			probe?.last_build_skew?.current_work_submission_count ?? null,
		last_api_build_skew_server_id:
			probe?.last_api_build_skew?.server_build_id ?? null,
		last_api_build_skew_active_client_id:
			probe?.last_api_build_skew?.active_client_build_id ?? null,
		last_api_build_skew_api_route_kind:
			probe?.last_api_build_skew?.api_route_kind ?? null,
		last_api_build_skew_status: probe?.last_api_build_skew?.status ?? null,
		last_api_build_skew_ok: probe?.last_api_build_skew?.ok ?? null,
		history_back_count: state.navigationHistory.back.length,
		history_forward_count: state.navigationHistory.forward.length,
		duplicate_css_href: duplicate_css_href,
		public_css_url_probe_background_image:
			public_css_url_probe_background_image,
	};
});

function is_settled_route() {
	return (
		fixture_state.current.probe_href !== null &&
		fixture_state.current.probe_href ===
			fixture_state.current.location_href &&
		fixture_state.current.current_href_text ===
			fixture_state.current.location_href
	);
}

function clamp(value: number, min: number, max: number) {
	return Math.min(Math.max(value, min), max);
}

function api_skew_kept_client_build_id(api_route_kind: string, ok: boolean) {
	return (
		fixture_state.current.route_client_build_id !== null &&
		fixture_state.current.last_api_build_skew_server_id !== null &&
		fixture_state.current.last_api_build_skew_active_client_id !== null &&
		fixture_state.current.last_api_build_skew_api_route_kind ===
			api_route_kind &&
		fixture_state.current.last_api_build_skew_ok === ok &&
		fixture_state.current.route_client_build_id ===
			fixture_state.current.last_api_build_skew_active_client_id &&
		fixture_state.current.route_client_build_id !==
			fixture_state.current.last_api_build_skew_server_id &&
		fixture_state.current.client_build_tag !==
			`client-${fixture_state.current.expected_deployment}`
	);
}

const click_targets = extract((state): ClickTarget[] => {
	return Array.from(
		state.document.querySelectorAll<HTMLElement>(
			"[data-bmb-action], [data-bmb-link]",
		),
	).flatMap((el) => {
		const rect = el.getBoundingClientRect();
		if (rect.width <= 0 || rect.height <= 0) {
			return [];
		}
		return [
			{
				name:
					el.getAttribute("data-bmb-action") ??
					el.getAttribute("data-bmb-link") ??
					el.tagName.toLowerCase(),
				content: el.textContent?.trim() ?? "",
				x: rect.left + rect.width / 2,
				y: rect.top + rect.height / 2,
			},
		];
	});
});

export const vorma_fixture_is_mounted = always(() => {
	if (fixture_state.current.is_dev && !fixture_state.current.probe_ready) {
		return true;
	}
	return (
		fixture_state.current.root_count === 1 &&
		fixture_state.current.data_script_count === 1 &&
		fixture_state.current.data_script_parses &&
		fixture_state.current.shell_count === 1 &&
		fixture_state.current.probe_ready
	);
});

export const vorma_dev_fixture_eventually_mounts = always(
	now(() => {
		return (
			fixture_state.current.is_dev &&
			fixture_state.current.root_count === 1 &&
			fixture_state.current.data_script_count === 1 &&
			fixture_state.current.data_script_parses &&
			!fixture_state.current.probe_ready
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.shell_count === 1 &&
				fixture_state.current.probe_ready
			);
		}).within(30, "seconds"),
	),
);

export const vorma_has_title = always(() => {
	return fixture_state.current.title.trim().length > 0;
});

export const vorma_probe_matches_browser_location = always(() => {
	if (!fixture_state.current.probe_ready) {
		return true;
	}
	const probe_href = fixture_state.current.probe_href;
	if (probe_href === null) {
		return false;
	}
	return probe_href === fixture_state.current.location_href;
});

export const vorma_rendered_route_matches_probe = always(() => {
	if (!fixture_state.current.probe_ready) {
		return true;
	}
	const probe_href = fixture_state.current.probe_href;
	if (probe_href === null) {
		return false;
	}
	return fixture_state.current.current_href_text === probe_href;
});

export const vorma_rendered_route_never_matches_pending_navigation = always(
	() => {
		if (
			!fixture_state.current.probe_ready ||
			fixture_state.current.pending_href === ""
		) {
			return true;
		}
		return (
			fixture_state.current.current_href_text !==
			fixture_state.current.pending_href
		);
	},
);

export const vorma_navigation_timing_is_ordered = always(() => {
	const pending_to_route =
		fixture_state.current.last_navigation_pending_to_route_ms;
	const pending_to_clear =
		fixture_state.current.last_navigation_pending_to_clear_ms;
	const pending_to_dom =
		fixture_state.current.last_navigation_pending_to_dom_ms;
	const route_to_dom = fixture_state.current.last_navigation_route_to_dom_ms;
	const route_to_clear =
		fixture_state.current.last_navigation_route_to_clear_ms;
	return (
		(pending_to_route === null || pending_to_route >= 0) &&
		(pending_to_dom === null || pending_to_dom >= 0) &&
		(pending_to_clear === null || pending_to_clear >= 0) &&
		(route_to_dom === null || route_to_dom >= 0) &&
		(route_to_clear === null || route_to_clear >= 0)
	);
});

export const vorma_has_no_duplicate_css_links = always(() => {
	return fixture_state.current.duplicate_css_href === null;
});

export const vorma_resolves_public_urls_in_imported_css = always(() => {
	if (!fixture_state.current.probe_ready) {
		return true;
	}
	const background_image =
		fixture_state.current.public_css_url_probe_background_image;
	return (
		background_image.includes("vorma_out_assets_public-url-probe_") &&
		!background_image.includes("@public")
	);
});

export const vorma_deployment_marker_is_known = always(() => {
	if (!fixture_state.current.probe_ready) {
		return true;
	}
	return (
		fixture_state.current.route_deployment === "from-A" ||
		fixture_state.current.route_deployment === "from-B"
	);
});

export const vorma_client_build_tag_is_known = always(() => {
	if (!fixture_state.current.probe_ready) {
		return true;
	}
	return (
		fixture_state.current.client_build_tag === "client-A" ||
		fixture_state.current.client_build_tag === "client-B"
	);
});

export const vorma_switch_result_is_known = always(() => {
	return (
		fixture_state.current.switch_result === "" ||
		fixture_state.current.switch_result === "A" ||
		fixture_state.current.switch_result === "B" ||
		fixture_state.current.switch_result === "revalidated:ok" ||
		fixture_state.current.switch_result === "revalidated:build_skew" ||
		fixture_state.current.switch_result ===
			"revalidated:max_retries_exhausted" ||
		fixture_state.current.switch_result === "query:ok" ||
		fixture_state.current.switch_result === "query:error" ||
		fixture_state.current.switch_result === "mutation:ok" ||
		fixture_state.current.switch_result === "mutation:error" ||
		fixture_state.current.switch_result === "mutation-error:ok" ||
		fixture_state.current.switch_result === "mutation-error:error"
	);
});

export const vorma_build_skew_events_include_context = always(() => {
	if (fixture_state.current.build_skew_detections === 0) {
		return true;
	}
	return (
		fixture_state.current.last_build_skew_server_id !== null &&
		fixture_state.current.last_build_skew_active_client_id !== null &&
		fixture_state.current.last_build_skew_response_kind !== null &&
		fixture_state.current.last_build_skew_requested_href !== null &&
		fixture_state.current.last_build_skew_status !== null &&
		fixture_state.current.last_build_skew_ok !== null &&
		fixture_state.current.last_build_skew_current_route_href !== null &&
		fixture_state.current.last_build_skew_current_work_submission_count !==
			null
	);
});

export const vorma_switch_revalidation_detects_build_skew = always(
	now(() => {
		return (
			fixture_state.current.expected_operation === "revalidate-pending"
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current
					.manual_revalidation_build_skew_detections > 0 &&
				fixture_state.current.expected_operation ===
					"revalidate-build_skew" &&
				fixture_state.current.switch_result === "revalidated:build_skew"
			);
		}).within(5, "seconds"),
	),
);

export const vorma_switch_navigation_reaches_new_deployment = always(
	now(() => {
		return (
			fixture_state.current.expected_operation === "navigate" &&
			(fixture_state.current.expected_deployment === "A" ||
				fixture_state.current.expected_deployment === "B")
		);
	}).implies(
		eventually(() => {
			return (
				is_settled_route() &&
				fixture_state.current.route_deployment ===
					`from-${fixture_state.current.expected_deployment}` &&
				fixture_state.current.client_build_tag ===
					`client-${fixture_state.current.expected_deployment}`
			);
		}).within(5, "seconds"),
	),
);

export const vorma_switch_query_detects_api_skew_without_adoption = always(
	now(() => {
		return (
			fixture_state.current.expected_operation === "query-pending" &&
			(fixture_state.current.expected_deployment === "A" ||
				fixture_state.current.expected_deployment === "B")
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.expected_operation === "query-ok" &&
				fixture_state.current.query_build_skew_detections > 0 &&
				api_skew_kept_client_build_id("query", true)
			);
		}).within(5, "seconds"),
	),
);

export const vorma_switch_mutation_detects_api_skew_without_adoption = always(
	now(() => {
		return (
			fixture_state.current.expected_operation === "mutation-pending" &&
			(fixture_state.current.expected_deployment === "A" ||
				fixture_state.current.expected_deployment === "B")
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.expected_operation === "mutation-ok" &&
				fixture_state.current.mutation_build_skew_detections > 0 &&
				api_skew_kept_client_build_id("mutation", true)
			);
		}).within(5, "seconds"),
	),
);

export const vorma_switch_failed_mutation_detects_api_skew_without_adoption =
	always(
		now(() => {
			return (
				fixture_state.current.expected_operation ===
					"mutation-error-pending" &&
				(fixture_state.current.expected_deployment === "A" ||
					fixture_state.current.expected_deployment === "B")
			);
		}).implies(
			eventually(() => {
				return (
					fixture_state.current.expected_operation ===
						"mutation-error-error" &&
					fixture_state.current.failed_api_build_skew_detections >
						0 &&
					api_skew_kept_client_build_id("mutation", false)
				);
			}).within(5, "seconds"),
		),
	);

export const vorma_home_route_renders_home = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/") {
		return true;
	}
	return fixture_state.current.rendered_route === "home";
});

export const vorma_counter_route_matches_loader_value = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/counter") {
		return true;
	}
	const n = fixture_state.current.search_n ?? 0;
	const expected = String(clamp(n, -5, 5));
	return (
		fixture_state.current.rendered_route === "counter" &&
		fixture_state.current.counter_value_text === expected
	);
});

export const vorma_count_query_action_returns_clamped_value = always(() => {
	if (
		!is_settled_route() ||
		fixture_state.current.pathname !== "/counter" ||
		fixture_state.current.count_action_next_text === ""
	) {
		return true;
	}
	return (
		fixture_state.current.count_action_next_text === "-5" ||
		fixture_state.current.count_action_next_text === "5"
	);
});

export const vorma_slow_route_matches_loader_delay = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/slow") {
		return true;
	}
	const delay_ms = fixture_state.current.search_delay_ms ?? 0;
	const expected = String(clamp(delay_ms, 0, 250));
	return (
		fixture_state.current.rendered_route === "slow" &&
		fixture_state.current.slow_delay_text === expected
	);
});

export const vorma_item_route_matches_url_param = always(() => {
	if (
		!is_settled_route() ||
		!fixture_state.current.pathname.startsWith("/items/")
	) {
		return true;
	}
	const expected = fixture_state.current.pathname.slice("/items/".length);
	return (
		fixture_state.current.rendered_route === "item" &&
		fixture_state.current.item_id === expected
	);
});

export const vorma_client_route_matches_url_param = always(() => {
	if (
		!is_settled_route() ||
		!fixture_state.current.pathname.startsWith("/client/")
	) {
		return true;
	}
	const expected = fixture_state.current.pathname.slice("/client/".length);
	return (
		fixture_state.current.rendered_route === "client" &&
		fixture_state.current.client_id === expected &&
		fixture_state.current.client_server_id_text === expected &&
		fixture_state.current.client_loader_id_text === expected
	);
});

export const vorma_client_route_has_client_loader_data = always(() => {
	if (
		!is_settled_route() ||
		!fixture_state.current.pathname.startsWith("/client/")
	) {
		return true;
	}
	return (
		fixture_state.current.client_server_stamp_text.trim().length > 0 &&
		fixture_state.current.client_loader_trigger_text.trim().length > 0 &&
		fixture_state.current.client_loader_stamp_text.trim().length > 0
	);
});

export const vorma_nested_index_renders_layout = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/nested") {
		return true;
	}
	return (
		fixture_state.current.rendered_route === "nested-index" &&
		fixture_state.current.nested_layout_count === 1 &&
		fixture_state.current.nested_section === "nested"
	);
});

export const vorma_nested_detail_renders_parent_and_leaf = always(() => {
	if (
		!is_settled_route() ||
		!fixture_state.current.pathname.startsWith("/nested/") ||
		!fixture_state.current.pathname.endsWith("/details")
	) {
		return true;
	}
	const id_start = "/nested/".length;
	const id_end = fixture_state.current.pathname.length - "/details".length;
	const expected = fixture_state.current.pathname.slice(id_start, id_end);
	return (
		fixture_state.current.rendered_route === "nested-detail" &&
		fixture_state.current.nested_layout_count === 1 &&
		fixture_state.current.nested_section === "nested" &&
		fixture_state.current.nested_detail_id === expected &&
		fixture_state.current.nested_detail_section_text === "nested"
	);
});

export const vorma_echo_route_has_output = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/echo") {
		return true;
	}
	return (
		fixture_state.current.rendered_route === "echo" &&
		fixture_state.current.echo_output.trim().length > 0
	);
});

export const vorma_fail_route_uses_error_boundary = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/fail") {
		return true;
	}
	return (
		fixture_state.current.rendered_route === "fail" &&
		fixture_state.current.error_text.includes(
			"Fixture loader failed on purpose.",
		)
	);
});

export const vorma_echo_submit_eventually_updates_output = always(
	now(() => {
		return (
			fixture_state.current.pathname === "/echo" &&
			fixture_state.current.rendered_route === "echo" &&
			fixture_state.current.echo_operation === "submit-pending"
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.echo_operation === "submit-ok" &&
				fixture_state.current.echo_output === "hello"
			);
		}).within(5, "seconds"),
	),
);

export const vorma_echo_failed_submit_eventually_reports_error = always(
	now(() => {
		return (
			fixture_state.current.pathname === "/echo" &&
			fixture_state.current.rendered_route === "echo" &&
			fixture_state.current.echo_operation === "fail-pending"
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.echo_operation === "fail-error" &&
				fixture_state.current.echo_output.trim().length > 0
			);
		}).within(5, "seconds"),
	),
);

export const vorma_work_eventually_settles = always(
	now(() => {
		return (
			fixture_state.current.pending_href !== "" ||
			fixture_state.current.revalidation_status !== "" ||
			fixture_state.current.prefetch_href !== "" ||
			fixture_state.current.submission_count > 0
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.pending_href === "" &&
				fixture_state.current.revalidation_status === "" &&
				fixture_state.current.prefetch_href === "" &&
				fixture_state.current.submission_count === 0
			);
		}).within(5, "seconds"),
	),
);

export const vorma_fixture_actions = actions(() => {
	if (
		fixture_state.current.pending_href !== "" ||
		fixture_state.current.revalidation_status !== "" ||
		fixture_state.current.submission_count > 0 ||
		fixture_state.current.expected_operation.endsWith("-pending")
	) {
		return ["Wait"];
	}

	if (
		fixture_state.current.pathname === "/counter" &&
		fixture_state.current.count_action_next_text === ""
	) {
		const count_action_targets = click_targets.current.filter((target) => {
			return target.name.startsWith("count-action-");
		});
		if (count_action_targets.length > 0) {
			return count_action_targets.map((target) => {
				return {
					Click: {
						name: target.name,
						content: target.content,
						point: { x: target.x, y: target.y },
					},
				};
			});
		}
	}

	const available_click_targets = fixture_state.current.is_dev
		? click_targets.current.filter((target) => {
				return (
					!target.name.startsWith("switch-") &&
					!target.content.startsWith("Switch")
				);
			})
		: click_targets.current;

	return [
		"Wait",
		...(is_settled_route() ? ["Reload" as const] : []),
		...(fixture_state.current.history_back_count > 0
			? ["Back" as const]
			: []),
		...(fixture_state.current.history_forward_count > 0
			? ["Forward" as const]
			: []),
		...available_click_targets.map((target) => {
			return {
				Click: {
					name: target.name,
					content: target.content,
					point: { x: target.x, y: target.y },
				},
			};
		}),
	];
});

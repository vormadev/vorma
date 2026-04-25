import {
	actions,
	always,
	eventually,
	extract,
	now,
} from "@antithesishq/bombadil";

export * from "@antithesishq/bombadil/defaults";

type FixtureState = {
	title: string;
	rootCount: number;
	dataScriptCount: number;
	dataScriptParses: boolean;
	shellCount: number;
	variant: string | null;
	pathname: string;
	searchN: number | null;
	searchDelayMS: number | null;
	currentHrefText: string;
	locationHref: string;
	probeHref: string | null;
	probeReady: boolean;
	renderedRoute: string | null;
	counterValueText: string;
	countActionNextText: string;
	slowDelayText: string;
	itemID: string | null;
	clientID: string | null;
	clientServerIDText: string;
	clientServerStampText: string;
	clientLoaderIDText: string;
	clientLoaderTriggerText: string;
	clientLoaderStampText: string;
	nestedLayoutCount: number;
	nestedSection: string | null;
	nestedDetailID: string | null;
	nestedDetailSectionText: string;
	echoOutput: string;
	errorText: string;
	pendingHref: string;
	revalidationStatus: string;
	prefetchHref: string;
	submissionCount: number;
	historyBackCount: number;
	historyForwardCount: number;
	duplicateCSSHref: string | null;
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
	if (data_script?.textContent) {
		try {
			JSON.parse(data_script.textContent);
			data_script_parses = true;
		} catch {
			data_script_parses = false;
		}
	}

	const probe = state.window.__vormaBombadil ?? null;
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
	const url = new URL(state.window.location.href);
	const raw_n = url.searchParams.get("n");
	const search_n = raw_n === null ? null : Number(raw_n);
	const raw_delay_ms = url.searchParams.get("delay_ms");
	const search_delay_ms = raw_delay_ms === null ? null : Number(raw_delay_ms);

	return {
		title: state.document.title,
		rootCount: state.document.querySelectorAll("#vorma-root").length,
		dataScriptCount: data_scripts.length,
		dataScriptParses: data_script_parses,
		shellCount: state.document.querySelectorAll("[data-bmb-shell]").length,
		variant:
			state.document
				.querySelector("[data-bmb-shell]")
				?.getAttribute("data-bmb-shell") ?? null,
		pathname: url.pathname,
		searchN: Number.isFinite(search_n) ? search_n : null,
		searchDelayMS: Number.isFinite(search_delay_ms)
			? search_delay_ms
			: null,
		currentHrefText:
			state.document.querySelector("[data-bmb-current-href]")
				?.textContent ?? "",
		locationHref: state.window.location.href,
		probeHref: probe?.route?.href ?? null,
		probeReady: probe?.route !== null && probe?.work !== null,
		renderedRoute:
			state.document
				.querySelector("[data-bmb-route]")
				?.getAttribute("data-bmb-route") ?? null,
		counterValueText:
			state.document.querySelector("[data-bmb-counter-value]")
				?.textContent ?? "",
		countActionNextText:
			state.document.querySelector("[data-bmb-count-action-next]")
				?.textContent ?? "",
		slowDelayText:
			state.document.querySelector("[data-bmb-slow-delay]")
				?.textContent ?? "",
		itemID:
			state.document
				.querySelector("[data-bmb-item-id]")
				?.getAttribute("data-bmb-item-id") ?? null,
		clientID:
			state.document
				.querySelector("[data-bmb-client-id]")
				?.getAttribute("data-bmb-client-id") ?? null,
		clientServerIDText:
			state.document.querySelector("[data-bmb-client-server-id]")
				?.textContent ?? "",
		clientServerStampText:
			state.document.querySelector("[data-bmb-client-server-stamp]")
				?.textContent ?? "",
		clientLoaderIDText:
			state.document.querySelector("[data-bmb-client-loader-id]")
				?.textContent ?? "",
		clientLoaderTriggerText:
			state.document.querySelector("[data-bmb-client-loader-trigger]")
				?.textContent ?? "",
		clientLoaderStampText:
			state.document.querySelector("[data-bmb-client-loader-stamp]")
				?.textContent ?? "",
		nestedLayoutCount: state.document.querySelectorAll(
			"[data-bmb-nested-layout]",
		).length,
		nestedSection:
			state.document
				.querySelector("[data-bmb-nested-section]")
				?.getAttribute("data-bmb-nested-section") ?? null,
		nestedDetailID:
			state.document
				.querySelector("[data-bmb-nested-detail-id]")
				?.getAttribute("data-bmb-nested-detail-id") ?? null,
		nestedDetailSectionText:
			state.document.querySelector("[data-bmb-nested-detail-section]")
				?.textContent ?? "",
		echoOutput:
			state.document.querySelector("[data-bmb-echo-output]")
				?.textContent ?? "",
		errorText:
			state.document.querySelector("[data-bmb-error]")?.textContent ?? "",
		pendingHref: probe?.work?.navigationHref ?? "",
		revalidationStatus: probe?.work?.revalidationStatus ?? "",
		prefetchHref: probe?.work?.prefetchHref ?? "",
		submissionCount: probe?.work?.submissionCount ?? 0,
		historyBackCount: state.navigationHistory.back.length,
		historyForwardCount: state.navigationHistory.forward.length,
		duplicateCSSHref: duplicate_css_href,
	};
});

function is_settled_route() {
	return (
		fixture_state.current.probeHref !== null &&
		fixture_state.current.probeHref ===
			fixture_state.current.locationHref &&
		fixture_state.current.currentHrefText ===
			fixture_state.current.locationHref
	);
}

function clamp(value: number, min: number, max: number) {
	return Math.min(Math.max(value, min), max);
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
	return (
		fixture_state.current.rootCount === 1 &&
		fixture_state.current.dataScriptCount === 1 &&
		fixture_state.current.dataScriptParses &&
		fixture_state.current.shellCount === 1 &&
		fixture_state.current.probeReady
	);
});

export const vorma_has_title = always(() => {
	return fixture_state.current.title.trim().length > 0;
});

export const vorma_probe_matches_browser_location = always(() => {
	const probe_href = fixture_state.current.probeHref;
	if (probe_href === null) {
		return false;
	}
	return probe_href === fixture_state.current.locationHref;
});

export const vorma_rendered_route_matches_probe = always(() => {
	const probe_href = fixture_state.current.probeHref;
	if (probe_href === null) {
		return false;
	}
	return fixture_state.current.currentHrefText === probe_href;
});

export const vorma_has_no_duplicate_css_links = always(() => {
	return fixture_state.current.duplicateCSSHref === null;
});

export const vorma_home_route_renders_home = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/") {
		return true;
	}
	return fixture_state.current.renderedRoute === "home";
});

export const vorma_counter_route_matches_loader_value = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/counter") {
		return true;
	}
	const n = fixture_state.current.searchN ?? 0;
	const expected = String(clamp(n, -5, 5));
	return (
		fixture_state.current.renderedRoute === "counter" &&
		fixture_state.current.counterValueText === expected
	);
});

export const vorma_count_query_action_returns_clamped_value = always(() => {
	if (
		!is_settled_route() ||
		fixture_state.current.pathname !== "/counter" ||
		fixture_state.current.countActionNextText === ""
	) {
		return true;
	}
	return (
		fixture_state.current.countActionNextText === "-5" ||
		fixture_state.current.countActionNextText === "5"
	);
});

export const vorma_slow_route_matches_loader_delay = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/slow") {
		return true;
	}
	const delay_ms = fixture_state.current.searchDelayMS ?? 0;
	const expected = String(clamp(delay_ms, 0, 250));
	return (
		fixture_state.current.renderedRoute === "slow" &&
		fixture_state.current.slowDelayText === expected
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
		fixture_state.current.renderedRoute === "item" &&
		fixture_state.current.itemID === expected
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
		fixture_state.current.renderedRoute === "client" &&
		fixture_state.current.clientID === expected &&
		fixture_state.current.clientServerIDText === expected &&
		fixture_state.current.clientLoaderIDText === expected
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
		fixture_state.current.clientServerStampText.trim().length > 0 &&
		fixture_state.current.clientLoaderTriggerText.trim().length > 0 &&
		fixture_state.current.clientLoaderStampText.trim().length > 0
	);
});

export const vorma_nested_index_renders_layout = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/nested") {
		return true;
	}
	return (
		fixture_state.current.renderedRoute === "nested-index" &&
		fixture_state.current.nestedLayoutCount === 1 &&
		fixture_state.current.nestedSection === "nested"
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
		fixture_state.current.renderedRoute === "nested-detail" &&
		fixture_state.current.nestedLayoutCount === 1 &&
		fixture_state.current.nestedSection === "nested" &&
		fixture_state.current.nestedDetailID === expected &&
		fixture_state.current.nestedDetailSectionText === "nested"
	);
});

export const vorma_echo_route_has_output = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/echo") {
		return true;
	}
	return (
		fixture_state.current.renderedRoute === "echo" &&
		fixture_state.current.echoOutput.trim().length > 0
	);
});

export const vorma_fail_route_uses_error_boundary = always(() => {
	if (!is_settled_route() || fixture_state.current.pathname !== "/fail") {
		return true;
	}
	return (
		fixture_state.current.renderedRoute === "fail" &&
		fixture_state.current.errorText.includes(
			"Fixture loader failed on purpose.",
		)
	);
});

export const vorma_echo_submit_eventually_updates_output = always(
	now(() => {
		const last_action = fixture_state.current;
		return (
			last_action.pathname === "/echo" &&
			last_action.renderedRoute === "echo" &&
			last_action.submissionCount > 0
		);
	}).implies(
		eventually(() => {
			return fixture_state.current.echoOutput === "hello";
		}).within(5, "seconds"),
	),
);

export const vorma_work_eventually_settles = always(
	now(() => {
		return (
			fixture_state.current.pendingHref !== "" ||
			fixture_state.current.revalidationStatus !== "" ||
			fixture_state.current.prefetchHref !== "" ||
			fixture_state.current.submissionCount > 0
		);
	}).implies(
		eventually(() => {
			return (
				fixture_state.current.pendingHref === "" &&
				fixture_state.current.revalidationStatus === "" &&
				fixture_state.current.prefetchHref === "" &&
				fixture_state.current.submissionCount === 0
			);
		}).within(5, "seconds"),
	),
);

export const vorma_fixture_actions = actions(() => {
	if (
		fixture_state.current.pathname === "/counter" &&
		fixture_state.current.countActionNextText === ""
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

	return [
		"Wait",
		...(is_settled_route() ? ["Reload" as const] : []),
		...(fixture_state.current.historyBackCount > 0
			? ["Back" as const]
			: []),
		...(fixture_state.current.historyForwardCount > 0
			? ["Forward" as const]
			: []),
		...click_targets.current.map((target) => {
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

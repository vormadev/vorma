import { actions, always, extract } from "@antithesishq/bombadil";

type LatencyTarget = {
	href: string;
	key: string;
};

type LatencyState = {
	location_href: string;
	pathname: string;
	search: string;
	current_href_text: string;
	pending_href: string;
	last_navigation_timing_href: string | null;
	last_navigation_pending_to_route_ms: number | null;
	last_navigation_pending_to_dom_ms: number | null;
	last_navigation_route_to_dom_ms: number | null;
};

type ClickTarget = {
	name: string;
	content: string;
	x: number;
	y: number;
};

const latency_targets: LatencyTarget[] = [
	{ href: "/", key: "home" },
	{ href: "/counter?n=0", key: "counter" },
	{ href: "/echo", key: "echo" },
	{ href: "/items/alpha", key: "item-alpha" },
	{ href: "/client/alpha", key: "client-alpha" },
	{ href: "/nested/alpha/details", key: "nested-alpha" },
];

const latency_state = extract((state): LatencyState => {
	const probe = state.window.__vorma_bombadil ?? null;
	return {
		location_href: state.window.location.href,
		pathname: state.window.location.pathname,
		search: state.window.location.search,
		current_href_text:
			state.document.querySelector("[data-bmb-current-href]")?.textContent ?? "",
		pending_href: probe?.work?.navigation_href ?? "",
		last_navigation_timing_href: probe?.navigation_timing.href ?? null,
		last_navigation_pending_to_route_ms:
			probe?.navigation_timing.pending_to_route_ms ?? null,
		last_navigation_pending_to_dom_ms:
			probe?.navigation_timing.pending_to_dom_ms ?? null,
		last_navigation_route_to_dom_ms: probe?.navigation_timing.route_to_dom_ms ?? null,
	};
});

const click_targets = extract((state): ClickTarget[] => {
	return Array.from(
		state.document.querySelectorAll<HTMLElement>("[data-bmb-link]"),
	).flatMap((el) => {
		const rect = el.getBoundingClientRect();
		if (rect.width <= 0 || rect.height <= 0) {
			return [];
		}
		return [
			{
				name: el.getAttribute("data-bmb-link") ?? "",
				content: el.textContent?.trim() ?? "",
				x: rect.left + rect.width / 2,
				y: rect.top + rect.height / 2,
			},
		];
	});
});

function current_target_idx(): number {
	return latency_targets.findIndex((target) => {
		const [pathname = "", search = ""] = target.href.split("?");
		return (
			latency_state.current.pathname === pathname &&
			latency_state.current.search === (search === "" ? "" : `?${search}`)
		);
	});
}

function next_target(): LatencyTarget {
	const idx = current_target_idx();
	if (idx < 0) {
		return latency_targets[0]!;
	}
	return latency_targets[(idx + 1) % latency_targets.length]!;
}

export const vorma_latency_probe_is_ordered = always(() => {
	const state = latency_state.current;
	return (
		(state.last_navigation_pending_to_route_ms === null ||
			state.last_navigation_pending_to_route_ms >= 0) &&
		(state.last_navigation_pending_to_dom_ms === null ||
			state.last_navigation_pending_to_dom_ms >= 0) &&
		(state.last_navigation_route_to_dom_ms === null ||
			state.last_navigation_route_to_dom_ms >= 0)
	);
});

export const vorma_latency_fixture_actions = actions(() => {
	const state = latency_state.current;
	if (state.pending_href !== "" || state.location_href !== state.current_href_text) {
		return ["Wait"];
	}

	const next_latency_target = next_target();
	const target = click_targets.current.find((candidate) => {
		return candidate.name === next_latency_target.key;
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
});

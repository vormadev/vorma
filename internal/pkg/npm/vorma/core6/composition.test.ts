import { describe, expect, it } from "vitest";
import { CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE } from "../core/constants.ts";
import {
	create_core6_client_composition,
	type Core6ClientComposition,
	type Core6ClientCompositionHost,
} from "./composition.ts";
import { core6_route_boot_result_kind } from "./route_boot.ts";
import {
	core6_route_navigation_owner_result_kind,
	type Core6RouteNavigationOwnerResult,
} from "./route_navigation_owner.ts";
import {
	core6_route_payload_field,
	type Core6RawRoutePayload,
	type Core6RoutePreparationParseInputArgs,
} from "./route_preparation.ts";
import type { Core6RouteCommit, Core6RouteState } from "./route_publication.ts";

const current_href = "https://example.test/current";
const current_key = "current-key";

function make_raw_payload(href: string): Core6RawRoutePayload {
	const target = new URL(href);
	const pattern = target.pathname;
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: [`${pattern}.js`],
		[core6_route_payload_field.loaders_data]: [
			{
				pathname: target.pathname,
				search: target.search,
			},
		],
		[core6_route_payload_field.matched_patterns]: [pattern],
		[core6_route_payload_field.params]: {},
		[core6_route_payload_field.search_schemas]: [{ pattern }],
		[core6_route_payload_field.splat_values]: [],
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

function create_fixture(): {
	api_fetches: string[];
	commits: Core6RouteCommit[];
	composition: Core6ClientComposition;
	current_href: () => string;
	route_fetches: string[];
} {
	let current_href_value = current_href;
	const api_fetches: string[] = [];
	const commits: Core6RouteCommit[] = [];
	const route_fetches: string[] = [];
	const host: Core6ClientCompositionHost = {
		clear_timer: () => {},
		current_href: () => {
			return current_href_value;
		},
		commit_publication: (publication) => {
			commits.push(publication.commit);
			const route = publication.commit.route_update?.route;
			if (route) {
				current_href_value = route.href;
			}
		},
		fetch_api_response: async ({ href }) => {
			api_fetches.push(href);
			return new Response(JSON.stringify({ ok: true }), {
				headers: {
					[CONTENT_TYPE_HEADER]: JSON_CONTENT_TYPE,
				},
			});
		},
		fetch_route_payload: async ({ intent }) => {
			return make_raw_payload(intent.href);
		},
		fetch_route_response: async ({ href }) => {
			const target = new URL(href);
			route_fetches.push(`${target.pathname}${target.search}`);
			return new Response(JSON.stringify(make_raw_payload(target.href)));
		},
		hard_redirect: () => {},
		import_module: async () => {
			return {};
		},
		parse_input: (args: Core6RoutePreparationParseInputArgs) => {
			return {
				pattern: args.pattern,
				query: args.search_params.get("q"),
				schema: args.schema,
			};
		},
		reload: () => {},
		route_state_equal: same_route_state,
		save_scroll_for_key: () => {},
		set_timer: () => {
			return Symbol();
		},
	};
	const composition = create_core6_client_composition({
		active_client_build_id: () => {
			return "build-1";
		},
		deployment_id: () => {
			return "deployment-1";
		},
		host,
		initial_position: {
			href: current_href,
			key: current_key,
			state: { from: "history" },
		},
	});
	return {
		api_fetches,
		commits,
		composition,
		current_href: () => {
			return current_href_value;
		},
		route_fetches,
	};
}

async function seed_current_route(
	composition: Core6ClientComposition,
	href = current_href,
): Promise<void> {
	const result = await composition.boot.boot({
		history_state: { from: "boot" },
		href,
		payload: make_raw_payload(href),
	});
	if (result.kind !== core6_route_boot_result_kind.booted) {
		throw new Error(`expected seed route to publish: ${result.kind}`);
	}
}

describe("core6 client composition", () => {
	it("shares runtime state between the shell and direct navigation owner", async () => {
		const fixture = create_fixture();
		await seed_current_route(fixture.composition);

		const result = await fixture.composition.navigation.navigate({
			href: "#section",
			state: { from: "navigation" },
		});

		expect(result).toMatchObject({
			did_navigate: true,
			history: {
				href: `${current_href}#section`,
				replace: false,
				state: { from: "navigation" },
			},
			kind: core6_route_navigation_owner_result_kind.same_document,
		});
		expect(fixture.composition.runtime.current_route()).toMatchObject({
			historyState: { from: "navigation" },
			href: `${current_href}#section`,
		});
		expect(fixture.current_href()).toBe(`${current_href}#section`);
		expect(fixture.commits).toHaveLength(2);
	});

	it("wires prepared prefetch promotion into direct navigation", async () => {
		const fixture = create_fixture();
		await seed_current_route(fixture.composition);
		const prefetch = fixture.composition.prefetch.start({
			href: "/prefetched?q=1",
		});
		if (!prefetch) {
			throw new Error("expected prefetch to start");
		}
		await expect(prefetch).resolves.toMatchObject({
			prepared: {
				href: "https://example.test/prefetched?q=1",
			},
		});
		expect(fixture.route_fetches).toHaveLength(1);

		const result = await fixture.composition.navigation.navigate({
			href: "/prefetched?q=1",
			state: { from: "promoted-navigation" },
		});

		expect(result).toMatchObject({
			did_navigate: true,
			history: {
				href: "https://example.test/prefetched?q=1",
				state: { from: "promoted-navigation" },
			},
			kind: core6_route_navigation_owner_result_kind.routed,
		});
		expect(fixture.route_fetches).toHaveLength(1);
		expect(fixture.composition.runtime.current_route()).toMatchObject({
			historyState: { from: "promoted-navigation" },
			href: "https://example.test/prefetched?q=1",
		});
	});

	it("wires API submit mutation revalidation into the shared route runtime", async () => {
		const fixture = create_fixture();
		await seed_current_route(fixture.composition);

		const result = await fixture.composition.api_submit.submit<{
			ok: boolean;
		}>({
			request_init: {
				body: JSON.stringify({ name: "Ada" }),
				method: "POST",
			},
			url: "/api/update",
		});

		expect(result).toMatchObject({
			data: { ok: true },
			success: true,
		});
		if (!result.success) {
			throw new Error("expected API submit to succeed");
		}
		await expect(result.revalidation_promise).resolves.toEqual({
			ok: true,
		});
		expect(fixture.api_fetches).toEqual([
			"https://example.test/api/update",
		]);
		expect(fixture.route_fetches).toHaveLength(1);
		expect(fixture.composition.runtime.current_route()).toMatchObject({
			href: current_href,
		});
	});

	it("keeps owner status surfaces independently observable", async () => {
		const fixture = create_fixture();
		await seed_current_route(fixture.composition);
		const result: Core6RouteNavigationOwnerResult =
			await fixture.composition.navigation.navigate({
				href: "mailto:ada@example.test",
			});

		expect(result).toMatchObject({
			did_navigate: false,
			history: null,
			kind: core6_route_navigation_owner_result_kind.hard_redirect,
		});
		expect(fixture.composition.navigation.current_status()).toBeNull();
		expect(fixture.composition.popstate.current_status()).toBeNull();
		expect(fixture.composition.prefetch.current_status()).toBeNull();
		expect(fixture.composition.revalidation.current_status()).toBeNull();
		expect(fixture.composition.api_submit.current_statuses()).toEqual([]);
	});
});

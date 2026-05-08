import { Data, Effect } from "effect";
import { jsonDeepEquals } from "vorma/kit/json";
import {
	createPatternRegistry,
	findNestedMatches,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher";
import type {
	RouteRenderEntry,
	RouteRenderState,
} from "../create_client_core.ts";
import type { LinkRouteState, LinkWorkState } from "../make_link_props.ts";
import type { LinkPropsBase, RouteErrorState } from "../types.ts";
import { get_entry_key } from "./outlet_slot_runtime.ts";

type LinkAttributeCandidate = {
	url: URL;
	matched_patterns: string[];
};

export type DecomposedState = {
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;
	loaders_data: unknown[];
	client_loaders_data: unknown[];
	matched_patterns: string[];
	import_urls: string[];
	entry_keys: string[];
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export class AdapterRuntimeInitFailed extends Data.TaggedError(
	"AdapterRuntimeInitFailed",
)<{
	readonly reason: string;
}> {}

export type AdapterRuntime = {
	register_link_pattern: (pattern: string) => void;
	decompose_route_render_state: (
		route_state: RouteRenderState,
	) => DecomposedState;
	get_link_attribute_state: (
		href: string,
		match_rules: LinkPropsBase["attributeMatchRules"],
		route_state: LinkRouteState | null,
		work_state: LinkWorkState,
	) => {
		active_exact: boolean;
		active_ancestor: boolean;
		pending_exact: boolean;
		pending_ancestor: boolean;
	};
};

function href_to_link_candidate(
	attribute_registry: PatternRegistry,
	href: string,
): LinkAttributeCandidate {
	const url = new URL(href, window.location.href);
	const match = findNestedMatches(attribute_registry, url.pathname);
	return {
		url,
		matched_patterns:
			match?.matches.map((m) => {
				return m.registeredPattern.originalPattern;
			}) ?? [],
	};
}

function exact_link_match(
	target: LinkAttributeCandidate,
	candidate: LinkAttributeCandidate,
	match_rules: LinkPropsBase["attributeMatchRules"],
): boolean {
	return (
		target.url.pathname === candidate.url.pathname &&
		(target.matched_patterns.length === 0 ||
			candidate.matched_patterns.length === 0 ||
			jsonDeepEquals(
				target.matched_patterns,
				candidate.matched_patterns,
			)) &&
		(match_rules?.includeSearch !== true ||
			target.url.search === candidate.url.search) &&
		(match_rules?.includeHash !== true ||
			target.url.hash === candidate.url.hash)
	);
}

function ancestor_link_match(
	target: LinkAttributeCandidate,
	candidate: LinkAttributeCandidate,
): boolean {
	const target_path =
		target.url.pathname === "/" ? "/" : `${target.url.pathname}/`;
	return (
		target.matched_patterns.length > 0 &&
		candidate.matched_patterns.length > target.matched_patterns.length &&
		(target_path === "/" ||
			candidate.url.pathname.startsWith(target_path)) &&
		target.matched_patterns.every((pattern, i) => {
			return candidate.matched_patterns[i] === pattern;
		})
	);
}

function stable<T>(prev: T, next: T): T {
	return jsonDeepEquals(prev, next) ? prev : next;
}

export function make_adapter_runtime(): Effect.Effect<
	AdapterRuntime,
	AdapterRuntimeInitFailed
> {
	return Effect.gen(function* () {
		const attribute_registry_res = createPatternRegistry({
			dynamicParamPrefixRune: ":",
			splatSegmentRune: "*",
			explicitIndexSegment: "_index",
		});
		if (!attribute_registry_res.ok) {
			return yield* Effect.fail(
				new AdapterRuntimeInitFailed({
					reason: attribute_registry_res.err,
				}),
			);
		}
		const attribute_registry = attribute_registry_res.val;
		let prev: DecomposedState = {
			entries: [],
			error: null,
			loaders_data: [],
			client_loaders_data: [],
			matched_patterns: [],
			import_urls: [],
			entry_keys: [],
			params: {},
			splat_values: [],
			client_build_id: "",
			history_state: undefined,
		};

		return {
			register_link_pattern: (pattern) => {
				registerPattern(attribute_registry, pattern);
			},
			decompose_route_render_state: (route_state) => {
				for (const entry of route_state.entries) {
					registerPattern(attribute_registry, entry.pattern);
				}

				const next: DecomposedState = {
					entries: route_state.entries,
					error: route_state.error,
					loaders_data: stable(
						prev.loaders_data,
						route_state.entries.map((e) => {
							return e.loader_data;
						}),
					),
					client_loaders_data: stable(
						prev.client_loaders_data,
						route_state.entries.map((e) => {
							return e.client_loader_data;
						}),
					),
					matched_patterns: stable(
						prev.matched_patterns,
						route_state.entries.map((e) => {
							return e.pattern;
						}),
					),
					import_urls: stable(
						prev.import_urls,
						route_state.entries.map((e) => {
							return e.module_url;
						}),
					),
					entry_keys: stable(
						prev.entry_keys,
						route_state.entries.map((e) => {
							return get_entry_key(e);
						}),
					),
					params: stable(prev.params, route_state.params),
					splat_values: stable(
						prev.splat_values,
						route_state.splat_values,
					),
					client_build_id: stable(
						prev.client_build_id,
						route_state.client_build_id,
					),
					history_state: stable(
						prev.history_state,
						route_state.history_state,
					),
				};

				prev = next;
				return next;
			},
			get_link_attribute_state: (
				href,
				match_rules,
				route_state,
				work_state,
			) => {
				if (match_rules?.skip === true || !route_state) {
					return {
						active_exact: false,
						active_ancestor: false,
						pending_exact: false,
						pending_ancestor: false,
					};
				}

				const target = href_to_link_candidate(attribute_registry, href);
				const route_url = new URL(
					route_state.href,
					window.location.href,
				);
				const route = {
					url: route_url,
					matched_patterns: route_state.matchedPatterns,
				};
				const pending = work_state.navigationHref
					? href_to_link_candidate(
							attribute_registry,
							work_state.navigationHref,
						)
					: null;

				return {
					active_exact: exact_link_match(target, route, match_rules),
					active_ancestor: ancestor_link_match(target, route),
					pending_exact: pending
						? exact_link_match(target, pending, match_rules)
						: false,
					pending_ancestor: pending
						? ancestor_link_match(target, pending)
						: false,
				};
			},
		};
	});
}

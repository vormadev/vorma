import type {
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RouteState,
} from "../core/types.ts";
import type { Core6PreparedRoute } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	core6_route_state_from_prepared,
	type Core6RoutePublishReason,
	type Core6RouteRenderState,
	type Core6RouteState,
} from "./route_publication.ts";

const core6_route_hook_abort_error_name = "AbortError";

export type Core6RouteHookInput = {
	current_render_state: Core6RouteRenderState | null;
	current_route: Core6RouteState | null;
	next_prepared: Core6PreparedRoute;
	reason: Core6RoutePublishReason;
	signal: AbortSignal;
};

export async function run_core6_route_transition_hooks(
	input: Core6RouteHookInput,
): Promise<void> {
	if (
		input.reason === core6_route_publish_reason.boot ||
		!input.current_route ||
		!input.current_render_state
	) {
		return;
	}
	throw_if_core6_route_hook_aborted(input.signal);

	const yield_hooks = input.current_render_state.entries
		.map((entry) => {
			return entry.module.default?.before_route_yield;
		})
		.filter((hook): hook is BeforeRouteYieldFn => {
			return typeof hook === "function";
		});
	const commit_hooks = input.next_prepared.matches
		.map((match) => {
			return match.module.default?.before_route_commit;
		})
		.filter((hook): hook is BeforeRouteCommitFn => {
			return typeof hook === "function";
		});
	if (yield_hooks.length === 0 && commit_hooks.length === 0) {
		return;
	}

	const args = {
		current: input.current_route as RouteState,
		next: core6_route_state_from_prepared(
			input.next_prepared,
		) as RouteState,
		signal: input.signal,
		trigger: input.reason,
	};
	await Promise.all(
		yield_hooks
			.map((hook) => {
				return hook(args);
			})
			.concat(
				commit_hooks.map((hook) => {
					return hook(args);
				}),
			),
	);
	throw_if_core6_route_hook_aborted(input.signal);
}

function throw_if_core6_route_hook_aborted(signal: AbortSignal): void {
	if (!signal.aborted) {
		return;
	}
	throw new DOMException("Aborted", core6_route_hook_abort_error_name);
}

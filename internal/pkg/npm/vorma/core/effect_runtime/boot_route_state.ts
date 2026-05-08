import { Effect, Ref } from "effect";
import type { RouteState } from "../types.ts";
import type { RoutePrepareInput, RouteRecord } from "./route_preparer.ts";
import { route_record_to_state } from "./route_publisher.ts";

export type BootRouteState = {
	capture: (input: {
		route: RouteRecord;
		prepare_input: RoutePrepareInput;
	}) => Effect.Effect<void>;
	clear: Effect.Effect<void>;
	snapshot: Effect.Effect<RouteState | null>;
};

export function make_boot_route_state(): Effect.Effect<BootRouteState, never> {
	return Effect.gen(function* () {
		const state_ref = yield* Ref.make<RouteState | null>(null);
		const capture: BootRouteState["capture"] = (input) => {
			if (input.prepare_input.trigger !== "boot") {
				return Effect.void;
			}
			return Ref.set(
				state_ref,
				route_record_to_state(
					input.route,
					input.prepare_input.href,
					input.prepare_input.history_state,
				),
			);
		};

		return {
			capture,
			clear: Ref.set(state_ref, null),
			snapshot: Ref.get(state_ref),
		};
	});
}

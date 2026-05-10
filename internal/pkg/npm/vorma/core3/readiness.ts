import type {
	BrowserPosition,
	CoreEffect,
	CoreOperation,
	OperationID,
	PreparedRoute,
	RouteSnapshot,
} from "./model.ts";
import { route_publication_target } from "./publication.ts";
import { prepared_route_at_position, route_facts_to_state } from "./route.ts";

export type RouteReadinessOperation = Extract<
	CoreOperation,
	{ kind: "boot" | "navigation" | "popstate" | "route_revalidation" }
>;

export type RouteReadinessInput = {
	browser: BrowserPosition | null;
	current: RouteSnapshot | null;
	hooks_required: boolean;
	operation: RouteReadinessOperation;
	prepared: PreparedRoute;
};

export type RouteReadinessPlan =
	| {
			kind: "publishable";
			operation_id: OperationID;
			prepared: PreparedRoute;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "hooks_running";
			operation_id: OperationID;
	  };

export function plan_route_readiness(
	input: RouteReadinessInput,
): RouteReadinessPlan | undefined {
	if (
		input.operation.kind === "boot" ||
		!input.hooks_required ||
		!input.current ||
		input.current.provisional
	) {
		return {
			kind: "publishable",
			operation_id: input.operation.id,
			prepared: input.prepared,
		};
	}
	const target = route_publication_target({
		browser: input.browser,
		operation: input.operation,
	});
	if (!target || target.reason === "boot") {
		return undefined;
	}
	const prepared = prepared_route_at_position(
		input.prepared,
		target.position,
	);
	return {
		effects: [
			{
				operation_id: input.operation.id,
				request: {
					current_route: route_facts_to_state(input.current.route),
					next_route: route_facts_to_state(prepared.route),
					prepared,
					trigger: target.reason,
				},
				type: "run_route_hooks",
			},
		],
		kind: "hooks_running",
		operation_id: input.operation.id,
	};
}

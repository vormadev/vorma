import {
	core6_route_preparation_trigger,
	type Core6RawRoutePayload,
} from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RoutePublication,
	type Core6RouteScroll,
	type Core6RouteState,
} from "./route_publication.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

export const core6_route_boot_result_kind = {
	already_booted: "already_booted",
	booted: "booted",
	interrupted: "interrupted",
	invalid: "invalid",
} as const;

export type Core6RouteBootResultKind =
	(typeof core6_route_boot_result_kind)[keyof typeof core6_route_boot_result_kind];

export type Core6RouteBootRuntime = Pick<
	Core6RouteRuntime,
	"current_route" | "run_route_payload"
>;

export type Core6RouteBootConfig = {
	runtime: Core6RouteBootRuntime;
};

export type Core6RouteBootInput = {
	history_state: unknown;
	href: string;
	payload: Core6RawRoutePayload;
	scroll?: Core6RouteScroll;
};

export type Core6RouteBootStatus = {
	href: string;
};

export type Core6RouteBootAlreadyBootedResult = {
	kind: typeof core6_route_boot_result_kind.already_booted;
	route: Core6RouteState;
};

export type Core6RouteBootBootedResult = {
	kind: typeof core6_route_boot_result_kind.booted;
	publication: Core6RoutePublication;
};

export type Core6RouteBootInterruptedResult = {
	kind: typeof core6_route_boot_result_kind.interrupted;
	reason: string;
};

export type Core6RouteBootInvalidResult = {
	error: string;
	kind: typeof core6_route_boot_result_kind.invalid;
};

export type Core6RouteBootResult =
	| Core6RouteBootAlreadyBootedResult
	| Core6RouteBootBootedResult
	| Core6RouteBootInterruptedResult
	| Core6RouteBootInvalidResult;

export type Core6RouteBootOwner = {
	boot: (input: Core6RouteBootInput) => Promise<Core6RouteBootResult>;
	current_status: () => Core6RouteBootStatus | null;
};

export function create_core6_route_boot_owner(
	config: Core6RouteBootConfig,
): Core6RouteBootOwner {
	let current_status: Core6RouteBootStatus | null = null;
	let next_run_id = 0;

	return {
		boot: async (input) => {
			const current_route = config.runtime.current_route();
			if (current_route) {
				return {
					kind: core6_route_boot_result_kind.already_booted,
					route: current_route,
				};
			}
			let target: URL;
			try {
				target = new URL(input.href);
			} catch (error) {
				return {
					error:
						error instanceof Error ? error.message : String(error),
					kind: core6_route_boot_result_kind.invalid,
				};
			}
			next_run_id++;
			const run_id = next_run_id;
			current_status = {
				href: target.href,
			};
			try {
				const result = await config.runtime.run_route_payload({
					intent: {
						history_state: input.history_state,
						href: target.href,
						preparation_trigger:
							core6_route_preparation_trigger.boot,
						publish_reason: core6_route_publish_reason.boot,
						scroll: input.scroll,
						search_params: target.searchParams,
					},
					kind: core6_route_transaction_kind.boot,
					payload: input.payload,
				});
				if (!result.ok) {
					return {
						kind: core6_route_boot_result_kind.interrupted,
						reason: result.reason,
					};
				}
				return {
					kind: core6_route_boot_result_kind.booted,
					publication: result.value,
				};
			} finally {
				if (run_id === next_run_id) {
					current_status = null;
				}
			}
		},
		current_status: () => {
			if (!current_status) {
				return null;
			}
			return { ...current_status };
		},
	};
}

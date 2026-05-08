import { Effect } from "effect";
import { BUILD_ID_HEADER } from "../constants.ts";
import type { RouteState } from "../types.ts";
import type { BuildSkewDetectedEvent, WorkState } from "./client_contract.ts";

export type BuildSkewReporter = {
	report: (input: BuildSkewReportInput) => Effect.Effect<boolean>;
};

export type BuildSkewReporterOptions = {
	client_build_id: string;
	current_route_state: Effect.Effect<RouteState | null>;
	current_work_state: Effect.Effect<WorkState>;
	on_detected?: (event: BuildSkewDetectedEvent) => Effect.Effect<void>;
};

export type BuildSkewReportInput = {
	response: Response;
	triggering_response: BuildSkewDetectedEvent["triggeringResponse"];
	default_behavior: BuildSkewDetectedEvent["defaultBehavior"];
};

export function make_build_skew_reporter(
	options: BuildSkewReporterOptions,
): BuildSkewReporter {
	return {
		report: (input) => {
			return Effect.gen(function* () {
				const server_build_id =
					input.response.headers.get(BUILD_ID_HEADER) ?? "";
				if (
					server_build_id.length === 0 ||
					server_build_id === options.client_build_id
				) {
					return false;
				}
				const current_route_state = yield* options.current_route_state;
				if (!current_route_state) {
					return false;
				}
				const current_work_state = yield* options.current_work_state;
				if (options.on_detected) {
					yield* options.on_detected({
						activeClientBuildID: options.client_build_id,
						serverBuildID: server_build_id,
						triggeringResponse: input.triggering_response,
						currentRouteState: current_route_state,
						currentWorkState: current_work_state,
						defaultBehavior: input.default_behavior,
					});
				}
				return true;
			}).pipe(
				Effect.catchAll(() => {
					return Effect.succeed(false);
				}),
			);
		},
	};
}

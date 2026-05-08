import { Effect } from "effect";
import type { RevalidationResult } from "../types.ts";
import type { BuildSkewReporter } from "./build_skew_reporter.ts";
import type { RevalidationReason } from "./client_contract.ts";
import type { NavigationActor } from "./navigation_actor.ts";
import {
	make_revalidation_coordinator,
	RevalidationAttemptFailed,
	RevalidationBuildSkew,
	type RevalidationAttemptInput,
	type RevalidationCoordinatorSnapshot,
} from "./revalidation_coordinator.ts";
import type { RouteFetcher, RouteFetchResult } from "./route_fetcher.ts";
import type { RoutePreparer } from "./route_preparer.ts";
import type { HistoryPosition, RoutePublisher } from "./route_publisher.ts";
import {
	WORK_REVALIDATION_STATUS_DEBOUNCING,
	WORK_REVALIDATION_STATUS_RETRYING,
	WORK_REVALIDATION_STATUS_RUNNING,
	type WorkRevalidationInput,
	type WorkStateActor,
} from "./work_state_actor.ts";

export type RouteRevalidator = {
	request: (
		reason: Exclude<RevalidationReason, "retry">,
		options?: { debounce?: boolean; skipWorkIndicator?: boolean },
	) => Effect.Effect<RevalidationResult>;
	cancel: Effect.Effect<void>;
	snapshot: Effect.Effect<RevalidationCoordinatorSnapshot>;
	shutdown: Effect.Effect<void>;
};

export type RouteRevalidatorOptions = {
	build_skew_reporter: BuildSkewReporter;
	current_position: () => Effect.Effect<HistoryPosition>;
	fetcher: RouteFetcher;
	navigation_actor: NavigationActor;
	route_key: (href: string) => string;
	route_preparer: RoutePreparer;
	route_publisher: RoutePublisher;
	work_actor: WorkStateActor;
};

export function make_route_revalidator(
	options: RouteRevalidatorOptions,
): Effect.Effect<RouteRevalidator, never> {
	return Effect.gen(function* () {
		const coordinator = yield* make_revalidation_coordinator({
			run: (input) => {
				return run_revalidation_attempt(options, input);
			},
			on_snapshot_change: (snapshot) => {
				return options.work_actor.set_revalidation(
					work_revalidation_from_snapshot(snapshot),
				);
			},
		});
		return {
			request: coordinator.request,
			cancel: coordinator.cancel(),
			snapshot: coordinator.snapshot,
			shutdown: coordinator.shutdown,
		};
	});
}

function run_revalidation_attempt(
	options: RouteRevalidatorOptions,
	input: RevalidationAttemptInput,
): Effect.Effect<void, RevalidationAttemptFailed | RevalidationBuildSkew> {
	return Effect.gen(function* () {
		if (input.reason !== "windowFocus") {
			yield* options.navigation_actor.idle;
		}
		const position = yield* options.current_position();
		const expected_route_key = options.route_key(position.href);
		const url = new URL(position.href);
		const fetch_result = yield* options.fetcher
			.fetch_route({ url, revalidation: true })
			.pipe(Effect.mapError(attempt_failed));
		if (fetch_result.kind === "build_skew") {
			yield* report_build_skew(
				options,
				input.reason,
				position,
				fetch_result,
			);
			return yield* Effect.fail(new RevalidationBuildSkew());
		}
		if (fetch_result.kind === "redirect") {
			const navigation_result = yield* options.navigation_actor
				.navigate(fetch_result.href, {
					replace: true,
					source: "redirect",
					state: position.state,
				})
				.pipe(Effect.mapError(attempt_failed));
			if (navigation_result.didNavigate) {
				return;
			}
			return yield* Effect.fail(attempt_failed(fetch_result));
		}
		if (fetch_result.kind !== "data") {
			return yield* Effect.fail(attempt_failed(fetch_result));
		}
		const prepared = yield* options.route_preparer
			.prepare_route({
				raw_payload: fetch_result.data,
				url,
				trigger: "revalidation",
				href: position.href,
				history_state: position.state,
			})
			.pipe(Effect.mapError(attempt_failed));
		const publish_position = yield* options.current_position();
		const work = yield* options.work_actor.snapshot;
		const publish_result = yield* options.route_publisher
			.publish({
				reason: "revalidation",
				prepared,
				position: publish_position,
				work,
				guard: options.current_position().pipe(
					Effect.map((current_position) => {
						return (
							options.route_key(current_position.href) ===
							expected_route_key
						);
					}),
				),
			})
			.pipe(Effect.mapError(attempt_failed));
		if (!publish_result.did_publish) {
			return yield* Effect.fail(attempt_failed(publish_result));
		}
	});
}

function work_revalidation_from_snapshot(
	snapshot: RevalidationCoordinatorSnapshot,
): WorkRevalidationInput | null {
	if (snapshot.phase === "idle") {
		return null;
	}
	if (snapshot.phase === "debouncing") {
		return {
			status: WORK_REVALIDATION_STATUS_DEBOUNCING,
			attempt: snapshot.attempt,
			skipWorkIndicator: snapshot.skipWorkIndicator,
		};
	}
	if (snapshot.phase === "retrying") {
		return {
			status: WORK_REVALIDATION_STATUS_RETRYING,
			attempt: snapshot.attempt,
			skipWorkIndicator: snapshot.skipWorkIndicator,
		};
	}
	return {
		status: WORK_REVALIDATION_STATUS_RUNNING,
		attempt: snapshot.attempt,
		skipWorkIndicator: snapshot.skipWorkIndicator,
	};
}

function attempt_failed(error: unknown): RevalidationAttemptFailed {
	return new RevalidationAttemptFailed({ error });
}

function report_build_skew(
	options: RouteRevalidatorOptions,
	reason: RevalidationReason,
	position: HistoryPosition,
	result: Extract<RouteFetchResult, { kind: "build_skew" }>,
): Effect.Effect<void> {
	return options.build_skew_reporter
		.report({
			response: result.response,
			triggering_response: {
				kind: "route",
				trigger: "revalidation",
				revalidationReason: reason,
				requestedHref: position.href,
				status: result.response.status,
				ok: result.response.ok,
			},
			default_behavior: "dropResponse",
		})
		.pipe(
			Effect.asVoid,
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
}

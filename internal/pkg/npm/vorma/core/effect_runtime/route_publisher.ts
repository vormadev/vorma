import { Data, Effect, Ref } from "effect";
import { jsonDeepEquals } from "vorma/kit/json";
import type {
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RouteState,
	RouteUpdateReason,
} from "../types.ts";
import type {
	ClientCommit,
	CommitFn,
	RouteRenderState,
	ScrollIntent,
	ScrollState,
	WorkState,
} from "./client_contract.ts";
import type {
	PreparedRoute,
	RoutePreparationFailed,
	RouteRecord,
} from "./route_preparer.ts";

export type HistoryPosition = {
	href: string;
	key: string;
	state: unknown;
};

export type RouteSnapshot = {
	position: HistoryPosition;
	route: RouteRecord;
};

export type RoutePublishReason =
	| "initial"
	| "navigation"
	| "popstate"
	| "revalidation"
	| "hmr";

export type RoutePublishInput = {
	reason: RoutePublishReason;
	prepared: PreparedRoute;
	position: HistoryPosition;
	scroll?: ScrollState;
	work?: WorkState;
	guard?: Effect.Effect<boolean>;
	signal?: AbortSignal;
};

export type RoutePositionMoveInput = {
	reason: Extract<RoutePublishReason, "navigation" | "popstate">;
	position: HistoryPosition;
	scroll?: ScrollState;
	work?: WorkState;
};

export type RoutePublishResult = {
	did_publish: boolean;
	did_update_route: boolean;
	snapshot: RouteSnapshot | null;
};

export type RoutePublisher = {
	publish: (
		input: RoutePublishInput,
	) => Effect.Effect<RoutePublishResult, RoutePublishFailed>;
	replace_current_route: (input: {
		reason: Extract<RoutePublishReason, "hmr">;
		route: RouteRecord;
		work?: WorkState;
	}) => Effect.Effect<RoutePublishResult, RouteCommitFailed>;
	move_position: (
		input: RoutePositionMoveInput,
	) => Effect.Effect<RoutePublishResult, RouteCommitFailed>;
	snapshot: Effect.Effect<RouteSnapshot | null>;
	route_state: Effect.Effect<RouteState | null>;
};

export type RoutePublisherOptions = {
	commit: CommitFn;
	initial_snapshot?: RouteSnapshot | null;
	apply_scroll?: (scroll: ScrollState) => Effect.Effect<void>;
	run_view_transition?: (
		publish: Effect.Effect<RoutePublishResult, RoutePublishFailed>,
	) => Effect.Effect<RoutePublishResult, RoutePublishFailed>;
};

export class RouteTransitionHookFailed extends Data.TaggedError(
	"RouteTransitionHookFailed",
)<{
	readonly error: unknown;
}> {}

export class RouteCommitFailed extends Data.TaggedError("RouteCommitFailed")<{
	readonly error: unknown;
}> {}

export type RoutePublishFailed =
	| RouteTransitionHookFailed
	| RouteCommitFailed
	| RoutePreparationFailed;

export function make_route_publisher(
	options: RoutePublisherOptions,
): Effect.Effect<RoutePublisher, never> {
	return Effect.gen(function* () {
		const snapshot_ref = yield* Ref.make<RouteSnapshot | null>(
			options.initial_snapshot ?? null,
		);

		const publish = (
			input: RoutePublishInput,
		): Effect.Effect<RoutePublishResult, RoutePublishFailed> => {
			const publish_effect = Effect.gen(function* () {
				const previous_snapshot = yield* Ref.get(snapshot_ref);
				const next_snapshot: RouteSnapshot = {
					position: input.position,
					route: input.prepared.route,
				};
				yield* run_transition_hooks(
					input,
					previous_snapshot,
					next_snapshot,
				);
				const can_publish = yield* should_publish(input.guard);
				if (!can_publish || input.signal?.aborted) {
					return {
						did_publish: false,
						did_update_route: false,
						snapshot: previous_snapshot,
					};
				}
				yield* input.prepared.apply_dom_side_effects;
				yield* Ref.set(snapshot_ref, next_snapshot);
				const client_commit = to_client_commit(
					input,
					previous_snapshot,
					next_snapshot,
				);
				yield* emit_commit(options.commit, client_commit);
				if (input.scroll && options.apply_scroll) {
					yield* options.apply_scroll(input.scroll);
				}
				return {
					did_publish: true,
					did_update_route: client_commit.route_update !== undefined,
					snapshot: next_snapshot,
				};
			});
			if (input.reason === "navigation" && options.run_view_transition) {
				return options.run_view_transition(publish_effect);
			}
			return publish_effect;
		};

		const move_position = (
			input: RoutePositionMoveInput,
		): Effect.Effect<RoutePublishResult, RouteCommitFailed> => {
			return Effect.gen(function* () {
				const previous_snapshot = yield* Ref.get(snapshot_ref);
				if (!previous_snapshot) {
					return {
						did_publish: false,
						did_update_route: false,
						snapshot: null,
					};
				}
				const next_snapshot: RouteSnapshot = {
					position: input.position,
					route: previous_snapshot.route,
				};
				yield* Ref.set(snapshot_ref, next_snapshot);
				const client_commit = to_client_commit(
					input,
					previous_snapshot,
					next_snapshot,
				);
				yield* emit_commit(options.commit, client_commit);
				if (input.scroll && options.apply_scroll) {
					yield* options.apply_scroll(input.scroll);
				}
				return {
					did_publish: true,
					did_update_route: client_commit.route_update !== undefined,
					snapshot: next_snapshot,
				};
			});
		};

		const replace_current_route = (input: {
			reason: Extract<RoutePublishReason, "hmr">;
			route: RouteRecord;
			work?: WorkState;
		}): Effect.Effect<RoutePublishResult, RouteCommitFailed> => {
			return Effect.gen(function* () {
				const previous_snapshot = yield* Ref.get(snapshot_ref);
				if (!previous_snapshot) {
					return {
						did_publish: false,
						did_update_route: false,
						snapshot: null,
					};
				}
				const next_snapshot: RouteSnapshot = {
					position: previous_snapshot.position,
					route: input.route,
				};
				yield* Ref.set(snapshot_ref, next_snapshot);
				const client_commit = to_client_commit(
					input,
					previous_snapshot,
					next_snapshot,
				);
				yield* emit_commit(options.commit, client_commit);
				return {
					did_publish: true,
					did_update_route: client_commit.route_update !== undefined,
					snapshot: next_snapshot,
				};
			});
		};

		return {
			publish,
			replace_current_route,
			move_position,
			snapshot: Ref.get(snapshot_ref),
			route_state: Ref.get(snapshot_ref).pipe(
				Effect.map((snapshot) => {
					if (!snapshot) {
						return null;
					}
					return route_snapshot_to_state(snapshot);
				}),
			),
		};
	});
}

function should_publish(
	guard: Effect.Effect<boolean> | undefined,
): Effect.Effect<boolean> {
	if (!guard) {
		return Effect.succeed(true);
	}
	return guard;
}

function run_transition_hooks(
	input: RoutePublishInput,
	previous_snapshot: RouteSnapshot | null,
	next_snapshot: RouteSnapshot,
): Effect.Effect<void, RouteTransitionHookFailed> {
	if (!previous_snapshot || input.reason === "initial") {
		return Effect.void;
	}
	const trigger = to_transition_trigger(input.reason);
	const current = route_snapshot_to_state(previous_snapshot);
	const next = route_snapshot_to_state(next_snapshot);
	const yield_hooks = route_hooks(
		previous_snapshot.route,
		"before_route_yield",
	);
	const commit_hooks = route_hooks(
		next_snapshot.route,
		"before_route_commit",
	);
	return Effect.forEach(
		[...yield_hooks, ...commit_hooks],
		(hook) => {
			return run_transition_hook(hook, {
				trigger,
				signal: input.signal ?? new AbortController().signal,
				current,
				next,
			});
		},
		{ concurrency: "unbounded", discard: true },
	);
}

function route_hooks(
	route: RouteRecord,
	kind: "before_route_commit" | "before_route_yield",
): Array<BeforeRouteCommitFn | BeforeRouteYieldFn> {
	return route.matches
		.map((match) => {
			const view_definition = match.module.default as
				| {
						before_route_commit?: BeforeRouteCommitFn;
						before_route_yield?: BeforeRouteYieldFn;
				  }
				| undefined;
			return view_definition?.[kind];
		})
		.filter((hook): hook is BeforeRouteCommitFn | BeforeRouteYieldFn => {
			return typeof hook === "function";
		});
}

function run_transition_hook(
	hook: BeforeRouteCommitFn | BeforeRouteYieldFn,
	args: {
		trigger: Exclude<RouteUpdateReason, "boot">;
		signal: AbortSignal;
		current: RouteState;
		next: RouteState;
	},
): Effect.Effect<void, RouteTransitionHookFailed> {
	return Effect.tryPromise({
		try: () => {
			return Promise.resolve(hook(args));
		},
		catch: (error) => {
			return new RouteTransitionHookFailed({ error });
		},
	});
}

function to_client_commit(
	input: {
		reason: RoutePublishReason;
		scroll?: ScrollState;
		work?: WorkState;
	},
	previous_snapshot: RouteSnapshot | null,
	next_snapshot: RouteSnapshot,
): ClientCommit {
	const previous_route = previous_snapshot
		? route_snapshot_to_state(previous_snapshot)
		: null;
	const route = route_snapshot_to_state(next_snapshot);
	const client_commit: ClientCommit = {
		route_render: {
			state: route_to_render_state(
				next_snapshot.route,
				next_snapshot.position.state,
			),
			scroll_intent: to_scroll_intent(input.scroll, next_snapshot.route),
		},
		work: input.work,
	};
	if (!previous_route || !jsonDeepEquals(previous_route, route)) {
		client_commit.route_update = {
			previous_route,
			reason: to_route_update_reason(input.reason),
			route,
		};
	}
	return client_commit;
}

function emit_commit(
	commit: CommitFn,
	client_commit: ClientCommit,
): Effect.Effect<void, RouteCommitFailed> {
	return Effect.try({
		try: () => {
			commit(client_commit);
		},
		catch: (error) => {
			return new RouteCommitFailed({ error });
		},
	});
}

export function route_snapshot_to_state(snapshot: RouteSnapshot): RouteState {
	return route_record_to_state(
		snapshot.route,
		snapshot.position.href,
		snapshot.position.state,
	);
}

export function route_record_to_state(
	route: RouteRecord,
	href: string,
	history_state: unknown,
): RouteState {
	return {
		href,
		historyState: history_state,
		clientBuildID: route.client_build_id,
		params: route.params,
		splatValues: route.splat_values,
		matches: route.matches.map((match) => {
			return {
				pattern: match.pattern,
				input: match.input,
				loaderData: match.loader_data,
				clientLoaderData: match.client_loader_data,
			};
		}),
		error: route.error,
	};
}

function route_to_render_state(
	route: RouteRecord,
	history_state: unknown,
): RouteRenderState {
	return {
		entries: route.matches.map((match) => {
			return {
				pattern: match.pattern,
				input: match.input,
				module_url: match.module_url,
				module: match.module,
				loader_data: match.loader_data,
				client_loader_data: match.client_loader_data,
			};
		}),
		error: route.error,
		params: route.params,
		splat_values: route.splat_values,
		client_build_id: route.client_build_id,
		history_state,
	};
}

function to_route_update_reason(reason: RoutePublishReason): RouteUpdateReason {
	if (reason === "initial") {
		return "boot";
	}
	if (reason === "hmr") {
		return "revalidation";
	}
	return reason;
}

function to_transition_trigger(
	reason: RoutePublishReason,
): Exclude<RouteUpdateReason, "boot"> {
	if (reason === "initial") {
		return "navigation";
	}
	if (reason === "hmr") {
		return "revalidation";
	}
	return reason;
}

function to_scroll_intent(
	scroll: ScrollState | undefined,
	route: RouteRecord,
): ScrollIntent | undefined {
	if (!scroll) {
		return undefined;
	}
	const target_idx = Math.max(0, route.matches.length - 1);
	return {
		scroll,
		target_route_id: make_route_id(
			target_idx,
			route.matches[target_idx]?.pattern ?? "",
		),
	};
}

function make_route_id(idx: number, pattern: string): string {
	return `${idx}:${pattern}`;
}

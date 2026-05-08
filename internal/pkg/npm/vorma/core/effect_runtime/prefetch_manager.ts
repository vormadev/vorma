import { Deferred, Effect, Fiber, Queue, Ref } from "effect";
import type { RouteFetcher } from "./route_fetcher.ts";
import type {
	ClientLoaderPrestart,
	PreparedRoute,
	RoutePreparer,
} from "./route_preparer.ts";
import type { WorkStateActor } from "./work_state_actor.ts";

export type PrefetchedRoute = {
	href: string;
	key: string;
	prepared: PreparedRoute;
};

export type PrefetchStartInput = {
	href: string;
	current_route_key: string;
	active_navigation_key?: string | null;
	history_state?: unknown;
};

export type PrefetchSnapshot = {
	active: null | {
		id: number;
		href: string;
		key: string;
	};
	prepared: null | {
		href: string;
		key: string;
	};
	nextID: number;
};

export type PrefetchManager = {
	start: (input: PrefetchStartInput) => Effect.Effect<void>;
	stop: (href: string) => Effect.Effect<void>;
	take: (href: string) => Effect.Effect<PrefetchedRoute | null>;
	snapshot: Effect.Effect<PrefetchSnapshot>;
	shutdown: Effect.Effect<void>;
};

export type PrefetchManagerOptions = {
	fetcher: RouteFetcher;
	is_external: (href: string) => boolean;
	route_key: (href: string) => string;
	route_preparer: RoutePreparer;
	prestart?: (input: {
		href: string;
		history_state: unknown;
		signal: AbortSignal;
		url: URL;
	}) => Effect.Effect<ClientLoaderPrestart[]>;
	work_actor: WorkStateActor;
};

type ActivePrefetch = {
	readonly id: number;
	readonly href: string;
	readonly key: string;
	readonly history_state: unknown;
	readonly controller: AbortController;
	readonly fiber: Fiber.RuntimeFiber<void, never>;
	readonly client_loader_prestarts: ClientLoaderPrestart[];
	readonly result: Deferred.Deferred<PrefetchedRoute | null>;
};

type Model = {
	readonly nextID: number;
	readonly active: ActivePrefetch | null;
	readonly prepared: PrefetchedRoute | null;
};

type Command =
	| {
			readonly _tag: "Start";
			readonly input: PrefetchStartInput;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: "Stop";
			readonly href: string;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: "Take";
			readonly href: string;
			readonly ack: Deferred.Deferred<PrefetchedRoute | null>;
	  }
	| {
			readonly _tag: "Completed";
			readonly id: number;
			readonly key: string;
			readonly href: string;
			readonly prepared: PreparedRoute;
	  }
	| {
			readonly _tag: "Failed";
			readonly id: number;
			readonly key: string;
	  }
	| {
			readonly _tag: "Shutdown";
	  };

export function make_prefetch_manager(
	options: PrefetchManagerOptions,
): Effect.Effect<PrefetchManager, never> {
	return Effect.gen(function* () {
		const queue = yield* Queue.unbounded<Command>();
		const model = yield* Ref.make<Model>({
			nextID: 0,
			active: null,
			prepared: null,
		});

		const snapshot_from_model = (current: Model): PrefetchSnapshot => {
			return {
				active: current.active
					? {
							id: current.active.id,
							href: current.active.href,
							key: current.active.key,
						}
					: null,
				prepared: current.prepared
					? {
							href: current.prepared.href,
							key: current.prepared.key,
						}
					: null,
				nextID: current.nextID,
			};
		};

		const cancel_active = (active: ActivePrefetch): Effect.Effect<void> => {
			return Effect.gen(function* () {
				yield* Effect.sync(() => {
					active.controller.abort();
				});
				yield* Effect.forEach(
					active.client_loader_prestarts,
					(prestart) => {
						return prestart.abort;
					},
					{ discard: true },
				);
				yield* Fiber.interruptFork(active.fiber);
				yield* Deferred.succeed(active.result, null);
			});
		};

		const clear_work = (): Effect.Effect<void> => {
			return options.work_actor.set_prefetch(null);
		};

		const set_active_work = (href: string): Effect.Effect<void> => {
			return options.work_actor.set_prefetch({ href });
		};

		const complete_prefetch = (
			id: number,
			key: string,
			href: string,
			prepared: PreparedRoute,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== id || current.active.key !== key) {
					return;
				}
				yield* Ref.set(model, {
					...current,
					active: null,
					prepared: {
						href,
						key,
						prepared,
					},
				});
				yield* Deferred.succeed(current.active.result, {
					href,
					key,
					prepared,
				});
				yield* clear_work();
			});
		};

		const fail_prefetch = (
			id: number,
			key: string,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== id || current.active.key !== key) {
					return;
				}
				yield* Effect.forEach(
					current.active.client_loader_prestarts,
					(prestart) => {
						return prestart.abort;
					},
					{ discard: true },
				);
				yield* Ref.set(model, {
					...current,
					active: null,
				});
				yield* Deferred.succeed(current.active.result, null);
				yield* clear_work();
			});
		};

		const run_prefetch = (
			id: number,
			href: string,
			key: string,
			history_state: unknown,
			controller: AbortController,
			client_loader_prestarts: ClientLoaderPrestart[],
		): Effect.Effect<void, never> => {
			return Effect.gen(function* () {
				const url = new URL(href);
				const fetch_result = yield* options.fetcher.fetch_route({
					url,
					signal: controller.signal,
				});
				if (fetch_result.kind !== "data") {
					yield* fail_prefetch(id, key);
					return;
				}
				const prepared = yield* options.route_preparer.prepare_route({
					raw_payload: fetch_result.data,
					url,
					trigger: "prefetch",
					href,
					history_state,
					signal: controller.signal,
					client_loader_prestarts,
				});
				yield* complete_prefetch(id, key, href, prepared);
			}).pipe(
				Effect.catchAll(() => {
					return Effect.gen(function* () {
						yield* Effect.forEach(
							client_loader_prestarts,
							(prestart) => {
								return prestart.abort;
							},
							{ discard: true },
						);
						yield* fail_prefetch(id, key);
					});
				}),
				Effect.catchAllDefect(() => {
					return Effect.gen(function* () {
						yield* Effect.forEach(
							client_loader_prestarts,
							(prestart) => {
								return prestart.abort;
							},
							{ discard: true },
						);
						yield* fail_prefetch(id, key);
					});
				}),
				Effect.asVoid,
			);
		};

		const prestart_client_loaders = (
			href: string,
			history_state: unknown,
			controller: AbortController,
		): Effect.Effect<ClientLoaderPrestart[]> => {
			if (!options.prestart) {
				return Effect.succeed([]);
			}
			return options.prestart({
				href,
				history_state,
				signal: controller.signal,
				url: new URL(href),
			});
		};

		const on_start = (
			command: Extract<Command, { _tag: "Start" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const href = command.input.href;
				const key = options.route_key(href);
				const current = yield* Ref.get(model);
				if (
					options.is_external(href) ||
					key === command.input.current_route_key ||
					key === command.input.active_navigation_key ||
					current.active?.key === key ||
					current.prepared?.key === key
				) {
					yield* Deferred.succeed(command.ack, undefined);
					return;
				}
				if (current.active) {
					yield* cancel_active(current.active);
				}
				const id = current.nextID + 1;
				const controller = new AbortController();
				const result = yield* Deferred.make<PrefetchedRoute | null>();
				const client_loader_prestarts = yield* prestart_client_loaders(
					href,
					command.input.history_state,
					controller,
				);
				const fiber = yield* Effect.forkDaemon(
					run_prefetch(
						id,
						href,
						key,
						command.input.history_state,
						controller,
						client_loader_prestarts,
					),
				);
				yield* Ref.set(model, {
					nextID: id,
					active: {
						id,
						href,
						key,
						history_state: command.input.history_state,
						controller,
						fiber,
						client_loader_prestarts,
						result,
					},
					prepared: current.prepared,
				});
				yield* set_active_work(href);
				yield* Deferred.succeed(command.ack, undefined);
			});
		};

		const on_stop = (
			command: Extract<Command, { _tag: "Stop" }>,
		): Effect.Effect<void> => {
			return stop_now(command.href).pipe(
				Effect.andThen(Deferred.succeed(command.ack, undefined)),
			);
		};

		const stop_now = (href: string): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const key = options.route_key(href);
				const current = yield* Ref.get(model);
				const should_clear_prepared = current.prepared?.key === key;
				if (current.active?.key !== key) {
					if (should_clear_prepared) {
						yield* Ref.set(model, { ...current, prepared: null });
					}
					return;
				}
				yield* cancel_active(current.active);
				yield* Ref.set(model, {
					...current,
					active: null,
					prepared: should_clear_prepared ? null : current.prepared,
				});
				yield* clear_work();
			});
		};

		const take_now = (
			href: string,
		): Effect.Effect<PrefetchedRoute | null> => {
			return Effect.gen(function* () {
				const key = options.route_key(href);
				const current = yield* Ref.get(model);
				if (current.prepared?.key !== key) {
					if (current.active?.key === key) {
						const result = yield* Deferred.await(
							current.active.result,
						);
						if (!result) {
							return null;
						}
						const next = yield* Ref.get(model);
						if (next.prepared?.key === key) {
							yield* Ref.set(model, {
								...next,
								prepared: null,
							});
						}
						return result;
					}
					return null;
				}
				const prepared = current.prepared;
				yield* Ref.set(model, { ...current, prepared: null });
				return prepared;
			});
		};

		const on_take = (
			command: Extract<Command, { _tag: "Take" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const prepared = yield* take_now(command.href);
				yield* Deferred.succeed(command.ack, prepared);
			});
		};

		const on_completed = (
			command: Extract<Command, { _tag: "Completed" }>,
		): Effect.Effect<void> => {
			return complete_prefetch(
				command.id,
				command.key,
				command.href,
				command.prepared,
			);
		};

		const on_failed = (
			command: Extract<Command, { _tag: "Failed" }>,
		): Effect.Effect<void> => {
			return fail_prefetch(command.id, command.key);
		};

		const command_program = (command: Command): Effect.Effect<void> => {
			switch (command._tag) {
				case "Start": {
					return on_start(command);
				}
				case "Stop": {
					return on_stop(command);
				}
				case "Take": {
					return on_take(command);
				}
				case "Completed": {
					return on_completed(command);
				}
				case "Failed": {
					return on_failed(command);
				}
				case "Shutdown": {
					return Effect.gen(function* () {
						const current = yield* Ref.get(model);
						if (current.active) {
							yield* cancel_active(current.active);
						}
						yield* clear_work();
						yield* Queue.shutdown(queue);
					});
				}
			}
		};

		const actor = Queue.take(queue).pipe(
			Effect.flatMap(command_program),
			Effect.forever,
			Effect.ensuring(
				Effect.gen(function* () {
					const current = yield* Ref.get(model);
					if (current.active) {
						yield* cancel_active(current.active);
					}
					yield* clear_work();
				}),
			),
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.forkDaemon(actor);

		const offer_void = (
			command_from_ack: (ack: Deferred.Deferred<void>) => Command,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const ack = yield* Deferred.make<void>();
				yield* Queue.offer(queue, command_from_ack(ack));
				yield* Deferred.await(ack);
			});
		};

		return {
			start: (input) => {
				return offer_void((ack) => {
					return { _tag: "Start", input, ack };
				});
			},
			stop: stop_now,
			take: take_now,
			snapshot: Ref.get(model).pipe(
				Effect.map((current) => {
					return snapshot_from_model(current);
				}),
			),
			shutdown: Queue.offer(queue, { _tag: "Shutdown" }).pipe(
				Effect.andThen(Fiber.join(actor_fiber)),
				Effect.asVoid,
				Effect.catchAll(() => {
					return Effect.void;
				}),
			),
		};
	});
}

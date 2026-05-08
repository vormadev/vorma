import { Deferred, Effect, Fiber, Ref } from "effect";
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
	readonly fiber: Fiber.Fiber<void, never> | null;
	readonly client_loader_prestarts: ClientLoaderPrestart[];
	readonly result: Deferred.Deferred<PrefetchedRoute | null>;
};

type Model = {
	readonly nextID: number;
	readonly active: ActivePrefetch | null;
	readonly prepared: PrefetchedRoute | null;
};

export function make_prefetch_manager(
	options: PrefetchManagerOptions,
): Effect.Effect<PrefetchManager, never> {
	return Effect.gen(function* () {
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
				if (active.fiber) {
					yield* Effect.forkDetach(Fiber.interrupt(active.fiber), {
						startImmediately: true,
					}).pipe(Effect.asVoid);
				}
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
				Effect.catch(() => {
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
				Effect.catchDefect(() => {
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

		const start_now = (input: PrefetchStartInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const href = input.href;
				const key = options.route_key(href);
				const current = yield* Ref.get(model);
				if (
					options.is_external(href) ||
					key === input.current_route_key ||
					key === input.active_navigation_key ||
					current.active?.key === key ||
					current.prepared?.key === key
				) {
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
					input.history_state,
					controller,
				);
				yield* Ref.set(model, {
					nextID: id,
					active: {
						id,
						href,
						key,
						history_state: input.history_state,
						controller,
						fiber: null,
						client_loader_prestarts,
						result,
					},
					prepared: current.prepared,
				});
				yield* set_active_work(href);
				const fiber = yield* Effect.forkDetach(
					run_prefetch(
						id,
						href,
						key,
						input.history_state,
						controller,
						client_loader_prestarts,
					),
					{ startImmediately: true },
				);
				yield* Ref.update(model, (current_after_start) => {
					if (current_after_start.active?.id !== id) {
						return current_after_start;
					}
					return {
						...current_after_start,
						active: {
							...current_after_start.active,
							fiber,
						},
					};
				});
			});
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

		return {
			start: (input) => {
				return start_now(input);
			},
			stop: stop_now,
			take: take_now,
			snapshot: Ref.get(model).pipe(
				Effect.map((current) => {
					return snapshot_from_model(current);
				}),
			),
			shutdown: Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active) {
					yield* cancel_active(current.active);
				}
				yield* clear_work();
			}).pipe(
				Effect.catch(() => {
					return Effect.void;
				}),
			),
		};
	});
}

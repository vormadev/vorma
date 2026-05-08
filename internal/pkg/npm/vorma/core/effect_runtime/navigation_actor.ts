import { Data, Deferred, Effect, Fiber, Queue, Ref } from "effect";
import type { ClientLoaderPrestart } from "./route_preparer.ts";

export type NavigationSource = "navigate" | "redirect" | "popstate";

export type NavigationAttempt = {
	id: number;
	href: string;
	key: string;
	replace: boolean;
	source: NavigationSource;
	redirectCount: number;
	signal: AbortSignal;
	client_loader_prestarts: ClientLoaderPrestart[];
	state?: unknown;
	scrollToTop?: boolean;
	skipWorkIndicator?: boolean;
};

export type LoadedRoute = {
	href: string;
	value: unknown;
};

export type NavigationResult = {
	didNavigate: boolean;
	href: string | null;
	redirectCount: number;
};

export type NavigationActorSnapshot = {
	active: {
		id: number;
		href: string;
		key: string;
		waiterCount: number;
		redirectCount: number;
	} | null;
	nextID: number;
};

export class NavigationLoadFailed extends Data.TaggedError(
	"NavigationLoadFailed",
)<{
	readonly error: unknown;
}> {}

export class NavigationRedirect extends Data.TaggedError("NavigationRedirect")<{
	readonly href: string;
	readonly hard?: boolean;
}> {}

export type NavigationActorOptions = {
	load: (
		attempt: NavigationAttempt,
	) => Effect.Effect<LoadedRoute, NavigationLoadFailed | NavigationRedirect>;
	publish: (
		loaded: LoadedRoute,
		attempt: NavigationAttempt,
	) => Effect.Effect<void, NavigationLoadFailed>;
	routeKey?: (href: string) => string;
	isExternal?: (href: string) => boolean;
	maxRedirects?: number;
	prestart?: (
		attempt: NavigationAttempt,
	) => Effect.Effect<ClientLoaderPrestart[]>;
};

export type NavigationActor = {
	navigate: (
		href: string,
		options?: {
			replace?: boolean;
			source?: NavigationSource;
			state?: unknown;
			scrollToTop?: boolean;
			skipWorkIndicator?: boolean;
		},
	) => Effect.Effect<NavigationResult>;
	abort_active_signal: Effect.Effect<void>;
	idle: Effect.Effect<void>;
	snapshot: Effect.Effect<NavigationActorSnapshot>;
	shutdown: Effect.Effect<void>;
};

type Waiter = {
	readonly deferred: Deferred.Deferred<NavigationResult>;
};

type Active = {
	readonly id: number;
	readonly href: string;
	readonly key: string;
	readonly replace: boolean;
	readonly source: NavigationSource;
	readonly redirectCount: number;
	readonly state: unknown;
	readonly scrollToTop: boolean | undefined;
	readonly skipWorkIndicator: boolean | undefined;
	readonly waiters: ReadonlyArray<Waiter>;
	readonly fiber: Fiber.RuntimeFiber<void, never> | null;
	readonly controller: AbortController;
	readonly client_loader_prestarts: ClientLoaderPrestart[];
};

type Model = {
	readonly nextID: number;
	readonly active: Active | null;
	readonly idle_waiters: ReadonlyArray<Deferred.Deferred<void>>;
};

type StartInput = {
	readonly href: string;
	readonly replace: boolean;
	readonly source: NavigationSource;
	readonly redirectCount: number;
	readonly state: unknown;
	readonly scrollToTop: boolean | undefined;
	readonly skipWorkIndicator: boolean | undefined;
	readonly waiters: ReadonlyArray<Waiter>;
};

type Command =
	| {
			readonly _tag: "Navigate";
			readonly href: string;
			readonly replace: boolean;
			readonly source: NavigationSource;
			readonly state: unknown;
			readonly scrollToTop: boolean | undefined;
			readonly skipWorkIndicator: boolean | undefined;
			readonly waiter: Waiter;
	  }
	| {
			readonly _tag: "Loaded";
			readonly id: number;
			readonly loaded: LoadedRoute;
			readonly attempt: NavigationAttempt;
	  }
	| {
			readonly _tag: "Published";
			readonly id: number;
			readonly href: string;
			readonly redirectCount: number;
			readonly result: "ok" | "failed";
	  }
	| {
			readonly _tag: "Failed";
			readonly id: number;
	  }
	| {
			readonly _tag: "Redirected";
			readonly id: number;
			readonly href: string;
			readonly hard: boolean;
	  }
	| {
			readonly _tag: "Shutdown";
	  };

const DEFAULT_MAX_REDIRECTS = 10;

export function make_navigation_actor(
	options: NavigationActorOptions,
): Effect.Effect<NavigationActor, never> {
	return Effect.gen(function* () {
		const queue = yield* Queue.unbounded<Command>();
		const model = yield* Ref.make<Model>({
			nextID: 0,
			active: null,
			idle_waiters: [],
		});
		const route_key =
			options.routeKey ??
			((href: string): string => {
				try {
					const url = new URL(href, "http://localhost");
					url.hash = "";
					return url.href;
				} catch {
					return href;
				}
			});
		const is_external =
			options.isExternal ??
			((_href: string): boolean => {
				return false;
			});
		const max_redirects = options.maxRedirects ?? DEFAULT_MAX_REDIRECTS;
		const resolve_waiters = (
			waiters: ReadonlyArray<Waiter>,
			result: NavigationResult,
		): Effect.Effect<void> => {
			return Effect.forEach(
				waiters,
				(waiter) => {
					return Deferred.succeed(waiter.deferred, result);
				},
				{ discard: true },
			);
		};

		const resolve_idle_waiters = (
			waiters: ReadonlyArray<Deferred.Deferred<void>>,
		): Effect.Effect<void> => {
			return Effect.forEach(
				waiters,
				(waiter) => {
					return Deferred.succeed(waiter, undefined);
				},
				{ discard: true },
			);
		};

		const interrupt_active = (active: Active): Effect.Effect<void> => {
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
					yield* Fiber.interruptFork(active.fiber);
				}
			});
		};

		const same_navigation_intent = (
			active: Active,
			command: Extract<Command, { _tag: "Navigate" }>,
		): boolean => {
			return (
				active.href === command.href &&
				active.replace === command.replace &&
				active.source === command.source &&
				active.state === command.state &&
				active.scrollToTop === command.scrollToTop &&
				active.skipWorkIndicator === command.skipWorkIndicator
			);
		};

		const abort_active_signal = Effect.gen(function* () {
			const current = yield* Ref.get(model);
			if (!current.active) {
				return;
			}
			yield* Effect.sync(() => {
				current.active!.controller.abort();
			});
			yield* Effect.forEach(
				current.active.client_loader_prestarts,
				(prestart) => {
					return prestart.abort;
				},
				{ discard: true },
			);
		});

		const attempt_program = (
			attempt: NavigationAttempt,
		): Effect.Effect<void, never> => {
			return options.load(attempt).pipe(
				Effect.flatMap((loaded) => {
					return Queue.offer(queue, {
						_tag: "Loaded",
						id: attempt.id,
						loaded,
						attempt,
					});
				}),
				Effect.catchTag("NavigationRedirect", (redirect) => {
					return Queue.offer(queue, {
						_tag: "Redirected",
						id: attempt.id,
						href: redirect.href,
						hard: redirect.hard === true,
					});
				}),
				Effect.catchAll(() => {
					return Queue.offer(queue, {
						_tag: "Failed",
						id: attempt.id,
					});
				}),
			);
		};

		const start = (input: StartInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const id = current.nextID + 1;
				const key = route_key(input.href);
				const controller = new AbortController();
				const base_attempt: NavigationAttempt = {
					id,
					href: input.href,
					key,
					replace: input.replace,
					source: input.source,
					redirectCount: input.redirectCount,
					signal: controller.signal,
					client_loader_prestarts: [],
					state: input.state,
					scrollToTop: input.scrollToTop,
					skipWorkIndicator: input.skipWorkIndicator,
				};
				const client_loader_prestarts = options.prestart
					? yield* options.prestart(base_attempt)
					: [];
				const attempt: NavigationAttempt = {
					...base_attempt,
					client_loader_prestarts,
				};
				yield* Ref.set(model, {
					nextID: id,
					idle_waiters: current.idle_waiters,
					active: {
						id,
						href: input.href,
						key,
						replace: input.replace,
						source: input.source,
						redirectCount: input.redirectCount,
						state: input.state,
						scrollToTop: input.scrollToTop,
						skipWorkIndicator: input.skipWorkIndicator,
						waiters: input.waiters,
						fiber: null,
						controller,
						client_loader_prestarts,
					},
				});
				const fiber = yield* Effect.forkDaemon(
					attempt_program(attempt),
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

		const on_navigate = (
			command: Extract<Command, { _tag: "Navigate" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const key = route_key(command.href);
				if (is_external(command.href)) {
					yield* Deferred.succeed(command.waiter.deferred, {
						didNavigate: false,
						href: null,
						redirectCount: 0,
					});
					return;
				}
				if (current.active?.key === key) {
					if (!same_navigation_intent(current.active, command)) {
						yield* resolve_waiters(current.active.waiters, {
							didNavigate: false,
							href: null,
							redirectCount: current.active.redirectCount,
						});
						yield* Ref.set(model, {
							...current,
							active: {
								...current.active,
								href: command.href,
								replace: command.replace,
								source: command.source,
								state: command.state,
								scrollToTop: command.scrollToTop,
								skipWorkIndicator: command.skipWorkIndicator,
								waiters: [command.waiter],
							},
						});
						return;
					}
					yield* Ref.set(model, {
						...current,
						active: {
							...current.active,
							waiters: [
								...current.active.waiters,
								command.waiter,
							],
						},
					});
					return;
				}
				if (current.active) {
					yield* interrupt_active(current.active);
					yield* resolve_waiters(current.active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: current.active.redirectCount,
					});
				}
				yield* start({
					href: command.href,
					replace: command.replace,
					source: command.source,
					redirectCount: 0,
					state: command.state,
					scrollToTop: command.scrollToTop,
					skipWorkIndicator: command.skipWorkIndicator,
					waiters: [command.waiter],
				});
			});
		};

		const on_loaded = (
			command: Extract<Command, { _tag: "Loaded" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== command.id) {
					return;
				}
				const active = current.active;
				const loaded: LoadedRoute = {
					...command.loaded,
					href: active.href,
				};
				const attempt: NavigationAttempt = {
					...command.attempt,
					href: active.href,
					replace: active.replace,
					source: active.source,
					redirectCount: active.redirectCount,
					state: active.state,
					scrollToTop: active.scrollToTop,
					skipWorkIndicator: active.skipWorkIndicator,
				};
				const publish_fiber = yield* Effect.fork(
					options.publish(loaded, attempt).pipe(
						Effect.either,
						Effect.flatMap((publish_result) => {
							return Queue.offer(queue, {
								_tag: "Published" as const,
								id: command.id,
								href: loaded.href,
								redirectCount: active.redirectCount,
								result:
									publish_result._tag === "Right"
										? "ok"
										: "failed",
							});
						}),
						Effect.asVoid,
					),
				);
				yield* Ref.set(model, {
					...current,
					active: {
						...active,
						fiber: publish_fiber,
					},
				});
			});
		};

		const on_published = (
			command: Extract<Command, { _tag: "Published" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== command.id) {
					return;
				}
				const active = current.active;
				yield* Ref.set(model, {
					...current,
					active: null,
					idle_waiters: [],
				});
				if (command.result === "failed") {
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: command.redirectCount,
					});
					yield* resolve_idle_waiters(current.idle_waiters);
					return;
				}
				yield* resolve_waiters(active.waiters, {
					didNavigate: true,
					href: command.href,
					redirectCount: command.redirectCount,
				});
				yield* resolve_idle_waiters(current.idle_waiters);
			});
		};

		const on_failed = (
			command: Extract<Command, { _tag: "Failed" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== command.id) {
					return;
				}
				const active = current.active;
				yield* Effect.forEach(
					active.client_loader_prestarts,
					(prestart) => {
						return prestart.abort;
					},
					{ discard: true },
				);
				yield* resolve_waiters(active.waiters, {
					didNavigate: false,
					href: null,
					redirectCount: active.redirectCount,
				});
				yield* resolve_idle_waiters(current.idle_waiters);
			});
		};

		const on_redirected = (
			command: Extract<Command, { _tag: "Redirected" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== command.id) {
					return;
				}
				const active = current.active;
				yield* Ref.set(model, {
					...current,
					active: null,
					idle_waiters: [],
				});
				yield* Effect.forEach(
					active.client_loader_prestarts,
					(prestart) => {
						return prestart.abort;
					},
					{ discard: true },
				);
				if (
					command.hard ||
					is_external(command.href) ||
					active.redirectCount >= max_redirects
				) {
					yield* Ref.set(model, {
						...current,
						active: null,
						idle_waiters: [],
					});
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: active.redirectCount,
					});
					yield* resolve_idle_waiters(current.idle_waiters);
					return;
				}
				yield* start({
					href: command.href,
					replace: active.replace,
					source: "redirect",
					redirectCount: active.redirectCount + 1,
					state: active.state,
					scrollToTop: active.scrollToTop,
					skipWorkIndicator: active.skipWorkIndicator,
					waiters: active.waiters,
				});
			});
		};

		const command_program = (command: Command): Effect.Effect<void> => {
			switch (command._tag) {
				case "Navigate": {
					return on_navigate(command);
				}
				case "Loaded": {
					return on_loaded(command);
				}
				case "Published": {
					return on_published(command);
				}
				case "Failed": {
					return on_failed(command);
				}
				case "Redirected": {
					return on_redirected(command);
				}
				case "Shutdown": {
					return Effect.gen(function* () {
						const current = yield* Ref.get(model);
						if (current.active) {
							yield* interrupt_active(current.active);
							yield* resolve_waiters(current.active.waiters, {
								didNavigate: false,
								href: null,
								redirectCount: current.active.redirectCount,
							});
						}
						yield* resolve_idle_waiters(current.idle_waiters);
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
						yield* interrupt_active(current.active);
						yield* resolve_waiters(current.active.waiters, {
							didNavigate: false,
							href: null,
							redirectCount: current.active.redirectCount,
						});
					}
					yield* resolve_idle_waiters(current.idle_waiters);
				}),
			),
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.forkDaemon(actor);

		return {
			navigate: (href, nav_options) => {
				return Effect.gen(function* () {
					const waiter = yield* Deferred.make<NavigationResult>();
					yield* on_navigate({
						_tag: "Navigate",
						href,
						replace: nav_options?.replace === true,
						source: nav_options?.source ?? "navigate",
						state: nav_options?.state,
						scrollToTop: nav_options?.scrollToTop,
						skipWorkIndicator: nav_options?.skipWorkIndicator,
						waiter: { deferred: waiter },
					});
					return yield* Deferred.await(waiter);
				});
			},
			abort_active_signal,
			idle: Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (!current.active) {
					return;
				}
				const waiter = yield* Deferred.make<void>();
				yield* Ref.set(model, {
					...current,
					idle_waiters: [...current.idle_waiters, waiter],
				});
				yield* Deferred.await(waiter);
			}),
			snapshot: Ref.get(model).pipe(
				Effect.map((current): NavigationActorSnapshot => {
					return {
						active: current.active
							? {
									id: current.active.id,
									href: current.active.href,
									key: current.active.key,
									waiterCount: current.active.waiters.length,
									redirectCount: current.active.redirectCount,
								}
							: null,
						nextID: current.nextID,
					};
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

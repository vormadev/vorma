import {
	Data,
	Deferred,
	Effect,
	Result as EffectResult,
	Fiber,
	Queue,
	Ref,
} from "effect";

export type NavigationSource = "navigate" | "redirect" | "popstate";

export type NavigationAttempt = {
	id: number;
	href: string;
	key: string;
	replace: boolean;
	source: NavigationSource;
	redirectCount: number;
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
};

export type NavigationActor = {
	navigate: (
		href: string,
		options?: {
			replace?: boolean;
			source?: NavigationSource;
		},
	) => Effect.Effect<NavigationResult>;
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
	readonly waiters: ReadonlyArray<Waiter>;
	readonly fiber: Fiber.Fiber<void, never>;
};

type Model = {
	readonly nextID: number;
	readonly active: Active | null;
};

type StartInput = {
	readonly href: string;
	readonly replace: boolean;
	readonly source: NavigationSource;
	readonly redirectCount: number;
	readonly waiters: ReadonlyArray<Waiter>;
};

type Command =
	| {
			readonly _tag: "Navigate";
			readonly href: string;
			readonly replace: boolean;
			readonly source: NavigationSource;
			readonly waiter: Waiter;
	  }
	| {
			readonly _tag: "Loaded";
			readonly id: number;
			readonly loaded: LoadedRoute;
			readonly attempt: NavigationAttempt;
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

		const interrupt_active = (active: Active): Effect.Effect<void> => {
			return Effect.forkDetach(Fiber.interrupt(active.fiber), {
				startImmediately: true,
			}).pipe(Effect.asVoid);
		};

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
				Effect.catch(() => {
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
				const attempt: NavigationAttempt = {
					id,
					href: input.href,
					key,
					replace: input.replace,
					source: input.source,
					redirectCount: input.redirectCount,
				};
				const fiber = yield* Effect.forkChild(attempt_program(attempt));
				yield* Ref.set(model, {
					nextID: id,
					active: {
						id,
						href: input.href,
						key,
						replace: input.replace,
						source: input.source,
						redirectCount: input.redirectCount,
						waiters: input.waiters,
						fiber,
					},
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
				const publish_result = yield* Effect.result(
					options.publish(command.loaded, command.attempt),
				);
				yield* Ref.set(model, { ...current, active: null });
				if (EffectResult.isFailure(publish_result)) {
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: active.redirectCount,
					});
					return;
				}
				yield* resolve_waiters(active.waiters, {
					didNavigate: true,
					href: command.loaded.href,
					redirectCount: active.redirectCount,
				});
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
				yield* Ref.set(model, { ...current, active: null });
				yield* resolve_waiters(active.waiters, {
					didNavigate: false,
					href: null,
					redirectCount: active.redirectCount,
				});
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
				yield* Ref.set(model, { ...current, active: null });
				if (
					command.hard ||
					is_external(command.href) ||
					active.redirectCount >= max_redirects
				) {
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: active.redirectCount,
					});
					return;
				}
				yield* start({
					href: command.href,
					replace: true,
					source: "redirect",
					redirectCount: active.redirectCount + 1,
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
				}),
			),
			Effect.catch(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.forkChild(actor);

		return {
			navigate: (href, nav_options) => {
				return Effect.gen(function* () {
					const waiter = yield* Deferred.make<NavigationResult>();
					yield* Queue.offer(queue, {
						_tag: "Navigate",
						href,
						replace: nav_options?.replace === true,
						source: nav_options?.source ?? "navigate",
						waiter: { deferred: waiter },
					});
					return yield* Deferred.await(waiter);
				});
			},
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
				Effect.catch(() => {
					return Effect.void;
				}),
			),
		};
	});
}

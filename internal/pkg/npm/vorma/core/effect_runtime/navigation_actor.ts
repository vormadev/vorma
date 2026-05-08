import {
	Data,
	Deferred,
	Effect,
	Result as EffectResult,
	Fiber,
	Ref,
} from "effect";
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
	readonly fiber: Fiber.Fiber<void, never> | null;
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

type NavigateInput = {
	readonly href: string;
	readonly replace: boolean;
	readonly source: NavigationSource;
	readonly state: unknown;
	readonly scrollToTop: boolean | undefined;
	readonly skipWorkIndicator: boolean | undefined;
	readonly waiter: Waiter;
};

type LoadedInput = {
	readonly id: number;
	readonly loaded: LoadedRoute;
	readonly attempt: NavigationAttempt;
};

type PublishedInput = {
	readonly id: number;
	readonly href: string;
	readonly redirectCount: number;
	readonly result: "ok" | "failed";
};

type RedirectedInput = {
	readonly id: number;
	readonly href: string;
	readonly hard: boolean;
};

const DEFAULT_MAX_REDIRECTS = 10;

export function make_navigation_actor(
	options: NavigationActorOptions,
): Effect.Effect<NavigationActor, never> {
	return Effect.gen(function* () {
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
					yield* Effect.forkDetach(Fiber.interrupt(active.fiber), {
						startImmediately: true,
					}).pipe(Effect.asVoid);
				}
			});
		};

		const same_navigation_intent = (
			active: Active,
			input: NavigateInput,
		): boolean => {
			return (
				active.href === input.href &&
				active.replace === input.replace &&
				active.source === input.source &&
				active.state === input.state &&
				active.scrollToTop === input.scrollToTop &&
				active.skipWorkIndicator === input.skipWorkIndicator
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
					return on_loaded({
						id: attempt.id,
						loaded,
						attempt,
					});
				}),
				Effect.catchTag("NavigationRedirect", (redirect) => {
					return on_redirected({
						id: attempt.id,
						href: redirect.href,
						hard: redirect.hard === true,
					});
				}),
				Effect.catch(() => {
					return on_failed(attempt.id);
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
				const fiber = yield* Effect.forkDetach(
					attempt_program(attempt),
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

		const on_navigate = (input: NavigateInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const key = route_key(input.href);
				if (is_external(input.href)) {
					yield* Deferred.succeed(input.waiter.deferred, {
						didNavigate: false,
						href: null,
						redirectCount: 0,
					});
					return;
				}
				if (current.active?.key === key) {
					if (!same_navigation_intent(current.active, input)) {
						yield* resolve_waiters(current.active.waiters, {
							didNavigate: false,
							href: null,
							redirectCount: current.active.redirectCount,
						});
						yield* Ref.set(model, {
							...current,
							active: {
								...current.active,
								href: input.href,
								replace: input.replace,
								source: input.source,
								state: input.state,
								scrollToTop: input.scrollToTop,
								skipWorkIndicator: input.skipWorkIndicator,
								waiters: [input.waiter],
							},
						});
						return;
					}
					yield* Ref.set(model, {
						...current,
						active: {
							...current.active,
							waiters: [...current.active.waiters, input.waiter],
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
					href: input.href,
					replace: input.replace,
					source: input.source,
					redirectCount: 0,
					state: input.state,
					scrollToTop: input.scrollToTop,
					skipWorkIndicator: input.skipWorkIndicator,
					waiters: [input.waiter],
				});
			});
		};

		const on_loaded = (input: LoadedInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== input.id) {
					return;
				}
				const active = current.active;
				const loaded: LoadedRoute = {
					...input.loaded,
					href: active.href,
				};
				const attempt: NavigationAttempt = {
					...input.attempt,
					href: active.href,
					replace: active.replace,
					source: active.source,
					redirectCount: active.redirectCount,
					state: active.state,
					scrollToTop: active.scrollToTop,
					skipWorkIndicator: active.skipWorkIndicator,
				};
				const publish_result = yield* Effect.result(
					options.publish(loaded, attempt),
				);
				yield* on_published({
					id: input.id,
					href: loaded.href,
					redirectCount: active.redirectCount,
					result: EffectResult.isSuccess(publish_result)
						? "ok"
						: "failed",
				});
			});
		};

		const on_published = (input: PublishedInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== input.id) {
					return;
				}
				const active = current.active;
				yield* Ref.set(model, {
					...current,
					active: null,
					idle_waiters: [],
				});
				if (input.result === "failed") {
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: input.redirectCount,
					});
					yield* resolve_idle_waiters(current.idle_waiters);
					return;
				}
				yield* resolve_waiters(active.waiters, {
					didNavigate: true,
					href: input.href,
					redirectCount: input.redirectCount,
				});
				yield* resolve_idle_waiters(current.idle_waiters);
			});
		};

		const on_failed = (id: number): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== id) {
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
				yield* resolve_waiters(active.waiters, {
					didNavigate: false,
					href: null,
					redirectCount: active.redirectCount,
				});
				yield* resolve_idle_waiters(current.idle_waiters);
			});
		};

		const on_redirected = (input: RedirectedInput): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.id !== input.id) {
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
					input.hard ||
					is_external(input.href) ||
					active.redirectCount >= max_redirects
				) {
					yield* resolve_waiters(active.waiters, {
						didNavigate: false,
						href: null,
						redirectCount: active.redirectCount,
					});
					yield* resolve_idle_waiters(current.idle_waiters);
					return;
				}
				yield* start({
					href: input.href,
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

		return {
			navigate: (href, nav_options) => {
				return Effect.gen(function* () {
					const waiter = yield* Deferred.make<NavigationResult>();
					yield* on_navigate({
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
			shutdown: Effect.gen(function* () {
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
			}).pipe(
				Effect.catch(() => {
					return Effect.void;
				}),
			),
		};
	});
}

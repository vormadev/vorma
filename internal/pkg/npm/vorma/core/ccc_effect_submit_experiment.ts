import { Data, Deferred, Effect, Fiber, Queue, Ref } from "effect";
import {
	VERCEL_X_DEPLOYMENT_ID,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
} from "./constants.ts";
import type { APIRouteKind, RevalidationResult } from "./types.ts";

export type SubmitResult<T = unknown> =
	| {
			success: true;
			data: T;
			response: Response;
			revalidation: Effect.Effect<RevalidationResult>;
	  }
	| {
			success: false;
			error: string;
			response?: Response;
			revalidation: Effect.Effect<RevalidationResult>;
	  };

export type SubmitRequest = {
	href: string;
	init?: RequestInit;
	options?: {
		apiRouteKind?: APIRouteKind;
		dedupeKey?: string;
		revalidate?: boolean;
		skipworkIndicator?: boolean;
	};
};

export type SubmitDispatch = {
	id: number;
	key: string;
	url: URL;
	method: string;
	apiRouteKind: APIRouteKind;
	init: RequestInit;
};

export type SubmitSnapshot = {
	active: Array<{
		id: number;
		key: string;
		method: string;
		href: string;
		apiRouteKind: APIRouteKind;
		skipworkIndicator: boolean;
	}>;
	nextID: number;
};

export type SubmitManager = {
	submit: <T = unknown>(
		request: SubmitRequest,
	) => Effect.Effect<SubmitResult<T>>;
	snapshot: Effect.Effect<SubmitSnapshot>;
	shutdown: Effect.Effect<void>;
};

export class SubmitDispatchFailed extends Data.TaggedError(
	"SubmitDispatchFailed",
)<{
	readonly error: unknown;
}> {}

export class SubmitAborted extends Data.TaggedError("SubmitAborted")<{}> {}

export type SubmitManagerOptions = {
	dispatch: (
		request: SubmitDispatch,
	) => Effect.Effect<Response, SubmitDispatchFailed | SubmitAborted>;
	revalidate?: (reason: "apiRequest") => Effect.Effect<RevalidationResult>;
	redirect?: (href: string, kind: "client" | "hard") => Effect.Effect<void>;
	baseHref?: string;
	deploymentID?: string;
	isSameOrigin?: (url: URL) => boolean;
};

type Waiter = {
	readonly deferred: Deferred.Deferred<SubmitResult>;
};

type ActiveSubmission = {
	readonly id: number;
	readonly key: string;
	readonly href: string;
	readonly method: string;
	readonly apiRouteKind: APIRouteKind;
	readonly skipworkIndicator: boolean;
	readonly waiter: Waiter;
	readonly fiber: Fiber.RuntimeFiber<void, never>;
};

type Model = {
	readonly nextID: number;
	readonly active: ReadonlyMap<string, ActiveSubmission>;
};

type Command =
	| {
			readonly _tag: "Submit";
			readonly request: SubmitRequest;
			readonly waiter: Waiter;
	  }
	| {
			readonly _tag: "Completed";
			readonly id: number;
			readonly key: string;
			readonly result: SubmitResult;
	  }
	| {
			readonly _tag: "Shutdown";
	  };

const REVALIDATION_OK: RevalidationResult = { ok: true };
const REVALIDATION_EXHAUSTED: RevalidationResult = {
	ok: false,
	reason: "max_retries_exhausted",
};

export function make_submit_manager(
	options: SubmitManagerOptions,
): Effect.Effect<SubmitManager, never> {
	return Effect.gen(function* () {
		const queue = yield* Queue.unbounded<Command>();
		const model = yield* Ref.make<Model>({
			nextID: 0,
			active: new Map(),
		});
		const base_href = options.baseHref ?? "http://localhost";
		const is_same_origin =
			options.isSameOrigin ??
			((url: URL): boolean => {
				return url.origin === new URL(base_href).origin;
			});
		const redirect =
			options.redirect ??
			((_href: string, _kind: "client" | "hard") => {
				return Effect.void;
			});
		const revalidate =
			options.revalidate ??
			((_reason: "apiRequest") => {
				return Effect.succeed(REVALIDATION_OK);
			});

		const revalidation_now = (
			should_revalidate: boolean,
		): Effect.Effect<Effect.Effect<RevalidationResult>> => {
			if (!should_revalidate) {
				return Effect.succeed(Effect.succeed(REVALIDATION_OK));
			}
			return Effect.gen(function* () {
				const deferred_result =
					yield* Deferred.make<RevalidationResult>();
				yield* Effect.forkDaemon(
					revalidate("apiRequest").pipe(
						Effect.flatMap((result) => {
							return Deferred.succeed(deferred_result, result);
						}),
						Effect.catchAll(() => {
							return Deferred.succeed(
								deferred_result,
								REVALIDATION_EXHAUSTED,
							);
						}),
					),
				);
				return Deferred.await(deferred_result);
			});
		};

		const resolve_waiter = (
			waiter: Waiter,
			result: SubmitResult,
		): Effect.Effect<void> => {
			return Deferred.succeed(waiter.deferred, result);
		};

		const interrupt_submission = (
			submission: ActiveSubmission,
		): Effect.Effect<void> => {
			return Fiber.interruptFork(submission.fiber).pipe(
				Effect.andThen(
					resolve_waiter(submission.waiter, {
						success: false,
						error: "Aborted",
						revalidation: Effect.succeed(REVALIDATION_OK),
					}),
				),
			);
		};

		const http_href = (href: string): boolean => {
			try {
				const protocol = new URL(href, base_href).protocol;
				return protocol === "http:" || protocol === "https:";
			} catch {
				return false;
			}
		};

		const redirect_from_response = (
			response: Response,
			base: URL,
		): { href: string; hard: boolean } | null => {
			const soft = response.headers.get(X_CLIENT_REDIRECT);
			if (soft) {
				return { href: new URL(soft, base).href, hard: false };
			}
			if (
				response.redirected &&
				response.url &&
				response.url !== base.href
			) {
				return { href: new URL(response.url, base).href, hard: false };
			}
			return null;
		};

		const response_data = (
			response: Response,
		): Effect.Effect<unknown, SubmitDispatchFailed> => {
			if (response.status === 204) {
				return Effect.succeed(undefined);
			}
			const content_type = response.headers.get("Content-Type");
			if (content_type?.toLowerCase().includes("json")) {
				return Effect.tryPromise({
					try: () => {
						return response.json();
					},
					catch: (error) => {
						return new SubmitDispatchFailed({ error });
					},
				});
			}
			return Effect.tryPromise({
				try: async () => {
					const text = await response.text();
					return text.length > 0 ? text : undefined;
				},
				catch: (error) => {
					return new SubmitDispatchFailed({ error });
				},
			});
		};

		const prepare_dispatch = (
			id: number,
			key: string,
			request: SubmitRequest,
		): Effect.Effect<
			| {
					_tag: "dispatch";
					dispatch: SubmitDispatch;
					shouldRevalidate: boolean;
			  }
			| { _tag: "immediate"; result: SubmitResult },
			never
		> => {
			return Effect.sync(() => {
				const url = new URL(request.href, base_href);
				if (!is_same_origin(url)) {
					return {
						_tag: "immediate" as const,
						result: {
							success: false,
							error: `submit only supports same-origin targets. Received: "${url.href}".`,
							revalidation: Effect.succeed(REVALIDATION_OK),
						},
					};
				}
				const method = request.init?.method
					? request.init.method.toUpperCase().trim()
					: "GET";
				const api_route_kind =
					request.options?.apiRouteKind ??
					(method === "GET" || method === "HEAD"
						? "query"
						: "mutation");
				const should_revalidate =
					request.options?.revalidate ??
					api_route_kind === "mutation";
				const headers = new Headers();
				if (options.deploymentID) {
					headers.set(VERCEL_X_DEPLOYMENT_ID, options.deploymentID);
				}
				new Headers(request.init?.headers ?? undefined).forEach(
					(value, name) => {
						headers.set(name, value);
					},
				);
				headers.set(X_ACCEPTS_CLIENT_REDIRECT, "1");

				const is_get = method === "GET" || method === "HEAD";
				const body = request.init?.body;
				const should_json =
					!is_get &&
					!!body &&
					typeof body === "object" &&
					!(body instanceof ReadableStream) &&
					!(body instanceof FormData) &&
					!(body instanceof URLSearchParams) &&
					!(body instanceof Blob) &&
					!(body instanceof ArrayBuffer) &&
					!ArrayBuffer.isView(body);
				const init: RequestInit = {
					...request.init,
					method,
					headers,
				};
				if (is_get) {
					delete init.body;
				} else if (should_json) {
					init.body = JSON.stringify(body);
					if (!headers.has("Content-Type")) {
						headers.set("Content-Type", "application/json");
					}
				}

				return {
					_tag: "dispatch" as const,
					dispatch: {
						id,
						key,
						url,
						method,
						apiRouteKind: api_route_kind,
						init,
					},
					shouldRevalidate: should_revalidate,
				};
			});
		};

		const result_from_response = (
			dispatch: SubmitDispatch,
			response: Response,
			should_revalidate: boolean,
		): Effect.Effect<SubmitResult, never> => {
			return Effect.catchAll(
				Effect.gen(function* () {
					const redirect_info = redirect_from_response(
						response,
						dispatch.url,
					);
					if (redirect_info) {
						if (!http_href(redirect_info.href)) {
							return {
								success: false as const,
								error: `Redirect target must use an HTTP(S) scheme. Received: "${redirect_info.href}".`,
								response,
								revalidation: Effect.succeed(REVALIDATION_OK),
							};
						}
						yield* redirect(
							redirect_info.href,
							redirect_info.hard ||
								!is_same_origin(new URL(redirect_info.href))
								? "hard"
								: "client",
						);
						return {
							success: true as const,
							data: undefined,
							response,
							revalidation: Effect.succeed(REVALIDATION_OK),
						};
					}
					if (!response.ok) {
						const revalidation_effect =
							yield* revalidation_now(should_revalidate);
						return {
							success: false as const,
							error: response.statusText,
							response,
							revalidation: revalidation_effect,
						};
					}
					const data = yield* response_data(response);
					const revalidation_effect =
						yield* revalidation_now(should_revalidate);
					return {
						success: true as const,
						data,
						response,
						revalidation: revalidation_effect,
					};
				}),
				(error) => {
					return Effect.succeed({
						success: false,
						error:
							error instanceof SubmitDispatchFailed
								? String(error.error)
								: String(error),
						response,
						revalidation: Effect.succeed(REVALIDATION_OK),
					});
				},
			);
		};

		const run_submission = (
			dispatch: SubmitDispatch,
			should_revalidate: boolean,
		): Effect.Effect<void, never> => {
			const normalize_defect = (error: unknown): Effect.Effect<void> => {
				return Queue.offer(queue, {
					_tag: "Completed",
					id: dispatch.id,
					key: dispatch.key,
					result: {
						success: false,
						error: String(error),
						revalidation: Effect.succeed(REVALIDATION_OK),
					},
				});
			};
			return options.dispatch(dispatch).pipe(
				Effect.flatMap((response) => {
					return result_from_response(
						dispatch,
						response,
						should_revalidate,
					);
				}),
				Effect.flatMap((result) => {
					return Queue.offer(queue, {
						_tag: "Completed",
						id: dispatch.id,
						key: dispatch.key,
						result,
					});
				}),
				Effect.catchAll((error) => {
					if (error instanceof SubmitAborted) {
						return Queue.offer(queue, {
							_tag: "Completed",
							id: dispatch.id,
							key: dispatch.key,
							result: {
								success: false,
								error: "Aborted",
								revalidation: Effect.succeed(REVALIDATION_OK),
							},
						});
					}
					return Effect.gen(function* () {
						const revalidation_effect =
							yield* revalidation_now(should_revalidate);
						yield* Queue.offer(queue, {
							_tag: "Completed",
							id: dispatch.id,
							key: dispatch.key,
							result: {
								success: false,
								error:
									error instanceof SubmitDispatchFailed
										? String(error.error)
										: String(error),
								revalidation: revalidation_effect,
							},
						});
					});
				}),
				Effect.catchAllDefect(normalize_defect),
			);
		};

		const on_submit = (
			command: Extract<Command, { _tag: "Submit" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const id = current.nextID + 1;
				const key =
					command.request.options?.dedupeKey ?? `submit:${id}`;
				const prepared = yield* prepare_dispatch(
					id,
					key,
					command.request,
				);
				if (prepared._tag === "immediate") {
					yield* resolve_waiter(command.waiter, prepared.result);
					yield* Ref.set(model, { ...current, nextID: id });
					return;
				}

				const next_active = new Map(current.active);
				const previous = next_active.get(key);
				if (previous) {
					next_active.delete(key);
					yield* interrupt_submission(previous);
				}
				const fiber = yield* Effect.fork(
					run_submission(
						prepared.dispatch,
						prepared.shouldRevalidate,
					),
				);
				next_active.set(key, {
					id,
					key,
					href: prepared.dispatch.url.href,
					method: prepared.dispatch.method,
					apiRouteKind: prepared.dispatch.apiRouteKind,
					skipworkIndicator:
						command.request.options?.skipworkIndicator === true,
					waiter: command.waiter,
					fiber,
				});
				yield* Ref.set(model, { nextID: id, active: next_active });
			});
		};

		const on_completed = (
			command: Extract<Command, { _tag: "Completed" }>,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const submission = current.active.get(command.key);
				if (!submission || submission.id !== command.id) {
					return;
				}
				const next_active = new Map(current.active);
				next_active.delete(command.key);
				yield* Ref.set(model, { ...current, active: next_active });
				yield* resolve_waiter(submission.waiter, command.result);
			});
		};

		const command_program = (command: Command): Effect.Effect<void> => {
			switch (command._tag) {
				case "Submit": {
					return on_submit(command);
				}
				case "Completed": {
					return on_completed(command);
				}
				case "Shutdown": {
					return Effect.gen(function* () {
						const current = yield* Ref.get(model);
						for (const submission of current.active.values()) {
							yield* interrupt_submission(submission);
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
					for (const submission of current.active.values()) {
						yield* interrupt_submission(submission);
					}
				}),
			),
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.fork(actor);

		return {
			submit: <T = unknown>(request: SubmitRequest) => {
				return Effect.gen(function* () {
					const deferred_result =
						yield* Deferred.make<SubmitResult>();
					yield* Queue.offer(queue, {
						_tag: "Submit",
						request,
						waiter: { deferred: deferred_result },
					});
					return (yield* Deferred.await(
						deferred_result,
					)) as SubmitResult<T>;
				});
			},
			snapshot: Ref.get(model).pipe(
				Effect.map((current): SubmitSnapshot => {
					return {
						nextID: current.nextID,
						active: Array.from(
							current.active.values(),
							(submission) => {
								return {
									id: submission.id,
									key: submission.key,
									method: submission.method,
									href: submission.href,
									apiRouteKind: submission.apiRouteKind,
									skipworkIndicator:
										submission.skipworkIndicator,
								};
							},
						),
					};
				}),
			),
			shutdown: Effect.gen(function* () {
				const current = yield* Ref.get(model);
				for (const submission of current.active.values()) {
					yield* interrupt_submission(submission);
				}
				yield* Queue.shutdown(queue);
				yield* Fiber.interruptFork(actor_fiber);
			}).pipe(
				Effect.catchAll(() => {
					return Effect.void;
				}),
			),
		};
	});
}

import { Cause, Data, Effect, Runtime } from "effect";
import { DATA_SCRIPT_ID, VORMA_ROOT_EL_ID } from "../constants.ts";
import type { ScrollState } from "./client_contract.ts";

export class InitialPayloadReadFailed extends Data.TaggedError(
	"InitialPayloadReadFailed",
)<{
	readonly reason: string;
}> {}

export class BrowserViewTransitionFailed extends Data.TaggedError(
	"BrowserViewTransitionFailed",
)<{
	readonly error: unknown;
}> {}

export type BrowserViewRuntimeOptions = {
	scroll_to?: (x: number, y: number) => void;
};

export type BrowserViewRuntime = {
	read_initial_payload: Effect.Effect<
		Record<string, unknown>,
		InitialPayloadReadFailed
	>;
	apply_scroll: (scroll: ScrollState | undefined) => Effect.Effect<void>;
	get_root_el: Effect.Effect<HTMLElement>;
	run_view_transition: <A, E>(input: {
		enabled: boolean;
		publish: Effect.Effect<A, E>;
	}) => Effect.Effect<A, E | BrowserViewTransitionFailed>;
};

type ViewTransition = {
	readonly updateCallbackDone?: PromiseLike<unknown>;
	readonly finished?: PromiseLike<unknown>;
};

type ViewTransitionDocument = Document & {
	startViewTransition?: (callback: () => unknown) => ViewTransition;
};

export function make_browser_view_runtime(
	options: BrowserViewRuntimeOptions = {},
): Effect.Effect<BrowserViewRuntime, never> {
	return Effect.succeed({
		read_initial_payload: Effect.sync(() => {
			const script = document.getElementById(DATA_SCRIPT_ID);
			if (!script?.textContent) {
				return Effect.fail(
					new InitialPayloadReadFailed({
						reason: "Missing Vorma data script.",
					}),
				);
			}
			try {
				const parsed = JSON.parse(script.textContent);
				if (
					!parsed ||
					typeof parsed !== "object" ||
					Array.isArray(parsed)
				) {
					return Effect.fail(
						new InitialPayloadReadFailed({
							reason: "Vorma data script must contain an object.",
						}),
					);
				}
				return Effect.succeed(parsed as Record<string, unknown>);
			} catch (error) {
				return Effect.fail(
					new InitialPayloadReadFailed({
						reason:
							error instanceof Error
								? error.message
								: String(error),
					}),
				);
			}
		}).pipe(Effect.flatten),
		apply_scroll: (scroll) => {
			if (!scroll) {
				return Effect.void;
			}
			return Effect.sync(() => {
				if ("hash" in scroll) {
					const raw = scroll.hash.startsWith("#")
						? scroll.hash.slice(1)
						: scroll.hash;
					let id: string;
					try {
						id = decodeURIComponent(raw);
					} catch {
						id = raw;
					}
					document.getElementById(id)?.scrollIntoView();
					return;
				}
				const scroll_to =
					options.scroll_to ??
					((x: number, y: number) => {
						window.scrollTo(x, y);
					});
				scroll_to(scroll.x, scroll.y);
			});
		},
		get_root_el: Effect.sync(() => {
			const existing = document.getElementById(VORMA_ROOT_EL_ID);
			if (existing) {
				return existing;
			}
			const root = document.createElement("div");
			root.id = VORMA_ROOT_EL_ID;
			document.body.insertBefore(root, document.body.firstChild);
			return root;
		}),
		run_view_transition: (input) => {
			if (!input.enabled) {
				return input.publish;
			}
			const start_view_transition = (document as ViewTransitionDocument)
				.startViewTransition;
			if (typeof start_view_transition !== "function") {
				return input.publish;
			}
			return Effect.gen(function* () {
				const runtime = yield* Effect.runtime<never>();
				return yield* Effect.async<
					typeof input.publish extends Effect.Effect<infer A, any>
						? A
						: never,
					| (typeof input.publish extends Effect.Effect<any, infer E>
							? E
							: never)
					| BrowserViewTransitionFailed
				>((resume) => {
					let publish_started = false;
					let resumed = false;
					const resume_once = (
						program: Effect.Effect<
							typeof input.publish extends Effect.Effect<
								infer A,
								any
							>
								? A
								: never,
							| (typeof input.publish extends Effect.Effect<
									any,
									infer E
							  >
									? E
									: never)
							| BrowserViewTransitionFailed
						>,
					): void => {
						if (resumed) {
							return;
						}
						resumed = true;
						resume(program);
					};
					try {
						const transition = start_view_transition.call(
							document,
							() => {
								publish_started = true;
								const exit_promise = Runtime.runPromiseExit(
									runtime,
									input.publish,
								);
								void exit_promise.then(
									(exit) => {
										if (exit._tag === "Success") {
											resume_once(
												Effect.succeed(exit.value),
											);
											return;
										}
										resume_once(
											Effect.failCause(exit.cause),
										);
									},
									(error: unknown) => {
										resume_once(
											Effect.fail(
												new BrowserViewTransitionFailed(
													{ error },
												),
											),
										);
									},
								);
								return exit_promise.then((exit) => {
									if (exit._tag === "Failure") {
										throw Cause.squash(exit.cause);
									}
								});
							},
						);
						const completion =
							transition.updateCallbackDone ??
							transition.finished;
						if (completion) {
							void Promise.resolve(completion).then(
								() => {
									if (!publish_started) {
										resume_once(
											Effect.fail(
												new BrowserViewTransitionFailed(
													{
														error: new Error(
															"View transition did not publish.",
														),
													},
												),
											),
										);
									}
								},
								(error: unknown) => {
									resume_once(
										Effect.fail(
											new BrowserViewTransitionFailed({
												error,
											}),
										),
									);
								},
							);
						}
					} catch (error) {
						resume_once(
							Effect.fail(
								new BrowserViewTransitionFailed({ error }),
							),
						);
					}
				});
			});
		},
	});
}

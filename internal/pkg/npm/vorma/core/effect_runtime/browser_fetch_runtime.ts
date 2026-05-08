import { Data, Effect } from "effect";
import {
	is_abort_error as default_is_abort_error,
	merge_abort_signals,
} from "./abort_signal.ts";

export class BrowserFetchFailed extends Data.TaggedError("BrowserFetchFailed")<{
	readonly error: unknown;
}> {}

export class BrowserFetchAborted extends Data.TaggedError(
	"BrowserFetchAborted",
)<{
	readonly error: unknown;
}> {}

export type BrowserFetchInput = {
	url: URL;
	init?: RequestInit;
	signals?: ReadonlyArray<AbortSignal | null | undefined>;
};

export type BrowserFetchRuntime = {
	fetch: (
		input: BrowserFetchInput,
	) => Effect.Effect<Response, BrowserFetchFailed | BrowserFetchAborted>;
};

export type BrowserFetchRuntimeOptions = {
	fetch?: (url: URL, init: RequestInit) => Promise<Response>;
	is_abort_error?: (error: unknown) => boolean;
};

export function make_browser_fetch_runtime(
	options: BrowserFetchRuntimeOptions = {},
): Effect.Effect<BrowserFetchRuntime, never> {
	return Effect.sync(() => {
		const fetch_impl =
			options.fetch ??
			((url: URL, init: RequestInit): Promise<Response> => {
				return fetch(url, init);
			});
		const is_abort_error = options.is_abort_error ?? default_is_abort_error;

		return {
			fetch: (input) => {
				return Effect.tryPromise({
					try: (signal) => {
						const request_init: RequestInit = {
							...input.init,
							signal: merge_abort_signals([
								signal,
								input.init?.signal,
								...(input.signals ?? []),
							]),
						};
						return fetch_impl(input.url, request_init);
					},
					catch: (error) => {
						if (is_abort_error(error)) {
							return new BrowserFetchAborted({ error });
						}
						return new BrowserFetchFailed({ error });
					},
				});
			},
		};
	});
}

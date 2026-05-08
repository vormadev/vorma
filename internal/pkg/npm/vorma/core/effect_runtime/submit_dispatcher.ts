import { Effect } from "effect";
import {
	SubmitAborted,
	type SubmitDispatch,
	SubmitDispatchFailed,
} from "./submit_manager.ts";

const abort_error_name = "AbortError";

export type SubmitDispatcher = {
	dispatch: (
		request: SubmitDispatch,
	) => Effect.Effect<Response, SubmitDispatchFailed | SubmitAborted>;
};

export type SubmitDispatcherOptions = {
	fetch?: (url: URL, init: RequestInit) => Promise<Response>;
};

export function make_submit_dispatcher(
	options: SubmitDispatcherOptions = {},
): Effect.Effect<SubmitDispatcher> {
	return Effect.sync(() => {
		const fetch_impl =
			options.fetch ??
			((url: URL, init: RequestInit): Promise<Response> => {
				return fetch(url, init);
			});

		return {
			dispatch: (request) => {
				return Effect.tryPromise({
					try: (signal) => {
						return fetch_impl(request.url, {
							...request.init,
							signal: merge_abort_signals([
								signal,
								request.init.signal,
							]),
						});
					},
					catch: (error) => {
						if (is_abort_error(error)) {
							return new SubmitAborted();
						}
						return new SubmitDispatchFailed({ error });
					},
				});
			},
		};
	});
}

function merge_abort_signals(
	signals: Array<AbortSignal | null | undefined>,
): AbortSignal {
	const live_signals = signals.filter((signal): signal is AbortSignal => {
		return signal !== null && signal !== undefined;
	});
	if (live_signals.length === 1) {
		return live_signals[0]!;
	}
	const controller = new AbortController();
	for (const signal of live_signals) {
		if (signal.aborted) {
			controller.abort();
			break;
		}
		signal.addEventListener(
			"abort",
			() => {
				controller.abort();
			},
			{ once: true },
		);
	}
	return controller.signal;
}

function is_abort_error(error: unknown): boolean {
	return error instanceof DOMException && error.name === abort_error_name;
}

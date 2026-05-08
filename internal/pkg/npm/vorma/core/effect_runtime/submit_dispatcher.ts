import { Effect } from "effect";
import {
	BrowserFetchAborted,
	BrowserFetchFailed,
	type BrowserFetchRuntime,
} from "./browser_fetch_runtime.ts";
import {
	SubmitAborted,
	type SubmitDispatch,
	SubmitDispatchFailed,
} from "./submit_manager.ts";

export type SubmitDispatcher = {
	dispatch: (
		request: SubmitDispatch,
	) => Effect.Effect<Response, SubmitDispatchFailed | SubmitAborted>;
};

export type SubmitDispatcherOptions = {
	fetch: BrowserFetchRuntime["fetch"];
};

export function make_submit_dispatcher(
	options: SubmitDispatcherOptions,
): Effect.Effect<SubmitDispatcher> {
	return Effect.sync(() => {
		return {
			dispatch: (request) => {
				return options
					.fetch({
						url: request.url,
						init: request.init,
					})
					.pipe(
						Effect.mapError((error) => {
							if (error instanceof BrowserFetchAborted) {
								return new SubmitAborted();
							}
							if (error instanceof BrowserFetchFailed) {
								return new SubmitDispatchFailed({
									error: error.error,
								});
							}
							return new SubmitDispatchFailed({ error });
						}),
					);
			},
		};
	});
}

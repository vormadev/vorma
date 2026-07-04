type Fn = (...args: Array<any>) => any;

type PendingCall<T extends Fn> = {
	promise: Promise<Awaited<ReturnType<T>>>;
	reject: (reason?: unknown) => void;
	resolve: (value: Awaited<ReturnType<T>>) => void;
};

/** A debounced wrapper around `T` — see {@link debounce}. Every call returns a promise resolving/rejecting with that debounced invocation's outcome; `cancel()` rejects every still-pending call. */
export type Debounced<T extends Fn> = ((
	...args: Parameters<T>
) => Promise<Awaited<ReturnType<T>>>) & {
	cancel: () => void;
};

/**
 * Debounce `fn`: repeated calls within `delayInMs` of each other collapse
 * into one underlying call using the LATEST call's arguments, after the
 * delay elapses with no further calls. Every call to the debounced wrapper
 * returns its own promise; ALL pending callers for a given debounce window
 * resolve/reject together with that one underlying call's outcome (this is
 * a coalescing debounce, not a "only the last caller gets notified" one —
 * nobody's promise is silently abandoned).
 *
 * `debounced.cancel()` clears the pending timer and rejects every
 * currently-pending caller's promise (marked as already-handled, so an
 * uncaught cancellation rejection never surfaces as an unhandled rejection
 * warning if the caller does not itself await/catch it).
 */
export function debounce<T extends Fn>(fn: T, delayInMs: number): Debounced<T> {
	let timeout_id: ReturnType<typeof globalThis.setTimeout> | undefined;
	let latest_args: Parameters<T> | undefined;
	let pending_calls: Array<PendingCall<T>> = [];

	const debounced = ((...args: Parameters<T>) => {
		latest_args = args;
		if (timeout_id !== undefined) {
			globalThis.clearTimeout(timeout_id);
		}

		let resolve_pending!: PendingCall<T>["resolve"];
		let reject_pending!: PendingCall<T>["reject"];
		const promise = new Promise<Awaited<ReturnType<T>>>((resolve, reject) => {
			resolve_pending = resolve;
			reject_pending = reject;
		});
		pending_calls.push({
			promise,
			reject: reject_pending,
			resolve: resolve_pending,
		});

		timeout_id = globalThis.setTimeout(() => {
			timeout_id = undefined;
			const args = latest_args as Parameters<T>;
			latest_args = undefined;
			const calls = pending_calls;
			pending_calls = [];
			try {
				Promise.resolve(fn(...args)).then(
					(value) => {
						for (const call of calls) {
							call.resolve(value);
						}
					},
					(error: unknown) => {
						reject_calls(calls, error, false);
					},
				);
			} catch (error) {
				reject_calls(calls, error, false);
			}
		}, delayInMs);
		return promise;
	}) as Debounced<T>;

	debounced.cancel = (): void => {
		if (timeout_id === undefined) {
			return;
		}
		globalThis.clearTimeout(timeout_id);
		timeout_id = undefined;
		latest_args = undefined;
		const calls = pending_calls;
		pending_calls = [];
		reject_calls(calls, new Error("Debounced call cancelled"), true);
	};

	return debounced;
}

function reject_calls<T extends Fn>(
	calls: Array<PendingCall<T>>,
	error: unknown,
	mark_handled: boolean,
): void {
	for (const call of calls) {
		if (mark_handled) {
			void call.promise.catch(() => {});
		}
		call.reject(error);
	}
}

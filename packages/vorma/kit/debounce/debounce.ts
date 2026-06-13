type Fn = (...args: Array<any>) => any;

type PendingCall<T extends Fn> = {
	promise: Promise<Awaited<ReturnType<T>>>;
	reject: (reason?: unknown) => void;
	resolve: (value: Awaited<ReturnType<T>>) => void;
};

export type Debounced<T extends Fn> = ((
	...args: Parameters<T>
) => Promise<Awaited<ReturnType<T>>>) & {
	cancel: () => void;
};

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

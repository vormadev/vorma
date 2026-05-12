export type Core6Deferred<T> = {
	readonly promise: Promise<T>;
	reject: (reason: unknown) => boolean;
	resolve: (value: T | PromiseLike<T>) => boolean;
	settled: () => boolean;
};

export function create_core6_deferred<T>(): Core6Deferred<T> {
	let settled = false;
	let resolve_inner!: (value: T | PromiseLike<T>) => void;
	let reject_inner!: (reason: unknown) => void;
	const promise = new Promise<T>((resolve, reject) => {
		resolve_inner = resolve;
		reject_inner = reject;
	});

	return {
		promise,
		reject: (reason) => {
			if (settled) {
				return false;
			}
			settled = true;
			reject_inner(reason);
			return true;
		},
		resolve: (value) => {
			if (settled) {
				return false;
			}
			settled = true;
			resolve_inner(value);
			return true;
		},
		settled: () => {
			return settled;
		},
	};
}

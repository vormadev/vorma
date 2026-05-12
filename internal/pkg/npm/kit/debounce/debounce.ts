type Fn = (...args: Array<any>) => any;

export type Debounced<T extends Fn> = ((
	...args: Parameters<T>
) => Promise<Awaited<ReturnType<T>>>) & {
	cancel: () => void;
};

export function debounce<T extends Fn>(
	fn: T,
	delay_in_ms: number,
): Debounced<T> {
	let timeout_id: ReturnType<typeof globalThis.setTimeout> | undefined;

	const debounced = ((...args: Parameters<T>) => {
		return new Promise<Awaited<ReturnType<T>>>((resolve, reject) => {
			if (timeout_id !== undefined) {
				globalThis.clearTimeout(timeout_id);
			}
			timeout_id = globalThis.setTimeout(() => {
				timeout_id = undefined;
				try {
					resolve(fn(...args));
				} catch (error) {
					reject(error);
				}
			}, delay_in_ms);
		});
	}) as Debounced<T>;

	debounced.cancel = (): void => {
		if (timeout_id === undefined) {
			return;
		}
		globalThis.clearTimeout(timeout_id);
		timeout_id = undefined;
	};

	return debounced;
}

type Fn = (...args: Array<any>) => any;

export function debounce<T extends Fn>(
	fn: T,
	delayInMs: number,
): (...args: Parameters<T>) => Promise<Awaited<ReturnType<T>>> {
	let timeoutID: any;

	return (...args: Parameters<T>) => {
		return new Promise<Awaited<ReturnType<T>>>((resolve, reject) => {
			clearTimeout(timeoutID);
			timeoutID = globalThis.setTimeout(() => {
				try {
					resolve(fn(...args));
				} catch (error) {
					reject(error);
				}
			}, delayInMs);
		});
	};
}

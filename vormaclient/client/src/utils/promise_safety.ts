export function observePromiseRejection<T>(promise: Promise<T>): Promise<T> {
	// Attach a side-chain catch so detached promises don't surface
	// unhandled rejections. The original promise still resolves/rejects.
	void promise.catch(() => {});
	return promise;
}

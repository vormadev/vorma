import { isAbortError } from "./utils/errors.ts";

export function wrapLoaderPromisesWithChildAbort(props: {
	loaderPromises: Array<Promise<any>>;
	abortControllers: Array<AbortController | null>;
}): Array<Promise<any>> {
	const { loaderPromises, abortControllers } = props;
	return loaderPromises.map(async (promise, index) => {
		return promise.catch((error) => {
			// If this promise failed with a true error (not just an abort)
			if (!isAbortError(error)) {
				// Abort all subsequent (child) loaders immediately
				for (let j = index + 1; j < abortControllers.length; j++) {
					abortControllers[j]?.abort();
				}
			}
			// Re-throw the error so Promise.allSettled sees it as 'rejected'
			throw error;
		});
	});
}

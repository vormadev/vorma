// Used by client route defs file (e.g., vorma.routes.ts)

type ImportPromise = Promise<Record<string, any>>;
type Key<T extends ImportPromise> = keyof Awaited<T>;

/**
 * Registers a route with the given route pattern,
 * module import promise, component export key, and
 * optional error boundary export key. Only for use
 * in your centralized build-time route definitions
 * file.
 */
export function route<IP extends ImportPromise>(
	// oxlint-disable-next-line no-unused-vars
	pattern: string,
	// oxlint-disable-next-line no-unused-vars
	importPromise: IP,
	// oxlint-disable-next-line no-unused-vars
	componentKey: Key<IP>,
	// oxlint-disable-next-line no-unused-vars
	errorBoundaryKey?: Key<IP>,
): void {}

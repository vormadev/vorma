type ImportPromise = Promise<Record<string, any>>;
type Key<T extends ImportPromise> = keyof Awaited<T>;

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

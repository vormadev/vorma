import type {
	BuildtimeImportKey,
	BuildtimeImportPromise,
} from "./src/runtime.ts";

export function route<IP extends BuildtimeImportPromise>(
	// oxlint-disable-next-line no-unused-vars
	pattern: string,
	// oxlint-disable-next-line no-unused-vars
	importPromise: IP,
	// oxlint-disable-next-line no-unused-vars
	componentKey: BuildtimeImportKey<IP>,
	// oxlint-disable-next-line no-unused-vars
	errorBoundaryKey?: BuildtimeImportKey<IP>,
): void {}

type BuildtimeImportPromise = Promise<Record<string, any>>;
type BuildtimeImportKey<T extends BuildtimeImportPromise> = keyof Awaited<T>;

export function route<IP extends BuildtimeImportPromise>(
	pattern: string,
	importPromise: IP,
	componentKey: BuildtimeImportKey<IP>,
	errorBoundaryKey?: BuildtimeImportKey<IP>,
): void {
	console.log(
		JSON.stringify({
			Pattern: pattern,
			Module: importPromise,
			ExportKey: componentKey,
			ErrorExportKey: errorBoundaryKey ?? "",
		}),
	);
}

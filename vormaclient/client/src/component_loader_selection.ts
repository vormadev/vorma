export function buildActiveComponents(props: {
	importURLs: string[];
	exportKeys: string[];
	modulesMap: Map<string, any>;
}): Array<any> {
	const { importURLs, exportKeys, modulesMap } = props;
	return importURLs.map((url, i) => {
		const module = modulesMap.get(url);
		const key = exportKeys[i] ?? "default";
		return module?.[key] ?? null;
	});
}

export function resolveErrorBoundaryComponent(props: {
	errorIdx: number;
	importURLs: string[];
	errorExportKeys: Array<string> | undefined;
	modulesMap: Map<string, any>;
	defaultErrorBoundary: any;
}): any {
	const {
		errorIdx,
		importURLs,
		errorExportKeys,
		modulesMap,
		defaultErrorBoundary,
	} = props;
	const errorModuleURL = importURLs[errorIdx];
	let errorComponent;

	if (errorModuleURL) {
		const errorModule = modulesMap.get(errorModuleURL);
		const errorKey = errorExportKeys ? errorExportKeys[errorIdx] : null;
		if (errorKey && errorModule) {
			errorComponent = errorModule[errorKey];
		}
	}

	return errorComponent ?? defaultErrorBoundary;
}

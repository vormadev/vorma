import type { SkipCheckContext } from "./skip_server_fetch_types.ts";

export function buildSkipResultItem(props: {
	ctx: SkipCheckContext;
	pattern: string;
}): { importURL: string; exportKey: string; loaderData: any } | null {
	const { ctx, pattern } = props;
	const moduleInfo = ctx.clientModuleMap[pattern];
	if (!moduleInfo) {
		return null;
	}

	const hasServerLoader = ctx.routeManifest[pattern] === 1;
	if (!hasServerLoader) {
		return {
			importURL: moduleInfo.importURL,
			exportKey: moduleInfo.exportKey,
			loaderData: undefined,
		};
	}

	const currentPatternIndex = ctx.currentMatchedPatterns.indexOf(pattern);
	if (currentPatternIndex === -1) {
		return null;
	}

	return {
		importURL: moduleInfo.importURL,
		exportKey: moduleInfo.exportKey,
		loaderData: ctx.currentLoadersData[currentPatternIndex],
	};
}

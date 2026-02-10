import { resolvePublicHref } from "./resolve_public_href.ts";

export async function loadComponentModules(
	importURLs: string[],
): Promise<Map<string, any>> {
	const dedupedURLs = [...new Set(importURLs)];
	const modules = await Promise.all(
		dedupedURLs.map(async (url) => {
			if (!url) return undefined;
			return import(/* @vite-ignore */ resolvePublicHref(url));
		}),
	);
	return new Map(dedupedURLs.map((url, i) => [url, modules[i]]));
}

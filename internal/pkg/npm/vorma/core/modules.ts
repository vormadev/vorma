/// <reference types="vite/client" />

export type ModulePreloadOptions = {
	dev?: boolean;
};

export function preload_modules(
	deps: string[],
	options: ModulePreloadOptions = {},
): void {
	if (options.dev ?? import.meta.env.DEV) {
		return;
	}
	for (const dep of new Set(deps)) {
		if (
			document.head.querySelector(
				`link[rel="modulepreload"][href="${dep}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "modulepreload";
		link.href = dep;
		document.head.appendChild(link);
	}
}

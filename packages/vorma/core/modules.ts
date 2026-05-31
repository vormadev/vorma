/// <reference types="vite/client" />

export function preload_modules(deps: string[]): void {
	if (import.meta.env.DEV) {
		return;
	}
	for (const dep of new Set(deps)) {
		if (document.head.querySelector(`link[rel="modulepreload"][href="${dep}"]`)) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "modulepreload";
		link.href = dep;
		document.head.appendChild(link);
	}
}

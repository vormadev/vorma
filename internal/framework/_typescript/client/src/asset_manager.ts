import { resolvePublicHref } from "./resolve_public_href.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export class AssetManager {
	static preloadModule(url: string): void {
		const href = resolvePublicHref(url);
		if (
			document.querySelector(
				`link[rel="modulepreload"][href="${CSS.escape(href)}"]`,
			)
		) {
			return;
		}

		const link = document.createElement("link");
		link.rel = "modulepreload";
		link.href = href;
		document.head.appendChild(link);
	}

	static preloadCSS(url: string): Promise<void> {
		const href = resolvePublicHref(url);

		if (
			document.querySelector(
				`link[rel="preload"][href="${CSS.escape(href)}"]`,
			)
		) {
			return Promise.resolve();
		}

		const link = document.createElement("link");
		link.rel = "preload";
		link.setAttribute("as", "style");
		link.href = href;

		document.head.appendChild(link);

		return new Promise((resolve, reject) => {
			link.onload = () => resolve();
			link.onerror = reject;
		});
	}

	static applyCSS(bundles: string[]): void {
		window.requestAnimationFrame(() => {
			const prefix = __vormaClientGlobal.get("publicPathPrefix");

			for (const bundle of bundles) {
				// Check using the data attribute without escaping
				if (
					document.querySelector(
						`link[data-vorma-css-bundle="${bundle}"]`,
					)
				) {
					continue;
				}

				const link = document.createElement("link");
				link.rel = "stylesheet";
				link.href = prefix + bundle;
				link.setAttribute("data-vorma-css-bundle", bundle);
				document.head.appendChild(link);
			}
		});
	}
}

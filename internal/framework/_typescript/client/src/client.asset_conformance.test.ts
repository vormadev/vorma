import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import { AssetManager } from "./asset_manager.ts";
import {
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

function findLink(
	selector: string,
	hrefSuffix: string,
): HTMLLinkElement | undefined {
	const links = Array.from(
		document.querySelectorAll<HTMLLinkElement>(selector),
	);
	return links.find((link) => link.href.endsWith(hrefSuffix));
}

describeNavigationTestSuite(() => {
	describe("Asset management conformance", () => {
		it("FEC-ASSET-001_FE-ASSET-001_FE-ASSET-002_module_and_css_preload_deduplicate_per_url", async () => {
			setupGlobalVormaContext({
				viteDevURL: "http://localhost:5173",
				publicPathPrefix: "/assets",
			});

			AssetManager.preloadModule("/module.js");
			AssetManager.preloadModule("/module.js");

			const cssPromise = AssetManager.preloadCSS("/bundle.css");
			AssetManager.preloadCSS("/bundle.css");

			const moduleLinks = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					`link[rel="modulepreload"]`,
				),
			).filter((link) =>
				link.href.endsWith("http://localhost:5173/module.js"),
			);
			expect(moduleLinks).toHaveLength(1);

			const cssLinks = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					`link[rel="preload"][as="style"]`,
				),
			).filter((link) =>
				link.href.endsWith("http://localhost:5173/bundle.css"),
			);
			expect(cssLinks).toHaveLength(1);

			cssLinks[0]?.dispatchEvent(new Event("load"));
			await cssPromise;
		});

		it("FEC-ASSET-002_FE-ASSET-003_css_preload_promise_resolves_on_load_and_rejects_on_error", async () => {
			setupGlobalVormaContext({
				viteDevURL: "http://localhost:5173",
				publicPathPrefix: "/assets",
			});

			const okPromise = AssetManager.preloadCSS("/ok.css");
			const okLink = findLink(
				`link[rel="preload"][as="style"]`,
				"http://localhost:5173/ok.css",
			);
			expect(okLink).toBeDefined();
			okLink?.dispatchEvent(new Event("load"));
			await expect(okPromise).resolves.toBeUndefined();

			const errPromise = AssetManager.preloadCSS("/err.css");
			const errLink = findLink(
				`link[rel="preload"][as="style"]`,
				"http://localhost:5173/err.css",
			);
			expect(errLink).toBeDefined();
			errLink?.dispatchEvent(new Event("error"));
			await expect(errPromise).rejects.toBeDefined();
		});

		it("FEC-ASSET-003_FE-ASSET-004_FE-ASSET-005_css_apply_dedupes_and_public_href_base_matches_dev_and_prod", async () => {
			setupGlobalVormaContext({
				viteDevURL: "http://localhost:5173",
				publicPathPrefix: "/assets",
			});

			AssetManager.preloadModule("/dev-module.js");
			const devCSSPromise = AssetManager.preloadCSS("/dev-style.css");

			const devModuleLink = findLink(
				`link[rel="modulepreload"]`,
				"http://localhost:5173/dev-module.js",
			);
			const devCSSLink = findLink(
				`link[rel="preload"][as="style"]`,
				"http://localhost:5173/dev-style.css",
			);
			expect(devModuleLink).toBeDefined();
			expect(devCSSLink).toBeDefined();
			devCSSLink?.dispatchEvent(new Event("load"));
			await devCSSPromise;

			setupGlobalVormaContext({
				viteDevURL: "",
				publicPathPrefix: "/assets",
			});

			AssetManager.preloadModule("/prod-module.js");
			const prodCSSPromise = AssetManager.preloadCSS("/prod-style.css");

			const prodModuleLink = findLink(
				`link[rel="modulepreload"]`,
				"/assets/prod-module.js",
			);
			const prodCSSLink = findLink(
				`link[rel="preload"][as="style"]`,
				"/assets/prod-style.css",
			);
			expect(prodModuleLink).toBeDefined();
			expect(prodCSSLink).toBeDefined();

			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb: FrameRequestCallback) => {
					cb(0);
					return 1;
				});
			try {
				AssetManager.applyCSS([
					"/bundle-a.css",
					"/bundle-a.css",
					"/bundle-b.css",
				]);
			} finally {
				rafSpy.mockRestore();
			}

			const applied = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					`link[data-vorma-css-bundle]`,
				),
			);
			expect(applied).toHaveLength(2);
			expect(
				applied.some(
					(link) =>
						link.getAttribute("data-vorma-css-bundle") ===
							"/bundle-a.css" &&
						link.href.endsWith("/assets/bundle-a.css"),
				),
			).toBe(true);
			expect(
				applied.some(
					(link) =>
						link.getAttribute("data-vorma-css-bundle") ===
							"/bundle-b.css" &&
						link.href.endsWith("/assets/bundle-b.css"),
				),
			).toBe(true);

			prodCSSLink?.dispatchEvent(new Event("load"));
			await prodCSSPromise;
		});

		it("FEC-ASSET-004_FE-ASSET-006_public_prefix_and_bundle_join_normalizes_boundary_slashes", async () => {
			setupGlobalVormaContext({
				viteDevURL: "",
				publicPathPrefix: "/assets/",
			});

			AssetManager.preloadModule("/module-with-slash.js");
			const preloadCSSPromise = AssetManager.preloadCSS(
				"/preload-with-slash.css",
			);

			const moduleLink = findLink(
				`link[rel="modulepreload"]`,
				"/assets/module-with-slash.js",
			);
			const preloadLink = findLink(
				`link[rel="preload"][as="style"]`,
				"/assets/preload-with-slash.css",
			);
			expect(moduleLink).toBeDefined();
			expect(preloadLink).toBeDefined();
			expect(moduleLink?.getAttribute("href")).toBe(
				"/assets/module-with-slash.js",
			);
			expect(preloadLink?.getAttribute("href")).toBe(
				"/assets/preload-with-slash.css",
			);

			preloadLink?.dispatchEvent(new Event("load"));
			await preloadCSSPromise;

			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb: FrameRequestCallback) => {
					cb(0);
					return 1;
				});
			try {
				AssetManager.applyCSS(["/apply-with-slash.css"]);
			} finally {
				rafSpy.mockRestore();
			}

			const applied = document.querySelector<HTMLLinkElement>(
				`link[data-vorma-css-bundle="/apply-with-slash.css"]`,
			);
			expect(applied).toBeDefined();
			expect(applied?.getAttribute("href")).toBe(
				"/assets/apply-with-slash.css",
			);
		});
	});
});

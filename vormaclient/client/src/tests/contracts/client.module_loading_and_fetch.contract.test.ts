import { describe, expect, it, vi } from "vitest";
import {
	createRouteDataResponse,
	installContractVormaGlobal,
	loadClientAPI,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client module-loading and fetch contracts", () => {
	it("includes current build ID in navigation fetch URL", async () => {
		installContractVormaGlobal({ buildID: "test-build-123" });
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(
				createRouteDataResponse(
					{},
					{ headers: { "X-Vorma-Build-Id": "test-build-123" } },
				),
			);

		await api.vormaNavigate("/test-url");
		await vi.runAllTimersAsync();

		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toBe(
			"http://localhost:3000/test-url?vorma_json=test-build-123",
		);
	});

	it("loads modules using publicPathPrefix and maps export keys to active components", async () => {
		const mockModule = {
			default: () => "DefaultExport",
			NamedExport: () => "NamedExport",
		};
		vi.doMock("/assets/multi-export.js", () => mockModule);
		installContractVormaGlobal({ publicPathPrefix: "/assets" });

		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: ["/multi-export.js", "/multi-export.js"],
				exportKeys: ["default", "NamedExport"],
				matchedPatterns: ["/route-a", "/route-b"],
				loadersData: [{}, {}],
			}),
		);

		await api.vormaNavigate("/multi-export");
		await vi.runAllTimersAsync();

		const components = api.__vormaClientGlobal.get("activeComponents");
		expect(components).toHaveLength(2);
		expect(components?.[0]).toBe(mockModule.default);
		expect(components?.[1]).toBe(mockModule.NamedExport);
	});

	it("sets active error boundary from server error index and error export key", async () => {
		const layout = () => "Layout";
		const page = () => "Page";
		const errorBoundary = () => "Error Boundary";

		vi.doMock("/layout.js", () => ({ Layout: layout }));
		vi.doMock("/page.js", () => ({ Page: page }));
		vi.doMock("/error.js", () => ({ ErrorBoundary: errorBoundary }));

		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: ["/layout.js", "/page.js", "/error.js"],
				exportKeys: ["Layout", "Page", "ErrorBoundary"],
				errorExportKeys: ["", "", "ErrorBoundary"],
				matchedPatterns: ["/layout", "/page", "/error"],
				loadersData: [{}, {}, {}],
				outermostServerErrorIdx: 2,
			}),
		);

		await api.vormaNavigate("/with-error-boundary");
		await vi.runAllTimersAsync();

		expect(api.__vormaClientGlobal.get("activeErrorBoundary")).toBe(
			errorBoundary,
		);
	});

	it("falls back to default error boundary when server index has no error component", async () => {
		const defaultErrorBoundary = () => null;
		installContractVormaGlobal({ defaultErrorBoundary });

		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				matchedPatterns: [],
				loadersData: [],
				outermostServerErrorIdx: 0,
			}),
		);

		await api.vormaNavigate("/fallback-error-boundary");
		await vi.runAllTimersAsync();

		expect(api.__vormaClientGlobal.get("activeErrorBoundary")).toBe(
			defaultErrorBoundary,
		);
	});

	it("replaces active error boundary when later navigations report a new boundary", async () => {
		const firstBoundary = () => "First Boundary";
		const secondBoundary = () => "Second Boundary";
		vi.doMock("/error-a.js", () => ({ ErrorA: firstBoundary }));
		vi.doMock("/error-b.js", () => ({ ErrorB: secondBoundary }));

		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					importURLs: ["/error-a.js"],
					exportKeys: ["ErrorA"],
					errorExportKeys: ["ErrorA"],
					matchedPatterns: ["/error-a"],
					loadersData: [{}],
					outermostServerErrorIdx: 0,
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					importURLs: ["/error-b.js"],
					exportKeys: ["ErrorB"],
					errorExportKeys: ["ErrorB"],
					matchedPatterns: ["/error-b"],
					loadersData: [{}],
					outermostServerErrorIdx: 0,
				}),
			);

		await api.vormaNavigate("/error-a");
		await vi.runAllTimersAsync();
		expect(api.__vormaClientGlobal.get("activeErrorBoundary")).toBe(
			firstBoundary,
		);

		await api.vormaNavigate("/error-b");
		await vi.runAllTimersAsync();
		expect(api.__vormaClientGlobal.get("activeErrorBoundary")).toBe(
			secondBoundary,
		);
		expect(fetchSpy).toHaveBeenCalledTimes(2);
	});

	it("passes matched server data into registered client wait functions", async () => {
		const waitFn = vi.fn().mockResolvedValue({ clientData: "ok" });
		installContractVormaGlobal({
			patternToWaitFnMap: { "/pattern": waitFn },
		});

		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: [],
				exportKeys: [],
				matchedPatterns: ["/pattern"],
				loadersData: [{ serverData: "test" }],
				hasRootData: true,
			}),
		);

		await api.vormaNavigate("/pattern/test");
		await vi.runAllTimersAsync();

		expect(waitFn).toHaveBeenCalledWith(
			expect.objectContaining({
				params: expect.any(Object),
				splatValues: expect.any(Array),
				serverDataPromise: expect.any(Promise),
				signal: expect.any(AbortSignal),
			}),
		);

		const call = waitFn.mock.calls[0]?.[0];
		const serverData = await call.serverDataPromise;
		expect(serverData).toEqual({
			matchedPatterns: ["/pattern"],
			loaderData: { serverData: "test" },
			rootData: { serverData: "test" },
			buildID: "1",
		});
	});

	it("falls back to server fetch when skip-cache is missing required server loader data", async () => {
		vi.doMock("/noop.js", () => ({
			default: () => null,
		}));
		installContractVormaGlobal({
			routeManifest: { "/needs-data": 1 },
			clientModuleMap: {
				"/needs-data": {
					importURL: "/noop.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: ["/needs-data"],
			loadersData: [],
		});
		const api = await loadClientAPI();
		await api.__registerClientLoaderPattern("/needs-data");

		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: ["/noop.js"],
				exportKeys: ["default"],
				matchedPatterns: ["/needs-data"],
				loadersData: [{ serverData: "from-server" }],
			}),
		);

		await api.vormaNavigate("/needs-data");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.__vormaClientGlobal.get("loadersData")).toEqual([
			{ serverData: "from-server" },
		]);
	});

	it("resolves module imports from viteDevURL when present", async () => {
		const devComponent = () => "DevComponent";
		vi.doMock("http://localhost:5173/dev-module.js", () => ({
			default: devComponent,
		}));
		installContractVormaGlobal({
			viteDevURL: "http://localhost:5173",
			publicPathPrefix: "/assets",
		});

		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: ["/dev-module.js"],
				exportKeys: ["default"],
				matchedPatterns: ["/dev"],
				loadersData: [{}],
			}),
		);

		await api.vormaNavigate("/dev-module");
		await vi.runAllTimersAsync();

		const components = api.__vormaClientGlobal.get("activeComponents");
		expect(components?.[0]).toBe(devComponent);
	});

	it("preloads unique deps as modulepreload links in production mode", async () => {
		const originalEnv = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		try {
			const api = await loadClientAPI();
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					importURLs: [],
					exportKeys: [],
					matchedPatterns: [],
					loadersData: [],
					deps: ["/dep-a.js", "/dep-b.js", "/dep-a.js"],
				}),
			);

			await api.vormaNavigate("/with-deps");
			await vi.runAllTimersAsync();

			const links = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					'link[rel="modulepreload"]',
				),
			);
			expect(links).toHaveLength(2);
			const hrefs = links.map((link) => link.getAttribute("href"));
			expect(hrefs).toEqual(
				expect.arrayContaining(["/dep-a.js", "/dep-b.js"]),
			);
		} finally {
			(import.meta.env as any).DEV = originalEnv;
		}
	});

	it("preloads css bundle assets during fetch route-data phase", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: [],
				exportKeys: [],
				matchedPatterns: [],
				loadersData: [],
				cssBundles: ["/a.css", "/b.css"],
			}),
		);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() => node.onload?.(new Event("load")));
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/with-css-bundles");
		await vi.runAllTimersAsync();

		const preloadLinks = Array.from(
			document.querySelectorAll<HTMLLinkElement>(
				'link[rel="preload"][as="style"]',
			),
		);
		expect(preloadLinks).toHaveLength(2);
		const hrefs = preloadLinks.map((link) => link.getAttribute("href"));
		expect(hrefs).toEqual(expect.arrayContaining(["/a.css", "/b.css"]));
	});

	it("normalizes trailing slashes when applying stylesheet hrefs", async () => {
		installContractVormaGlobal({ publicPathPrefix: "/static/" });
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: [],
				exportKeys: [],
				matchedPatterns: [],
				loadersData: [],
				cssBundles: ["/styles.css"],
			}),
		);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() => node.onload?.(new Event("load")));
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/with-stylesheet");
		await vi.runAllTimersAsync();

		const stylesheet = document.querySelector<HTMLLinkElement>(
			'link[rel="stylesheet"][data-vorma-css-bundle="/styles.css"]',
		);
		expect(stylesheet).toBeTruthy();
		expect(stylesheet?.getAttribute("href")).toBe("/static/styles.css");
	});

	it("cleans up completed navigation entries so same-url navigation refetches", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/cleanup-target");
		await vi.runAllTimersAsync();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});

		await api.vormaNavigate("/cleanup-target");
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(2);
	});
});

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VORMA_SYMBOL } from "../../app/context.ts";
import {
	buildRouteDataRequestURL,
	resolveServerRouteDataResult,
	startParallelClientLoaders,
} from "../../core/navigation/fetch_route_data_server.ts";
import type { NavigateProps } from "../../core/navigation/types.ts";
import * as renderRuntimeModule from "../../core/render_runtime.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function installVormaGlobal(overrides: Record<string, unknown> = {}): void {
	(globalThis as any)[VORMA_SYMBOL] = {
		buildID: "1",
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		activeComponents: [],
		outermostServerError: undefined,
		outermostClientError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		clientLoadersData: [],
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: {},
		...overrides,
	};
}

function buildNavigationProps(
	overrides: Partial<NavigateProps> = {},
): NavigateProps {
	return {
		href: "http://localhost:3000/next",
		navigationType: "browserHistory",
		...overrides,
	};
}

function createMatch(pattern: string) {
	return {
		registeredPattern: {
			originalPattern: pattern,
			normalizedSegments: [],
			lastSegType: "static",
		},
		params: {},
		splatValues: [],
	};
}

describe("fetch route data server internals", () => {
	beforeEach(() => {
		installVormaGlobal();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe("buildRouteDataRequestURL", () => {
		it("always appends vorma_json build ID", () => {
			installVormaGlobal({
				buildID: "build-42",
				deploymentID: "deploy-1",
			});

			const requestURL = buildRouteDataRequestURL({
				targetHref: "http://localhost:3000/items?existing=1",
				navigationType: "browserHistory",
			});

			expect(requestURL.searchParams.get("existing")).toBe("1");
			expect(requestURL.searchParams.get("vorma_json")).toBe("build-42");
			expect(requestURL.searchParams.has("dpl")).toBe(false);
		});

		it("adds deployment ID only for revalidation requests", () => {
			installVormaGlobal({
				buildID: "build-42",
				deploymentID: "deploy-1",
			});

			const requestURL = buildRouteDataRequestURL({
				targetHref: "http://localhost:3000/items",
				navigationType: "revalidation",
			});

			expect(requestURL.searchParams.get("vorma_json")).toBe("build-42");
			expect(requestURL.searchParams.get("dpl")).toBe("deploy-1");
		});
	});

	describe("resolveServerRouteDataResult", () => {
		it("returns aborted outcome when server response is missing", () => {
			const controller = new AbortController();
			const result = resolveServerRouteDataResult({
				controller,
				navigationProps: buildNavigationProps(),
				serverResult: { redirectData: null, response: undefined },
			});

			expect(result).toEqual({
				type: "outcome",
				outcome: { type: "aborted" },
			});
			expect(controller.signal.aborted).toBe(true);
		});

		it("returns redirect outcome when redirect should be effectuated", () => {
			const controller = new AbortController();
			const redirectData = {
				status: "should",
				href: "/redirect-target",
				hrefDetails: {
					isHTTP: true,
					isInternal: true,
					isExternal: false,
					absoluteURL: "http://localhost:3000/redirect-target",
				},
			} as const;
			const response = new Response("{}", {
				status: 200,
				headers: { "Content-Type": "application/json" },
			});

			const result = resolveServerRouteDataResult({
				controller,
				navigationProps: buildNavigationProps(),
				serverResult: {
					redirectData: redirectData as any,
					response,
					json: {
						matchedPatterns: [],
						loadersData: [],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: [],
						cssBundles: [],
						title: undefined,
						metaHeadEls: undefined,
						restHeadEls: undefined,
					},
				},
			});

			expect(result).toEqual({
				type: "outcome",
				outcome: {
					type: "redirect",
					redirectData: redirectData as any,
					props: buildNavigationProps(),
				},
			});
			expect(controller.signal.aborted).toBe(true);
		});

		it("returns redirect outcome even when redirect response status is non-OK", () => {
			const controller = new AbortController();
			const redirectData = {
				status: "should",
				href: "/redirect-target",
				hrefDetails: {
					isHTTP: true,
					isInternal: true,
					isExternal: false,
					absoluteURL: "http://localhost:3000/redirect-target",
				},
			} as const;
			const response = new Response("{}", {
				status: 409,
				headers: { "Content-Type": "application/json" },
			});

			const result = resolveServerRouteDataResult({
				controller,
				navigationProps: buildNavigationProps(),
				serverResult: {
					redirectData: redirectData as any,
					response,
					json: undefined,
				},
			});

			expect(result).toEqual({
				type: "outcome",
				outcome: {
					type: "redirect",
					redirectData: redirectData as any,
					props: buildNavigationProps(),
				},
			});
			expect(controller.signal.aborted).toBe(true);
		});

		it("throws for non-OK responses", () => {
			const controller = new AbortController();
			const response = new Response("fail", { status: 500 });

			expect(() =>
				resolveServerRouteDataResult({
					controller,
					navigationProps: buildNavigationProps(),
					serverResult: { redirectData: null, response },
				}),
			).toThrow("Fetch failed with status 500");
			expect(controller.signal.aborted).toBe(true);
		});

		it("returns success payload for OK JSON responses", () => {
			const controller = new AbortController();
			const response = new Response("{}", {
				status: 200,
				headers: { "Content-Type": "application/json" },
			});
			const json = {
				matchedPatterns: ["/items/:id"],
				loadersData: [{ id: "123" }],
				importURLs: ["/items.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				hasRootData: true,
				params: { id: "123" },
				splatValues: [],
				deps: [],
				cssBundles: [],
				title: undefined,
				metaHeadEls: undefined,
				restHeadEls: undefined,
			};

			const result = resolveServerRouteDataResult({
				controller,
				navigationProps: buildNavigationProps(),
				serverResult: { redirectData: null, response, json },
			});

			expect(result).toEqual({
				type: "success",
				response,
				json,
			});
			expect(controller.signal.aborted).toBe(false);
		});
	});

	describe("startParallelClientLoaders", () => {
		it("starts matched loader wait functions and forwards resolved server data", async () => {
			vi.spyOn(
				renderRuntimeModule,
				"findPartialMatchesOnClient",
			).mockResolvedValue({
				params: { id: "123" },
				splatValues: [],
				matches: [createMatch("/a"), createMatch("/b")],
			} as any);

			installVormaGlobal({
				patternToWaitFnMap: {
					"/a": async (props: any) => {
						const serverData = await props.serverDataPromise;
						return {
							pattern: "/a",
							loaderData: serverData.loaderData,
							buildID: serverData.buildID,
						};
					},
					"/b": async (props: any) => {
						const serverData = await props.serverDataPromise;
						return {
							pattern: "/b",
							loaderData: serverData.loaderData,
							buildID: serverData.buildID,
						};
					},
				},
			});

			const serverPromise = Promise.resolve({
				redirectData: null,
				response: new Response("{}", {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "build-77",
					},
				}),
				json: {
					matchedPatterns: ["/a", "/b"],
					loadersData: ["A", "B"],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: false,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
					title: undefined,
					metaHeadEls: undefined,
					restHeadEls: undefined,
				},
			});

			const runningLoaders = await startParallelClientLoaders({
				pathname: "/next",
				serverPromise,
				signal: new AbortController().signal,
			});

			expect(Array.from(runningLoaders.keys())).toEqual(["/a", "/b"]);
			await expect(runningLoaders.get("/a")).resolves.toEqual({
				pattern: "/a",
				loaderData: "A",
				buildID: "build-77",
			});
			await expect(runningLoaders.get("/b")).resolves.toEqual({
				pattern: "/b",
				loaderData: "B",
				buildID: "build-77",
			});
		});

		it("converts server promise rejection into unavailable-server-data abort errors", async () => {
			vi.spyOn(
				renderRuntimeModule,
				"findPartialMatchesOnClient",
			).mockResolvedValue({
				params: {},
				splatValues: [],
				matches: [createMatch("/a"), createMatch("/b")],
			} as any);

			installVormaGlobal({
				patternToWaitFnMap: {
					"/a": async (props: any) => props.serverDataPromise,
					"/b": async (props: any) => props.serverDataPromise,
				},
			});

			const runningLoaders = await startParallelClientLoaders({
				pathname: "/next",
				serverPromise: Promise.reject(
					new Error("server route fetch failed"),
				),
				signal: new AbortController().signal,
			});

			await expect(runningLoaders.get("/a")).rejects.toMatchObject({
				name: "AbortError",
			});
			await expect(runningLoaders.get("/b")).rejects.toMatchObject({
				name: "AbortError",
			});
		});
	});
});

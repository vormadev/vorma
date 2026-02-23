import { beforeEach, describe, expect, it } from "vitest";
import { VORMA_SYMBOL } from "../../app/context.ts";
import {
	buildRouteDataRequestURL,
	resolveServerRouteDataResult,
} from "../../core/navigation/fetch_route_data_server.ts";
import type { NavigateProps } from "../../core/navigation/types.ts";

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
		isTouchDevice: false,
		patternToWaitFnMap: {},
		clientLoadersData: [],
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		clientModuleMap: {},
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

describe("fetch route data server internals", () => {
	beforeEach(() => {
		installVormaGlobal();
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
});

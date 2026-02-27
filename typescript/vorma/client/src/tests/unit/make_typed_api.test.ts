import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { VormaAppConfig } from "../../../src/runtime.ts";
import { makeTypedAPIClient, VORMA_SYMBOL } from "../../runtime.ts";

type TestVormaApp = {
	routes: readonly [
		{
			_type: "query";
			pattern: "/search";
			phantomInputType: { query: string };
			phantomOutputType: { ids: Array<string> };
		},
		{
			_type: "mutation";
			pattern: "/session";
			phantomInputType: { username: string };
			phantomOutputType: { ok: boolean };
			method: "POST";
		},
	];
	appConfig: VormaAppConfig;
	rootData: null;
};

const TEST_CONFIG: VormaAppConfig & { __phantom: TestVormaApp } = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
	__phantom: undefined as unknown as TestVormaApp,
};

function installVormaGlobalForTypedAPITests() {
	(globalThis as any)[VORMA_SYMBOL] = {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: TEST_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: undefined,
		runtimeRouteSnapshot: {
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
			activeErrorBoundary: undefined,
			rootElementID: undefined,
			outermostServerError: undefined,
			outermostClientError: undefined,
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			clientLoadersData: [],
		},
	};
}

describe("makeTypedAPIClient", () => {
	beforeEach(() => {
		installVormaGlobalForTypedAPITests();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it("merges resolver defaults with per-call requestInit headers", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ids: ["1"] }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		const typedAPI = makeTypedAPIClient(TEST_CONFIG, () => {
			return {
				credentials: "include",
				headers: {
					Authorization: "Bearer token",
					"X-Defaults": "1",
				},
			};
		});

		await typedAPI.query({
			pattern: "/search",
			input: { query: "vorma" },
			requestInit: {
				headers: {
					"X-Trace-ID": "trace-1",
				},
			},
			options: {
				revalidate: false,
			},
		});

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstFetchCall = fetchSpy.mock.calls[0];
		expect(firstFetchCall).toBeDefined();
		const submitURL = firstFetchCall?.[0];
		const submitRequestInit = firstFetchCall?.[1] as RequestInit;
		const submitHeaders = new Headers(
			submitRequestInit.headers ?? undefined,
		);

		expect(submitURL).toBeInstanceOf(URL);
		expect((submitURL as URL).pathname).toBe("/api/search");
		expect((submitURL as URL).search).toBe("?query=vorma");
		expect(submitRequestInit.method).toBe("GET");
		expect(submitRequestInit.credentials).toBe("include");
		expect(submitHeaders.get("Authorization")).toBe("Bearer token");
		expect(submitHeaders.get("X-Defaults")).toBe("1");
		expect(submitHeaders.get("X-Trace-ID")).toBe("trace-1");
	});

	it("supports mutation calls without request-init decoration", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		const typedAPI = makeTypedAPIClient(TEST_CONFIG);

		await typedAPI.mutate({
			pattern: "/session",
			input: { username: "alice" },
			requestInit: {
				headers: {
					"X-Request-ID": "req-1",
				},
			},
			options: {
				revalidate: false,
			},
		});

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstFetchCall = fetchSpy.mock.calls[0];
		expect(firstFetchCall).toBeDefined();
		const submitRequestInit = firstFetchCall?.[1] as RequestInit;
		const submitHeaders = new Headers(
			submitRequestInit.headers ?? undefined,
		);

		expect(submitRequestInit.method).toBe("POST");
		expect(submitRequestInit.body).toBe(
			JSON.stringify({ username: "alice" }),
		);
		expect(submitHeaders.get("X-Request-ID")).toBe("req-1");
	});
});

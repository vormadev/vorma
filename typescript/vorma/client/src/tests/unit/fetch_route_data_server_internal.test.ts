import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import type {
	GetRouteDataOutput,
	NavigateProps,
} from "../../../src/runtime.ts";
import {
	buildRouteDataRequestURL,
	createServerRouteDataPromise,
	decodeServerRouteDataJSONOrThrow,
	resolveServerRouteDataResult,
	startParallelClientLoaders,
	VORMA_SYMBOL,
} from "../../runtime.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function installVormaGlobal(overrides: Record<string, unknown> = {}): void {
	const baseGlobalState = {
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

	const runtimeRouteSnapshot = {
		buildID: baseGlobalState.buildID,
		matchedPatterns: baseGlobalState.matchedPatterns,
		loadersData: baseGlobalState.loadersData,
		importURLs: baseGlobalState.importURLs,
		exportKeys: baseGlobalState.exportKeys,
		errorExportKeys: baseGlobalState.errorExportKeys,
		hasRootData: baseGlobalState.hasRootData,
		params: baseGlobalState.params,
		splatValues: baseGlobalState.splatValues,
		activeComponents: baseGlobalState.activeComponents,
		activeErrorBoundary: baseGlobalState.activeErrorBoundary,
		outermostServerError: baseGlobalState.outermostServerError,
		outermostClientError: baseGlobalState.outermostClientError,
		outermostServerErrorIdx: baseGlobalState.outermostServerErrorIdx,
		outermostClientErrorIdx: baseGlobalState.outermostClientErrorIdx,
		outermostError: baseGlobalState.outermostError,
		outermostErrorIdx: baseGlobalState.outermostErrorIdx,
		rootElementID: baseGlobalState.rootElementID,
		clientLoadersData: baseGlobalState.clientLoadersData,
		...(typeof overrides.runtimeRouteSnapshot === "object" &&
		overrides.runtimeRouteSnapshot !== null
			? overrides.runtimeRouteSnapshot
			: {}),
	};
	const nonSnapshotGlobalState: Record<string, unknown> = {
		...baseGlobalState,
	};
	delete nonSnapshotGlobalState.buildID;
	delete nonSnapshotGlobalState.matchedPatterns;
	delete nonSnapshotGlobalState.loadersData;
	delete nonSnapshotGlobalState.importURLs;
	delete nonSnapshotGlobalState.exportKeys;
	delete nonSnapshotGlobalState.errorExportKeys;
	delete nonSnapshotGlobalState.hasRootData;
	delete nonSnapshotGlobalState.params;
	delete nonSnapshotGlobalState.splatValues;
	delete nonSnapshotGlobalState.activeComponents;
	delete nonSnapshotGlobalState.activeErrorBoundary;
	delete nonSnapshotGlobalState.rootElementID;
	delete nonSnapshotGlobalState.outermostServerError;
	delete nonSnapshotGlobalState.outermostClientError;
	delete nonSnapshotGlobalState.outermostServerErrorIdx;
	delete nonSnapshotGlobalState.outermostClientErrorIdx;
	delete nonSnapshotGlobalState.outermostError;
	delete nonSnapshotGlobalState.outermostErrorIdx;
	delete nonSnapshotGlobalState.clientLoadersData;
	delete nonSnapshotGlobalState.runtimeRouteSnapshot;

	(globalThis as any)[VORMA_SYMBOL] = {
		...nonSnapshotGlobalState,
		runtimeRouteSnapshot,
	};
}

function createRegisteredPatternRegistry(patterns: string[]) {
	const registry = createPatternRegistry({
		dynamicParamPrefixRune: TEST_VORMA_APP_CONFIG.loadersDynamicRune,
		splatSegmentRune: TEST_VORMA_APP_CONFIG.loadersSplatRune,
		explicitIndexSegment:
			TEST_VORMA_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
	});
	for (const pattern of patterns) {
		registerPattern(registry, pattern);
	}
	return registry;
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

function buildValidRouteDataJSON(
	overrides: Partial<GetRouteDataOutput> = {},
): GetRouteDataOutput {
	return {
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
		...overrides,
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

	describe("decodeServerRouteDataJSONOrThrow", () => {
		it("returns decoded payload for valid route-data JSON", () => {
			const json = buildValidRouteDataJSON();
			expect(
				decodeServerRouteDataJSONOrThrow({
					json,
				}),
			).toEqual(json);
		});

		it("throws when payload root is not an object", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: null,
				}),
			).toThrow("route-data payload must be an object");
		});

		it("throws when a required route-data key is missing", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: {
						loadersData: [],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: [],
						cssBundles: [],
					},
				}),
			).toThrow('missing required key "matchedPatterns"');
		});

		it("throws when top-level required array fields are not arrays", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: buildValidRouteDataJSON({
						importURLs: "/a.js" as unknown as string[],
					}),
				}),
			).toThrow('"importURLs" must be an array');
		});

		it("throws when hasRootData is not a boolean", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: buildValidRouteDataJSON({
						hasRootData: "yes" as unknown as boolean,
					}),
				}),
			).toThrow('"hasRootData" must be a boolean');
		});

		it("throws when params is not an object", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: buildValidRouteDataJSON({
						params: [] as unknown as Record<string, string>,
					}),
				}),
			).toThrow('"params" must be an object');
		});

		it("throws when optional clientLoadersData is present and not an array", () => {
			expect(() =>
				decodeServerRouteDataJSONOrThrow({
					json: {
						...buildValidRouteDataJSON(),
						clientLoadersData: "bad" as unknown as unknown[],
					},
				}),
			).toThrow('"clientLoadersData" must be an array when present');
		});

		it("accepts semantically odd but top-level shape-correct payloads", () => {
			expect(
				decodeServerRouteDataJSONOrThrow({
					json: {
						...buildValidRouteDataJSON({
							matchedPatterns: [""],
							loadersData: [],
							importURLs: [],
							exportKeys: [],
							errorExportKeys: [],
							hasRootData: true,
						}),
						clientLoadersData: [],
					},
				}),
			).toEqual({
				...buildValidRouteDataJSON({
					matchedPatterns: [""],
					loadersData: [],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: true,
				}),
				clientLoadersData: [],
			});
		});

		it("deep-freezes decoded payload to prevent post-decode mutation", () => {
			const decoded = decodeServerRouteDataJSONOrThrow({
				json: buildValidRouteDataJSON({
					loadersData: [{ nested: { count: 1 } }],
				}),
			});

			expect(Object.isFrozen(decoded)).toBe(true);
			expect(Object.isFrozen(decoded.loadersData)).toBe(true);
			expect(Object.isFrozen(decoded.loadersData[0] as object)).toBe(
				true,
			);
			expect(
				Object.isFrozen(
					(decoded.loadersData[0] as { nested: { count: number } })
						.nested,
				),
			).toBe(true);

			expect(() => {
				(decoded.matchedPatterns as string[]).push("/next");
			}).toThrow(TypeError);
			expect(() => {
				(
					decoded.loadersData[0] as { nested: { count: number } }
				).nested.count = 2;
			}).toThrow(TypeError);
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
			const json = buildValidRouteDataJSON();

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

	describe("createServerRouteDataPromise", () => {
		it("throws and aborts when route-data JSON violates top-level contract at ingress", async () => {
			const abortController = new AbortController();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(
					JSON.stringify({
						matchedPatterns: ["/a"],
						loadersData: ["A"],
						importURLs: "/a.js",
						exportKeys: ["default"],
						errorExportKeys: [""],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: [],
						cssBundles: [],
					}),
					{
						status: 200,
						headers: { "Content-Type": "application/json" },
					},
				),
			);

			await expect(
				createServerRouteDataPromise({
					abortController,
					url: new URL("http://localhost:3000/next"),
				}),
			).rejects.toThrow('"importURLs" must be an array');
			expect(abortController.signal.aborted).toBe(true);
		});
	});

	describe("startParallelClientLoaders", () => {
		it("starts matched loader wait functions and forwards resolved server data", async () => {
			installVormaGlobal({
				patternRegistry: createRegisteredPatternRegistry([
					"/a",
					"/a/b",
				]),
				patternToWaitFnMap: {
					"/a": async (props: any) => {
						const serverData = await props.serverDataPromise;
						return {
							pattern: "/a",
							loaderData: serverData.loaderData,
							buildID: serverData.buildID,
						};
					},
					"/a/b": async (props: any) => {
						const serverData = await props.serverDataPromise;
						return {
							pattern: "/a/b",
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
						"X-Wave-Framework-Build-Id": "build-77",
					},
				}),
				json: {
					matchedPatterns: ["/a", "/a/b"],
					loadersData: ["A", "B"],
					importURLs: ["/a.js", "/a-b.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
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
				pathname: "/a/b",
				serverPromise,
				signal: new AbortController().signal,
			});

			expect(Array.from(runningLoaders.keys())).toEqual(["/a", "/a/b"]);
			await expect(runningLoaders.get("/a")).resolves.toEqual({
				pattern: "/a",
				loaderData: "A",
				buildID: "build-77",
			});
			await expect(runningLoaders.get("/a/b")).resolves.toEqual({
				pattern: "/a/b",
				loaderData: "B",
				buildID: "build-77",
			});
		});

		it("starts speculative loader work before the server route-data promise resolves", async () => {
			const startedPatterns: string[] = [];
			installVormaGlobal({
				patternRegistry: createRegisteredPatternRegistry([
					"/a",
					"/a/b",
				]),
				patternToWaitFnMap: {
					"/a": async (props: any) => {
						startedPatterns.push("/a");
						const serverData = await props.serverDataPromise;
						return serverData.loaderData;
					},
					"/a/b": async (props: any) => {
						startedPatterns.push("/a/b");
						const serverData = await props.serverDataPromise;
						return serverData.loaderData;
					},
				},
			});

			let resolveServerResult: (value: unknown) => void = () => {};
			const serverPromise = new Promise<unknown>((resolve) => {
				resolveServerResult = resolve;
			});

			const runningLoaders = await startParallelClientLoaders({
				pathname: "/a/b",
				serverPromise: serverPromise as Promise<any>,
				signal: new AbortController().signal,
			});

			expect(Array.from(runningLoaders.keys())).toEqual(["/a", "/a/b"]);
			expect(startedPatterns).toEqual(["/a", "/a/b"]);

			let settledBeforeServerResolve = false;
			void runningLoaders.get("/a")?.finally(() => {
				settledBeforeServerResolve = true;
			});
			await Promise.resolve();
			expect(settledBeforeServerResolve).toBe(false);

			resolveServerResult({
				redirectData: null,
				response: new Response("{}", {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Wave-Framework-Build-Id": "build-78",
					},
				}),
				json: {
					matchedPatterns: ["/a", "/a/b"],
					loadersData: ["A", "B"],
					importURLs: ["/a.js", "/a-b.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
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

			await expect(runningLoaders.get("/a")).resolves.toEqual("A");
			await expect(runningLoaders.get("/a/b")).resolves.toEqual("B");
		});

		it("converts server promise rejection into unavailable-server-data abort errors", async () => {
			installVormaGlobal({
				patternRegistry: createRegisteredPatternRegistry([
					"/a",
					"/a/b",
				]),
				patternToWaitFnMap: {
					"/a": async (props: any) => props.serverDataPromise,
					"/a/b": async (props: any) => props.serverDataPromise,
				},
			});
			const serverPromise = Promise.reject(
				new Error("server route fetch failed"),
			);
			void serverPromise.catch(() => {});

			const runningLoaders = await startParallelClientLoaders({
				pathname: "/a/b",
				serverPromise,
				signal: new AbortController().signal,
			});

			await expect(runningLoaders.get("/a")).rejects.toMatchObject({
				name: "AbortError",
			});
			await expect(runningLoaders.get("/a/b")).rejects.toMatchObject({
				name: "AbortError",
			});
		});
	});
});

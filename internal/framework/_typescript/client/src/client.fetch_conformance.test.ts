import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getBuildID,
	navigationStateManager,
	revalidate,
	submit,
	vormaNavigate,
} from "./client";
import {
	addBuildIDListener,
	addRouteChangeListener,
} from "./events.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";
import { AssetManager } from "./asset_manager.ts";

function baseRouteData(overrides?: Record<string, any>) {
	return {
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		errorExportKeys: [],
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		deps: [],
		cssBundles: [],
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		...(overrides || {}),
	};
}

function setMockLocationRoot() {
	window.location.href = "http://localhost:3000/";
	window.location.pathname = "/";
	window.location.search = "";
	window.location.hash = "";
}

function getFetchURLAt(index: number): URL {
	const fetchArg = vi.mocked(fetch).mock.calls[index]?.[0];
	if (!fetchArg) {
		throw new Error(`Missing fetch call at index ${index}`);
	}
	return new URL(fetchArg.toString(), window.location.href);
}

function getFetchHeadersAt(index: number): Headers {
	const init = vi.mocked(fetch).mock.calls[index]?.[1];
	return new Headers(init?.headers);
}

function makeRedirectedResponse(response: Response, targetURL: string): Response {
	Object.defineProperty(response, "redirected", {
		value: true,
		configurable: true,
	});
	Object.defineProperty(response, "url", {
		value: targetURL,
		configurable: true,
	});
	return response;
}

describeNavigationTestSuite(({ addListener }) => {
	describe("Request/redirect/build-id conformance", () => {
		it("FEC-FETCH-001_FE-FETCH-001_FE-FETCH-002_route_and_revalidation_queries_include_build_and_deployment", async () => {
			setupGlobalVormaContext({
				buildID: "build-fetch-001",
				deploymentID: "dpl-fetch-001",
			});

			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(
					createMockResponse(baseRouteData(), {
						headers: {
							"X-Vorma-Build-Id": "build-fetch-001",
						},
					}),
				),
			);

			await vormaNavigate("/fetch-query-conformance");
			await revalidate();

			const routeURL = getFetchURLAt(0);
			const revalidateURL = getFetchURLAt(1);

			expect(routeURL.searchParams.get("vorma_json")).toBe(
				"build-fetch-001",
			);
			expect(revalidateURL.searchParams.get("vorma_json")).toBe(
				"build-fetch-001",
			);
			expect(revalidateURL.searchParams.get("dpl")).toBe(
				"dpl-fetch-001",
			);
		});

		it("FEC-FETCH-002_FE-FETCH-003_route_and_action_requests_send_redirect_handshake_header", async () => {
			vi.mocked(fetch)
				.mockResolvedValueOnce(createMockResponse(baseRouteData()))
				.mockResolvedValueOnce(createMockResponse({ ok: true }));

			await vormaNavigate("/fetch-header-route");
			await submit(
				"/api/fetch-header-action",
				{
					method: "POST",
					body: JSON.stringify({ action: true }),
					headers: {
						"Content-Type": "application/json",
					},
				},
				{ revalidate: false },
			);

			expect(getFetchHeadersAt(0).get("X-Accepts-Client-Redirect")).toBe(
				"1",
			);
			expect(getFetchHeadersAt(1).get("X-Accepts-Client-Redirect")).toBe(
				"1",
			);
		});

		it("FEC-FETCH-003_FE-FETCH-004_submit_serializes_bodies_per_contract", async () => {
			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(createMockResponse({ ok: true })),
			);

			const formData = new FormData();
			formData.append("name", "vorma");

			await submit(
				"/api/fetch-body-form",
				{
					method: "POST",
					body: formData,
				},
				{ revalidate: false },
			);

			await submit(
				"/api/fetch-body-string",
				{
					method: "POST",
					body: "plain-string-body",
				},
				{ revalidate: false },
			);

			const objectBody = { nested: { ok: true }, n: 7 };
			await submit(
				"/api/fetch-body-object",
				{
					method: "POST",
					body: objectBody as any,
				},
				{ revalidate: false },
			);

			expect(vi.mocked(fetch).mock.calls).toHaveLength(3);
			expect(vi.mocked(fetch).mock.calls[0]?.[1]?.body).toBe(formData);
			expect(vi.mocked(fetch).mock.calls[1]?.[1]?.body).toBe(
				"plain-string-body",
			);
			expect(vi.mocked(fetch).mock.calls[2]?.[1]?.body).toBe(
				JSON.stringify(objectBody),
			);
		});

		it("FEC-FETCH-004_FE-FETCH-005_redirect_priority_is_reload_then_native_then_client", async () => {
			vi.mocked(fetch).mockReset();
			setMockLocationRoot();

			const allSignalsResponse = makeRedirectedResponse(
				createMockResponse(null, {
					headers: {
						"X-Vorma-Reload": "/priority-reload",
						"X-Client-Redirect": "/priority-client-ignored",
						"X-Vorma-Build-Id": "priority-build",
					},
				}),
				"http://localhost:3000/priority-native-ignored",
			);

			vi.mocked(fetch).mockResolvedValueOnce(allSignalsResponse);
			await vormaNavigate("/priority-source-a");

			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);
			expect(window.location.href).toContain("/priority-reload");
			expect(window.location.href).toContain(
				"vorma_reload=priority-build",
			);

			vi.mocked(fetch).mockReset();
			setMockLocationRoot();
			vi.mocked(fetch)
				.mockResolvedValueOnce(
					makeRedirectedResponse(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect":
									"/priority-client-lower",
							},
						}),
						"http://localhost:3000/priority-native",
					),
				)
				.mockResolvedValueOnce(createMockResponse(baseRouteData()));

			await vormaNavigate("/priority-source-b");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);
			expect(getFetchURLAt(1).pathname).toBe("/priority-native");

			vi.mocked(fetch).mockReset();
			setMockLocationRoot();
			vi.mocked(fetch)
				.mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect": "/priority-client",
						},
					}),
				)
				.mockResolvedValueOnce(createMockResponse(baseRouteData()));

			await vormaNavigate("/priority-source-c");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);
			expect(getFetchURLAt(1).pathname).toBe("/priority-client");
		});

		it("FEC-FETCH-005_FE-FETCH-006_FE-FETCH-007_redirect_eligibility_and_strategy_match_contract", async () => {
			vi.mocked(fetch).mockReset();
			setMockLocationRoot();
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(baseRouteData(), {
					headers: {
						"X-Client-Redirect": "mailto:test@example.com",
					},
				}),
			);

			await vormaNavigate("/eligibility-mailto");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);
			expect(window.location.href.startsWith("mailto:")).toBe(false);

			vi.mocked(fetch).mockReset();
			setMockLocationRoot();
			vi.mocked(fetch)
				.mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect":
								"/eligibility-internal-target",
						},
					}),
				)
				.mockResolvedValueOnce(createMockResponse(baseRouteData()));

			await vormaNavigate("/eligibility-internal-source");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);
			expect(getFetchURLAt(1).pathname).toBe(
				"/eligibility-internal-target",
			);

			vi.mocked(fetch).mockReset();
			setMockLocationRoot();
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(null, {
					headers: {
						"X-Client-Redirect": "https://example.com/vorma-out",
					},
				}),
			);

			await vormaNavigate("/eligibility-external-source");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);
			expect(window.location.href).toMatch(
				/^https:\/\/example\.com\/vorma-out\/?$/,
			);
		});

		it("FEC-FETCH-006_FE-FETCH-008_forced_reload_redirect_adds_vorma_reload_query", async () => {
			setMockLocationRoot();
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(null, {
					headers: {
						"X-Vorma-Reload": "/forced-reload-target",
						"X-Vorma-Build-Id": "forced-build-id",
					},
				}),
			);

			await vormaNavigate("/forced-reload-source");

			const redirected = new URL(window.location.href);
			expect(redirected.pathname).toBe("/forced-reload-target");
			expect(redirected.searchParams.get("vorma_reload")).toBe(
				"forced-build-id",
			);
		});

		it("FEC-FETCH-007_FE-FETCH-009_redirect_loop_stops_at_max_and_reports_error", async () => {
			setMockLocationRoot();
			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect": "/loop-target",
						},
					}),
				),
			);

			const result = await navigationStateManager.navigate({
				href: "/loop-source",
				navigationType: "userNavigation",
			});

			expect(result.didNavigate).toBe(false);
			expect(vi.mocked(fetch).mock.calls).toHaveLength(10);
			expect(navigationStateManager.getNavigationsSize()).toBe(0);
			expect(
				vi
					.mocked(console.error)
					.mock.calls.some((call) =>
						call.some((arg) =>
							String(arg).includes("Too many redirects"),
						),
					),
			).toBe(true);
		});

		it("FEC-FETCH-008_FE-FETCH-010_FE-FETCH-011_build_id_event_emits_with_ordering_before_redirect_handoff_completes", async () => {
			const buildEvents: Array<{ oldID: string; newID: string }> = [];
			addListener(addBuildIDListener, (event) => {
				buildEvents.push(event.detail as { oldID: string; newID: string });
			});

			let callCount = 0;
			let secondFetchStarted = false;
			let resolveSecondFetch: ((response: Response) => void) | null = null;

			vi.mocked(fetch).mockImplementation(() => {
				callCount++;
				if (callCount === 1) {
					return Promise.resolve(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect":
									"/build-event-redirect-target",
								"X-Vorma-Build-Id": "build-new-id",
							},
						}),
					);
				}
				if (callCount === 2) {
					secondFetchStarted = true;
					return new Promise<Response>((resolve) => {
						resolveSecondFetch = resolve;
					});
				}
				return Promise.resolve(
					createMockResponse(baseRouteData(), {
						headers: {
							"X-Vorma-Build-Id": "build-new-id",
						},
					}),
				);
			});

			const navigatePromise = vormaNavigate("/build-event-source");
			await vi.advanceTimersByTimeAsync(16);

			expect(secondFetchStarted).toBe(true);
			expect(buildEvents).toContainEqual({
				oldID: "1",
				newID: "build-new-id",
			});
			expect(getBuildID()).toBe("build-new-id");

			resolveSecondFetch?.(
				createMockResponse(baseRouteData(), {
					headers: {
						"X-Vorma-Build-Id": "build-new-id",
					},
				}),
			);
			await navigatePromise;
		});

		it("FEC-FETCH-009_FE-FETCH-012_fetch_and_json_failures_leave_page_stable_and_clean_navigation_state", async () => {
			setupGlobalVormaContext({
				buildID: "stable-build",
				hasRootData: true,
				loadersData: ["stable-root"],
				matchedPatterns: ["/stable"],
				params: {},
				splatValues: [],
				importURLs: [],
				exportKeys: [],
			});
			document.title = "Stable Title";
			setMockLocationRoot();

			const routeChangeSpy = vi.fn();
			addListener(addRouteChangeListener, routeChangeSpy);

			vi.mocked(fetch).mockRejectedValueOnce(new Error("network-down"));
			const failedNetworkNav = await navigationStateManager.navigate({
				href: "/network-failure",
				navigationType: "userNavigation",
			});

			expect(failedNetworkNav.didNavigate).toBe(false);
			expect(routeChangeSpy).not.toHaveBeenCalled();
			expect(navigationStateManager.getNavigationsSize()).toBe(0);
			expect(document.title).toBe("Stable Title");
			expect(__vormaClientGlobal.get("loadersData")).toEqual([
				"stable-root",
			]);
			expect(__vormaClientGlobal.get("matchedPatterns")).toEqual([
				"/stable",
			]);

			vi.mocked(fetch).mockResolvedValueOnce(
				new Response("{invalid-json", {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "stable-build",
					},
				}),
			);
			const failedJSONNav = await navigationStateManager.navigate({
				href: "/json-failure",
				navigationType: "userNavigation",
			});

			expect(failedJSONNav.didNavigate).toBe(false);
			expect(routeChangeSpy).not.toHaveBeenCalled();
			expect(navigationStateManager.getNavigationsSize()).toBe(0);
			expect(document.title).toBe("Stable Title");
			expect(__vormaClientGlobal.get("loadersData")).toEqual([
				"stable-root",
			]);
			expect(__vormaClientGlobal.get("matchedPatterns")).toEqual([
				"/stable",
			]);
		});

			it("FEC-FETCH-010_FE-FETCH-013_FE-FETCH-014_FE-FETCH-015_submit_dedup_and_auto_revalidation_and_native_redirect_method_gating_follow_contract", async () => {
			setMockLocationRoot();

			let firstRequestAborted = false;
			let callCount = 0;
			vi.mocked(fetch).mockImplementation((_url, init) => {
				callCount++;
				if (callCount === 1) {
					return new Promise((resolve, reject) => {
						init?.signal?.addEventListener("abort", () => {
							firstRequestAborted = true;
							const error = new Error("aborted");
							error.name = "AbortError";
							reject(error);
						});
					});
				}
				return Promise.resolve(createMockResponse({ ok: true }));
			});

			const dedupedFirst = submit(
				"/api/fetch-dedupe",
				{
					method: "POST",
				},
				{
					dedupeKey: "shared-key",
					revalidate: false,
				},
			);
			const dedupedSecond = submit(
				"/api/fetch-dedupe",
				{
					method: "POST",
				},
				{
					dedupeKey: "shared-key",
					revalidate: false,
				},
			);
			const [firstResult, secondResult] = await Promise.all([
				dedupedFirst,
				dedupedSecond,
			]);
			expect(firstRequestAborted).toBe(true);
			expect(firstResult).toEqual({ success: false, error: "Aborted" });
			expect(secondResult).toEqual({
				success: true,
				data: { ok: true },
			});
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);

			vi.mocked(fetch).mockReset();
			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(createMockResponse({ ok: true })),
			);
			const [noKeyA, noKeyB] = await Promise.all([
				submit(
					"/api/fetch-no-key",
					{
						method: "POST",
					},
					{ revalidate: false },
				),
				submit(
					"/api/fetch-no-key",
					{
						method: "POST",
					},
					{ revalidate: false },
				),
			]);
			expect(noKeyA.success).toBe(true);
			expect(noKeyB.success).toBe(true);
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);

			vi.mocked(fetch).mockReset();
			vi.mocked(fetch)
				.mockResolvedValueOnce(createMockResponse({ mutated: true }))
				.mockResolvedValueOnce(createMockResponse(baseRouteData()));
			await submit("/api/fetch-auto-revalidate", {
				method: "POST",
			});
			expect(vi.mocked(fetch).mock.calls).toHaveLength(2);

			vi.mocked(fetch).mockReset();
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse({ queried: true }),
			);
			await submit("/api/fetch-auto-revalidate-get", {
				method: "GET",
			});
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);

			vi.mocked(fetch).mockReset();
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse({ mutated: true }),
			);
			await submit(
				"/api/fetch-auto-revalidate-disabled",
				{
					method: "POST",
				},
				{ revalidate: false },
			);
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);

			vi.mocked(fetch).mockReset();
			const nativeRedirectPOST = makeRedirectedResponse(
				createMockResponse({ fromPost: "ok" }),
				"http://localhost:3000/native-post-target",
			);
			vi.mocked(fetch).mockResolvedValueOnce(nativeRedirectPOST);
			const postRedirectResult = await submit(
				"/api/fetch-native-redirect-post",
				{
					method: "POST",
				},
				{ revalidate: false },
			);
				expect(postRedirectResult).toEqual({
					success: true,
					data: { fromPost: "ok" },
				});
				expect(vi.mocked(fetch).mock.calls).toHaveLength(1);
			});

			it("FEC-FETCH-011_FE-FETCH-016_submit_includes_deployment_header_when_present", async () => {
				setupGlobalVormaContext({
					deploymentID: "dpl-submit-011",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ ok: true }),
				);

				const result = await submit(
					"/api/deployment-header",
					{
						method: "POST",
					},
					{ revalidate: false },
				);

				expect(result).toEqual({
					success: true,
					data: { ok: true },
				});
				expect(getFetchHeadersAt(0).get("x-deployment-id")).toBe(
					"dpl-submit-011",
				);
			});

			it("FEC-FETCH-012_FE-FETCH-017_stale_build_success_outcome_does_not_mutate_module_map_or_apply_css", async () => {
				setupGlobalVormaContext({
					buildID: "build-current-012",
					clientModuleMap: {
						"/existing": {
							importURL: "/existing.js",
							exportKey: "default",
							errorExportKey: "",
						},
					},
				});

				const applyCSSSpy = vi.spyOn(AssetManager, "applyCSS");
				const json = baseRouteData({
					matchedPatterns: ["/new-pattern"],
					importURLs: ["/new.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					cssBundles: ["/new.css"],
				});
				const response = createMockResponse(json, {
					headers: {
						"X-Vorma-Build-Id": "build-stale-012",
					},
				});

				await navigationStateManager.processSuccessfulNavigation(
					{
						type: "success",
						response,
						json,
						props: {
							href: "/stale-build-target",
							navigationType: "userNavigation",
							redirectCount: 0,
							replace: false,
							scrollToTop: true,
						},
						cssBundlePromises: [],
						waitFnPromise: Promise.resolve({ data: [] }),
					} as any,
					{
						type: "prefetch",
						intent: "none",
						phase: "fetching",
						startTime: Date.now(),
						targetUrl: "http://localhost:3000/stale-build-target",
						originUrl: "http://localhost:3000/",
						scrollToTop: true,
						replace: false,
						state: undefined,
						control: { promise: Promise.resolve(), abortController: null },
					} as any,
				);

				expect(__vormaClientGlobal.get("clientModuleMap")).toEqual({
					"/existing": {
						importURL: "/existing.js",
						exportKey: "default",
						errorExportKey: "",
					},
				});
				expect(applyCSSSpy).not.toHaveBeenCalled();
			});

			it("FEC-FETCH-013_FE-FETCH-018_native_redirect_to_current_url_is_terminal_and_not_replayed", async () => {
				setMockLocationRoot();

				const redirectedToCurrent = makeRedirectedResponse(
					createMockResponse(baseRouteData()),
					"http://localhost:3000/",
				);
				vi.mocked(fetch).mockResolvedValueOnce(redirectedToCurrent);

				const result = await navigationStateManager.navigate({
					href: "/native-current",
					navigationType: "userNavigation",
				});

				expect(result.didNavigate).toBe(false);
				expect(vi.mocked(fetch).mock.calls).toHaveLength(1);
				expect(navigationStateManager.getNavigationsSize()).toBe(0);
			});
		});
	});

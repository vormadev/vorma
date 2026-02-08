import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	vormaNavigate,
} from "./client";
import {
	__registerClientLoaderPattern,
	completeClientLoaders,
	findPartialMatchesOnClient,
	setClientLoadersState,
} from "./client_loaders.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";
import {
	addRouteChangeListener,
} from "./events.ts";
import {
	initClient,
} from "./init_client.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

function makeAbortError() {
	return new DOMException("Aborted", "AbortError");
}

async function flushMicrotasks(times = 3): Promise<void> {
	for (let i = 0; i < times; i++) {
		await Promise.resolve();
	}
}

describeNavigationTestSuite(({ addListener }) => {
	describe("Client loader conformance", () => {
		it("FEC-CL-001_FE-CL-001_FE-CL-002_registration_and_partial_match_fallback_return_longest_registered_parent", async () => {
			setupGlobalVormaContext({
				patternToWaitFnMap: {
					"/parent": vi.fn(),
					"/parent/:id": vi.fn(),
					"/parent/:id/details": vi.fn(),
				},
			});

			await __registerClientLoaderPattern("/parent");
			await __registerClientLoaderPattern("/parent/:id");
			await __registerClientLoaderPattern("/parent/:id/details");

			const result = await findPartialMatchesOnClient(
				"/parent/42/details/extra/segment",
			);

			expect(result).toBeTruthy();
			expect(result?.params).toEqual({ id: "42" });

			const matchedPatterns =
				result?.matches.map(
					(match: any) => match?.registeredPattern?.originalPattern,
				) || [];
			expect(matchedPatterns).toContain("/parent/:id/details");
			expect(matchedPatterns[matchedPatterns.length - 1]).toBe(
				"/parent/:id/details",
			);
		});

		it("FEC-CL-002_FE-CL-003_client_loader_invocation_payload_includes_params_splat_server_data_and_signal", async () => {
			const waitFn = vi.fn().mockResolvedValue("client-loader-data");

			setupGlobalVormaContext({
				patternToWaitFnMap: { "/docs/:id/*": waitFn },
				outermostServerErrorIdx: undefined,
			});

			const result = await completeClientLoaders(
				{
					matchedPatterns: ["/docs/:id/*"],
					loadersData: [{ from: "server" }],
					importURLs: [],
					hasRootData: true,
					params: { id: "7" },
					splatValues: ["a", "b"],
				},
				"build-cl-2",
				new Map(),
				new AbortController().signal,
			);

			expect(waitFn).toHaveBeenCalledTimes(1);
			const payload = waitFn.mock.calls[0]?.[0];
			expect(payload.params).toEqual({ id: "7" });
			expect(payload.splatValues).toEqual(["a", "b"]);
			expect(payload.signal).toBeInstanceOf(AbortSignal);
			expect(await payload.serverDataPromise).toEqual({
				matchedPatterns: ["/docs/:id/*"],
				loaderData: { from: "server" },
				rootData: { from: "server" },
				buildID: "build-cl-2",
			});
			expect(result.data).toEqual(["client-loader-data"]);
		});

		it("FEC-CL-003_FE-CL-004_FE-CL-005_server_error_cutoff_and_child_abort_behavior_are_enforced", async () => {
			{
				const top = vi.fn().mockResolvedValue("top");
				const atServerError = vi.fn().mockResolvedValue("skip-me");
				const childOfServerError = vi.fn().mockResolvedValue("skip-me-too");

				setupGlobalVormaContext({
					patternToWaitFnMap: {
						"/a": top,
						"/a/b": atServerError,
						"/a/b/c": childOfServerError,
					},
					outermostServerErrorIdx: 1,
				});

				const result = await completeClientLoaders(
					{
						matchedPatterns: ["/a", "/a/b", "/a/b/c"],
						loadersData: [{}, {}, {}],
						importURLs: [],
						hasRootData: true,
						params: {},
						splatValues: [],
					},
					"build-cl-3a",
					new Map(),
					new AbortController().signal,
				);

				expect(top).toHaveBeenCalledTimes(1);
				expect(atServerError).not.toHaveBeenCalled();
				expect(childOfServerError).not.toHaveBeenCalled();
				expect(result).toEqual({
					data: ["top", undefined, undefined],
					errorMessage: undefined,
				});
			}

			{
				const top = vi.fn().mockResolvedValue("top-ok");
				const failing = vi
					.fn()
					.mockRejectedValue(new Error("client-loader-boom"));
				let childAborted = false;
				const child = vi.fn().mockImplementation(({ signal }) => {
					return new Promise((_resolve, reject) => {
						if (signal.aborted) {
							childAborted = true;
							reject(makeAbortError());
							return;
						}
						signal.addEventListener(
							"abort",
							() => {
								childAborted = true;
								reject(makeAbortError());
							},
							{ once: true },
						);
					});
				});

				setupGlobalVormaContext({
					patternToWaitFnMap: {
						"/x": top,
						"/x/y": failing,
						"/x/y/z": child,
					},
					outermostServerErrorIdx: undefined,
				});

				const result = await completeClientLoaders(
					{
						matchedPatterns: ["/x", "/x/y", "/x/y/z"],
						loadersData: [{}, {}, {}],
						importURLs: [],
						hasRootData: true,
						params: {},
						splatValues: [],
					},
					"build-cl-3b",
					new Map(),
					new AbortController().signal,
				);

				expect(top).toHaveBeenCalledTimes(1);
				expect(failing).toHaveBeenCalledTimes(1);
				expect(child).toHaveBeenCalledTimes(1);
				expect(childAborted).toBe(true);
				expect(result).toEqual({
					data: ["top-ok", undefined],
					errorMessage: "client-loader-boom",
				});
			}
		});

		it("FEC-CL-004_FE-CL-006_abort_rejections_are_non_fatal_and_not_promoted_to_client_error_message", async () => {
			setupGlobalVormaContext({
				patternToWaitFnMap: {
					"/abort-only": vi.fn().mockRejectedValue(makeAbortError()),
				},
			});

			const result = await completeClientLoaders(
				{
					matchedPatterns: ["/abort-only"],
					loadersData: [{}],
					importURLs: [],
					hasRootData: true,
					params: {},
					splatValues: [],
				},
				"build-cl-4",
				new Map(),
				new AbortController().signal,
			);

			setClientLoadersState(result);
			expect(result.errorMessage).toBeUndefined();
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				undefined,
			]);
			expect(__vormaClientGlobal.get("outermostClientErrorIdx")).toBe(
				undefined,
			);
			expect(__vormaClientGlobal.get("outermostClientError")).toBe(
				undefined,
			);
		});

		it("FEC-CL-005_FE-CL-007_initial_client_loaders_complete_before_first_render", async () => {
			let resolveInitialLoader: (value: string) => void = () => {};
			const initialLoaderPromise = new Promise<string>((resolve) => {
				resolveInitialLoader = resolve;
			});
			const waitFn = vi.fn().mockImplementation(() => initialLoaderPromise);
			const RootComp = () => "root";

			setupGlobalVormaContext({
				hasRootData: true,
				matchedPatterns: ["/"],
				importURLs: ["/cl-root.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ from: "init-server" }],
				params: {},
				splatValues: [],
				publicPathPrefix: "/",
				patternToWaitFnMap: { "/": waitFn },
			});

			vi.doMock("/cl-root.js", () => ({ default: RootComp }));

			const renderFn = vi.fn();
			const initPromise = initClient({
				vormaAppConfig,
				renderFn,
			});

			await vi.waitFor(() => expect(waitFn).toHaveBeenCalledTimes(1));
			expect(waitFn).toHaveBeenCalledTimes(1);
			expect(renderFn).not.toHaveBeenCalled();

			resolveInitialLoader("init-client-data");
			await initPromise;

			expect(renderFn).toHaveBeenCalledTimes(1);
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				"init-client-data",
			]);
		});

		it("FEC-CL-005_FE-CL-008_navigation_waits_for_client_loaders_before_commit_dispatch", async () => {
			setupGlobalVormaContext({
				publicPathPrefix: "/",
			});

			let resolveNavigationLoader: (value: string) => void = () => {};
			const navigationLoaderPromise = new Promise<string>((resolve) => {
				resolveNavigationLoader = resolve;
			});
			const waitFn = vi
				.fn()
				.mockImplementation(() => navigationLoaderPromise);

			__vormaClientGlobal.set("patternToWaitFnMap", { "/next": waitFn });
			vi.doMock("/cl-next.js", () => ({ default: () => "next-comp" }));

			const routeChangeSpy = vi.fn();
			addListener(addRouteChangeListener, routeChangeSpy);

			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					outermostServerError: undefined,
					outermostServerErrorIdx: undefined,
					errorExportKeys: [""],
					matchedPatterns: ["/next"],
					loadersData: [{ from: "server-next" }],
					importURLs: ["/cl-next.js"],
					exportKeys: ["default"],
					hasRootData: true,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
					title: undefined,
					metaHeadEls: undefined,
					restHeadEls: undefined,
				}),
			);

			const navigationPromise = vormaNavigate("/next");
			await vi.waitFor(() => expect(waitFn).toHaveBeenCalledTimes(1));
			expect(waitFn).toHaveBeenCalledTimes(1);
			expect(routeChangeSpy).not.toHaveBeenCalled();

			resolveNavigationLoader("nav-client-data");
			await navigationPromise;
			await vi.runAllTimersAsync();

			expect(routeChangeSpy).toHaveBeenCalledTimes(1);
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				"nav-client-data",
			]);
		});

		it("FEC-CL-006_FE-CL-009_client_loader_error_state_projects_first_true_error_idx_and_message", async () => {
			setupGlobalVormaContext({
				patternToWaitFnMap: {
					"/ok": vi.fn().mockResolvedValue("ok"),
					"/bad": vi
						.fn()
						.mockRejectedValue(new Error("projected-client-error")),
				},
			});

			const result = await completeClientLoaders(
				{
					matchedPatterns: ["/ok", "/bad"],
					loadersData: [{}, {}],
					importURLs: [],
					hasRootData: true,
					params: {},
					splatValues: [],
				},
				"build-cl-6",
				new Map(),
				new AbortController().signal,
			);

			setClientLoadersState(result);
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				"ok",
				undefined,
			]);
			expect(__vormaClientGlobal.get("outermostClientErrorIdx")).toBe(1);
			expect(__vormaClientGlobal.get("outermostClientError")).toBe(
				"projected-client-error",
			);
		});
	});
});

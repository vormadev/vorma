import { describe, expect, it, vi } from "vitest";
import {
	createDeferred,
	createRouteDataResponse,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

async function loadClientAPI() {
	vi.resetModules();
	return import("../../index.ts");
}

describe("client error and edge contracts", () => {
	it("clears navigating state for AbortError failures", async () => {
		const api = await loadClientAPI();
		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		vi.spyOn(window, "fetch").mockRejectedValueOnce(abortError);

		expect(api.getStatus().isNavigating).toBe(false);
		const navPromise = api.vormaNavigate("/abort-nav");
		expect(api.getStatus().isNavigating).toBe(true);

		await navPromise;
		await vi.runAllTimersAsync();

		expect(api.getStatus().isNavigating).toBe(false);
	});

	it("handles non-abort failures by clearing loading state and staying operable", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/stable");
		document.title = "Stable Title";
		vi.spyOn(window, "fetch").mockRejectedValueOnce(
			new Error("Network failure"),
		);

		await api.vormaNavigate("/unreachable");
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/stable");
		expect(document.title).toBe("Stable Title");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Recovered Page" },
			}),
		);
		await api.vormaNavigate("/recovered-non-abort");
		await vi.runAllTimersAsync();
		expect(document.title).toBe("Recovered Page");
	});

	it("does not mutate established router state after a failed navigation", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/stable/:id"],
					loadersData: [{ user: { id: "1" } }],
					hasRootData: true,
					params: { id: "1" },
					splatValues: ["tail"],
					title: { dangerousInnerHTML: "Stable Route" },
				}),
			)
			.mockRejectedValueOnce(new Error("boom"));

		await api.vormaNavigate("/stable/1");
		await vi.runAllTimersAsync();

		const stableRouterData = api.getRouterData();
		const stableTitle = document.title;

		await api.vormaNavigate("/failing-route");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual(stableRouterData);
		expect(document.title).toBe(stableTitle);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("treats empty JSON response as failed and then recovers", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/before-empty");
		document.title = "Before Empty";
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(new Response("", { status: 200 }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Recovered" },
				}),
			);

		await api.vormaNavigate("/empty");
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/before-empty");
		expect(document.title).toBe("Before Empty");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		await api.vormaNavigate("/recovered");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/recovered");
		expect(document.title).toBe("Recovered");
	});

	it("handles non-ok responses and still allows later navigation", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/before-404");
		document.title = "Before 404";
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(new Response("Not Found", { status: 404 }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Success Page" },
				}),
			);

		await api.vormaNavigate("/missing");
		await vi.runAllTimersAsync();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/before-404");
		expect(document.title).toBe("Before 404");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		await api.vormaNavigate("/success");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/success");
		expect(document.title).toBe("Success Page");
	});

	it("does not leak unhandled rejections when user navigation gets a failed response", async () => {
		const api = await loadClientAPI();
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");
		const serverDataPromiseErrors: Array<unknown> = [];

		patternToWaitFnMap["/failed-with-loader"] = async ({
			serverDataPromise,
		}: {
			serverDataPromise: Promise<{ loaderData: { Title: string } }>;
		}) => {
			const { loaderData } = await serverDataPromise.catch((error) => {
				serverDataPromiseErrors.push(error);
				throw error;
			});
			return loaderData.Title;
		};
		await api.__registerClientLoaderPattern("/failed-with-loader");

		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("Server error", { status: 500 }),
		);

		const unhandledRejections: Array<unknown> = [];
		const unhandledRejectionHandler = (reason: unknown) => {
			unhandledRejections.push(reason);
		};
		process.on("unhandledRejection", unhandledRejectionHandler);

		try {
			await api.vormaNavigate("/failed-with-loader");
			await vi.runAllTimersAsync();
			await Promise.resolve();

			expect(unhandledRejections).toEqual([]);
			expect(serverDataPromiseErrors).toHaveLength(1);
			expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
			expect((serverDataPromiseErrors[0] as Error).name).toBe(
				"AbortError",
			);
		} finally {
			process.off("unhandledRejection", unhandledRejectionHandler);
		}
	});

	it("does not leak unhandled rejections when user navigation redirects before loader data is available", async () => {
		const api = await loadClientAPI();
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");
		const serverDataPromiseErrors: Array<unknown> = [];

		patternToWaitFnMap["/redirect-with-loader"] = async ({
			serverDataPromise,
		}: {
			serverDataPromise: Promise<{ loaderData: { Title: string } }>;
		}) => {
			const { loaderData } = await serverDataPromise.catch((error) => {
				serverDataPromiseErrors.push(error);
				throw error;
			});
			return loaderData.Title;
		};
		await api.__registerClientLoaderPattern("/redirect-with-loader");

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/redirect-target" } },
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Redirect Target" },
				}),
			);

		const unhandledRejections: Array<unknown> = [];
		const unhandledRejectionHandler = (reason: unknown) => {
			unhandledRejections.push(reason);
		};
		process.on("unhandledRejection", unhandledRejectionHandler);

		try {
			await api.vormaNavigate("/redirect-with-loader");
			await vi.runAllTimersAsync();
			await Promise.resolve();

			expect(unhandledRejections).toEqual([]);
			expect(serverDataPromiseErrors).toHaveLength(1);
			expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
			expect((serverDataPromiseErrors[0] as Error).name).toBe(
				"AbortError",
			);
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			process.off("unhandledRejection", unhandledRejectionHandler);
		}
	});

	it("rejects client-loader serverDataPromise with AbortError when required server loader payload is missing", async () => {
		const api = await loadClientAPI();
		const pattern = "/missing-server-loader-payload";
		api.__vormaClientGlobal.set("routeManifest", { [pattern]: 1 });
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");
		const serverDataPromiseErrors: Array<unknown> = [];

		patternToWaitFnMap[pattern] = async ({
			serverDataPromise,
		}: {
			serverDataPromise: Promise<{ loaderData: { Title: string } }>;
		}) => {
			const { loaderData } = await serverDataPromise.catch((error) => {
				serverDataPromiseErrors.push(error);
				throw error;
			});
			return loaderData.Title;
		};
		await api.__registerClientLoaderPattern(pattern);

		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: [pattern],
				loadersData: [],
				hasRootData: false,
			}),
		);

		await api.vormaNavigate(pattern);
		await vi.runAllTimersAsync();

		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("does not let stale revalidation override later prefetched navigation", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/");

		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			(input: RequestInfo | URL) => {
				const href =
					typeof input === "string"
						? input
						: input instanceof URL
							? input.href
							: input.url;
				if (href.includes("/about")) {
					return Promise.resolve(
						createRouteDataResponse({
							title: { dangerousInnerHTML: "About Page" },
						}),
					) as any;
				}
				return revalidationDeferred.promise as any;
			},
		);

		const handlers = api.__getPrefetchHandlers({ href: "/about" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		const anchor = document.createElement("a");
		anchor.href = "/about";
		document.body.appendChild(anchor);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
		});
		Object.defineProperty(clickEvent, "target", { value: anchor });

		await handlers?.onClick(clickEvent);
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");

		revalidationDeferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Home Page" },
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		document.body.removeChild(anchor);
	});
});

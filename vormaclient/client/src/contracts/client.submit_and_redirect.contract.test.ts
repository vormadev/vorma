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

describe("client submit/redirect contracts", () => {
	it("deduplicates submissions with the same dedupe key by aborting the first", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;

		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise as any;
			})
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				}),
			);

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(true);
		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		firstDeferred.reject(abortError);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({ success: false, error: "Aborted" });
		expect(secondResult).toEqual({ success: true, data: { ok: true } });
	});

	it("keeps submitting state active through same-key dedupe handoff", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();

		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise as any)
			.mockImplementationOnce(() => secondDeferred.promise as any);

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "handoff-key",
				revalidate: false,
			},
		);
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "handoff-key",
				revalidate: false,
			},
		);

		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		firstDeferred.reject(abortError);

		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		secondDeferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({ success: false, error: "Aborted" });
		expect(secondResult).toEqual({ success: true, data: { ok: true } });
		expect(api.getStatus().isSubmitting).toBe(false);
	});

	it("does not deduplicate submissions with different dedupe keys", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise as any)
			.mockImplementationOnce(() => secondDeferred.promise as any);

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "key-a",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "key-b",
				revalidate: false,
			},
		);

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(api.getStatus().isSubmitting).toBe(true);

		firstDeferred.resolve(
			new Response(JSON.stringify({ id: "a" }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);
		secondDeferred.resolve(
			new Response(JSON.stringify({ id: "b" }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		await Promise.all([firstSubmit, secondSubmit]);
		await vi.runAllTimersAsync();
		expect(api.getStatus().isSubmitting).toBe(false);
	});

	it("does not deduplicate submissions when no dedupe key is provided", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		const signals: AbortSignal[] = [];

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				const signal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				if (signal) signals.push(signal);
				return firstDeferred.promise as any;
			})
			.mockImplementationOnce((_url, init) => {
				const signal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				if (signal) signals.push(signal);
				return secondDeferred.promise as any;
			});

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(signals).toHaveLength(2);
		expect(signals[0]?.aborted).toBe(false);
		expect(signals[1]?.aborted).toBe(false);

		firstDeferred.resolve(
			new Response(JSON.stringify({ id: "one" }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);
		secondDeferred.resolve(
			new Response(JSON.stringify({ id: "two" }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({ success: true, data: { id: "one" } });
		expect(secondResult).toEqual({ success: true, data: { id: "two" } });
		expect(api.getStatus().isSubmitting).toBe(false);
	});

	it("clears submitting state when a deduped replacement submission fails", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();

		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise as any)
			.mockRejectedValueOnce(new Error("Network failure"));

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "cleanup-key",
				revalidate: false,
			},
		);
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		const secondResult = await api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "cleanup-key",
				revalidate: false,
			},
		);

		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		firstDeferred.reject(abortError);
		const firstResult = await firstSubmit;

		await vi.runAllTimersAsync();
		expect(firstResult).toEqual({ success: false, error: "Aborted" });
		expect(secondResult).toEqual({
			success: false,
			error: "Network failure",
		});
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("preserves BodyInit payloads and serializes object bodies for submit requests", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const formData = new FormData();
		formData.append("field", "value");
		await api.submit(
			"/api/form",
			{ method: "POST", body: formData },
			{
				revalidate: false,
			},
		);

		await api.submit(
			"/api/string",
			{ method: "POST", body: "raw-string" },
			{
				revalidate: false,
			},
		);

		const paramsBody = new URLSearchParams({ q: "search", page: "1" });
		await api.submit(
			"/api/params",
			{ method: "POST", body: paramsBody },
			{
				revalidate: false,
			},
		);

		const objectBody = { key: "value", nested: { ok: true } };
		await api.submit(
			"/api/json",
			{ method: "POST", body: objectBody as any },
			{ revalidate: false },
		);

		expect(fetchSpy.mock.calls[0]?.[1]).toEqual(
			expect.objectContaining({ body: formData }),
		);
		expect(fetchSpy.mock.calls[1]?.[1]).toEqual(
			expect.objectContaining({ body: "raw-string" }),
		);
		expect(fetchSpy.mock.calls[2]?.[1]).toEqual(
			expect.objectContaining({ body: paramsBody }),
		);
		expect(fetchSpy.mock.calls[3]?.[1]).toEqual(
			expect.objectContaining({ body: JSON.stringify(objectBody) }),
		);
	});

	it("omits bodies for GET/HEAD/implicit-GET submit requests even when provided", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.submit(
			"/api/get-with-body",
			{ method: "GET", body: "should-not-send" } as any,
			{ revalidate: false },
		);

		await api.submit(
			"/api/head-with-body",
			{
				method: "HEAD",
				body: new URLSearchParams({ q: "ignored" }),
			} as any,
			{ revalidate: false },
		);

		await api.submit(
			"/api/implicit-get-with-body",
			{ body: JSON.stringify({ should: "omit" }) } as any,
			{ revalidate: false },
		);

		const getInit = fetchSpy.mock.calls[0]?.[1] as RequestInit | undefined;
		const headInit = fetchSpy.mock.calls[1]?.[1] as RequestInit | undefined;
		const implicitGetInit = fetchSpy.mock.calls[2]?.[1] as
			| RequestInit
			| undefined;

		expect(getInit?.method).toBe("GET");
		expect(headInit?.method).toBe("HEAD");
		expect(
			Object.prototype.hasOwnProperty.call(getInit ?? {}, "body"),
		).toBe(false);
		expect(
			Object.prototype.hasOwnProperty.call(headInit ?? {}, "body"),
		).toBe(false);
		expect(implicitGetInit?.method).toBeUndefined();
		expect(
			Object.prototype.hasOwnProperty.call(implicitGetInit ?? {}, "body"),
		).toBe(false);
	});

	it("auto-revalidates after non-GET submissions unless disabled", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ updated: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "After Revalidate" },
				}),
			);

		const result = await api.submit("/api/mutate", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: { updated: true } });
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const revalidateURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(revalidateURL.href).toContain("vorma_json=1");
		expect(document.title).toBe("After Revalidate");
	});

	it("does not auto-revalidate after GET submissions", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ data: ["a"] }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const result = await api.submit("/api/search", { method: "GET" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: { data: ["a"] } });
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("follows internal redirect responses from submit", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/after-submit" } },
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "After Submit" },
				}),
			);

		const result = await api.submit("/api/action", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/after-submit");
		expect(document.title).toBe("After Submit");
	});

	it("performs hard reload redirect when submit response includes X-Vorma-Reload", async () => {
		const api = await loadClientAPI();
		let locationHref = window.location.href;
		const originalLocation = window.location;
		Object.defineProperty(window, "location", {
			value: {
				...originalLocation,
				get href() {
					return locationHref;
				},
				set href(value) {
					locationHref = String(value);
				},
			},
			configurable: true,
		});

		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/force-reload",
							"X-Vorma-Build-Id": "reload-build-1",
						},
					},
				),
			);

			const result = await api.submit("/api/action", { method: "POST" });
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(locationHref).toContain("/force-reload");
			expect(locationHref).toContain("vorma_reload=reload-build-1");
		} finally {
			Object.defineProperty(window, "location", {
				value: originalLocation,
				configurable: true,
			});
		}
	});

	it("performs hard redirect for external submit redirect targets", async () => {
		const api = await loadClientAPI();
		let locationHref = window.location.href;
		const originalLocation = window.location;
		Object.defineProperty(window, "location", {
			value: {
				...originalLocation,
				get href() {
					return locationHref;
				},
				set href(value) {
					locationHref = String(value);
				},
			},
			configurable: true,
		});

		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "https://external.com",
						},
					},
				),
			);

			const result = await api.submit("/api/action", { method: "POST" });
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(locationHref).toMatch(/^https:\/\/external\.com\/?$/);
		} finally {
			Object.defineProperty(window, "location", {
				value: originalLocation,
				configurable: true,
			});
		}
	});

	it("prioritizes X-Vorma-Reload over X-Client-Redirect", async () => {
		const api = await loadClientAPI();
		let locationHref = window.location.href;
		const originalLocation = window.location;
		Object.defineProperty(window, "location", {
			value: {
				...originalLocation,
				get href() {
					return locationHref;
				},
				set href(value) {
					locationHref = String(value);
				},
			},
			configurable: true,
		});

		try {
			const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/force-reload-priority",
							"X-Client-Redirect": "/ignored-soft-redirect",
							"X-Vorma-Build-Id": "priority-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/priority-start");
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(locationHref).toContain("/force-reload-priority");
			expect(locationHref).toContain("vorma_reload=priority-build-1");
			expect(locationHref).not.toContain("/ignored-soft-redirect");
		} finally {
			Object.defineProperty(window, "location", {
				value: originalLocation,
				configurable: true,
			});
		}
	});

	it("follows native fetch redirects for non-GET submit requests", async () => {
		const api = await loadClientAPI();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: "http://localhost:3000/native-submit-redirect",
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: {
						dangerousInnerHTML:
							"Native Submit Redirect Destination",
					},
				}),
			);

		const result = await api.submit("/api/native-redirect", {
			method: "POST",
		});
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(secondFetchURL.pathname).toBe("/native-submit-redirect");
		expect(window.location.pathname).toBe("/native-submit-redirect");
		expect(document.title).toBe("Native Submit Redirect Destination");
	});

	it("includes redirect-accept header on navigation and submit fetches", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/header-check");
		await vi.runAllTimersAsync();
		await api.submit(
			"/api/header-check",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		const navHeaders = fetchSpy.mock.calls[0]?.[1]?.headers as Headers;
		const submitHeaders = fetchSpy.mock.calls[1]?.[1]?.headers as Headers;
		expect(navHeaders.get("X-Accepts-Client-Redirect")).toBe("1");
		expect(submitHeaders.get("X-Accepts-Client-Redirect")).toBe("1");
	});

	it("updates build ID and dispatches event before following navigation redirects", async () => {
		const api = await loadClientAPI();
		let buildIdDuringEvent: string | undefined;
		const cleanup = api.addBuildIDListener(() => {
			buildIdDuringEvent = api.getBuildID();
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Build-Id": "redirect-build-22",
							"X-Client-Redirect": "/redirect-target",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "Redirect Target" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "redirect-build-22",
						},
					},
				),
			);

		await api.vormaNavigate("/redirect-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(secondFetchURL.searchParams.get("vorma_json")).toBe(
			"redirect-build-22",
		);
		expect(buildIdDuringEvent).toBeDefined();
		expect(buildIdDuringEvent).toBe("redirect-build-22");
		expect(api.getBuildID()).toBe("redirect-build-22");
		expect(document.title).toBe("Redirect Target");

		cleanup();
	});

	it("ignores non-http redirect targets and continues normal rendering", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{
					title: { dangerousInnerHTML: "Normal Page" },
				},
				{ headers: { "X-Client-Redirect": "mailto:test@example.com" } },
			),
		);

		await api.vormaNavigate("/ignores-mailto");
		await vi.runAllTimersAsync();

		expect(window.location.href).not.toContain("mailto:");
		expect(document.title).toBe("Normal Page");
	});

	it("returns failure results for status and thrown errors", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		fetchSpy.mockResolvedValueOnce(
			new Response(null, {
				status: 500,
				statusText: "Internal Server Error",
			}),
		);
		const serverError = await api.submit(
			"/api/fail",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		expect(serverError).toEqual({ success: false, error: "500" });

		fetchSpy.mockRejectedValueOnce(new Error("Network failure"));
		const networkError = await api.submit(
			"/api/network-fail",
			{ method: "POST" },
			{ revalidate: false },
		);
		expect(networkError).toEqual({
			success: false,
			error: "Network failure",
		});

		const abortFailure = new Error("Aborted");
		abortFailure.name = "AbortError";
		fetchSpy.mockRejectedValueOnce(abortFailure);
		const abortResult = await api.submit(
			"/api/abort",
			{ method: "POST" },
			{ revalidate: false },
		);
		expect(abortResult).toEqual({ success: false, error: "Aborted" });
	});

	it("caps redirect chains and exits loading state", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		for (let i = 0; i < 15; i++) {
			fetchSpy.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": `/redirect-${i}` } },
				),
			);
		}

		await api.vormaNavigate("/redirect-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(10);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/");
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Too many redirects",
		);
	});
});

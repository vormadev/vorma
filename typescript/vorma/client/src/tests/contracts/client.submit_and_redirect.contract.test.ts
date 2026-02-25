import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createDeferred,
	createJSONResponse,
	createRouteDataResponse,
	loadClientAPI,
	setupContractTestSuite,
	stubWindowLocationHref,
	waitForRequestCount,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client submit/redirect contracts", () => {
	it("throws and avoids fetch when submit target is cross-origin", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		await expect(
			api.submit("https://external.example/api", { method: "POST" }),
		).rejects.toThrow("submit(...) only supports same-origin targets.");
		expect(fetchSpy).not.toHaveBeenCalled();
	});

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
			.mockResolvedValueOnce(createJSONResponse({ ok: true }));

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

		secondDeferred.resolve(createJSONResponse({ ok: true }));

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

		firstDeferred.resolve(createJSONResponse({ id: "a" }));
		secondDeferred.resolve(createJSONResponse({ id: "b" }));

		await Promise.all([firstSubmit, secondSubmit]);
		await vi.runAllTimersAsync();
		expect(api.getStatus().isSubmitting).toBe(false);
	});

	it("drops stale redirect side effects when concurrent different-key submits overlap", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		try {
			const submitA = api.submit(
				"/api/resource",
				{ method: "POST" },
				{
					dedupeKey: "key-a",
					revalidate: false,
				},
			);
			const submitB = api.submit(
				"/api/resource",
				{ method: "POST" },
				{
					dedupeKey: "key-b",
					revalidate: false,
				},
			);

			await waitForRequestCount({ requests, count: 2 });
			expect(api.getStatus().isSubmitting).toBe(true);

			// A submit returns redirect and starts redirect navigation A.
			requests[0]!.resolve(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/redirect-a" } },
				),
			);
			await waitForRequestCount({ requests, count: 3 });

			// B submit returns redirect and should supersede redirect navigation A.
			requests[1]!.resolve(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/redirect-b" } },
				),
			);
			await waitForRequestCount({ requests, count: 4 });

			// Resolve stale A redirect target late with unique build ID.
			requests[2]!.resolve(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "Redirect A (Stale)" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "stale-redirect-a-build",
						},
					},
				),
			);

			// Resolve B redirect target after, this must be the visible winner.
			requests[3]!.resolve(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "Redirect B (Winner)" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "winner-redirect-b-build",
						},
					},
				),
			);

			const [resultA, resultB] = await Promise.all([submitA, submitB]);
			await vi.runAllTimersAsync();

			expect(resultA).toEqual({
				success: false,
				error: "Redirect failed",
			});
			expect(resultB).toEqual({ success: true, data: undefined });
			expect(window.location.pathname).toBe("/redirect-b");
			expect(document.title).toBe("Redirect B (Winner)");
			expect(api.getBuildID()).toBe("winner-redirect-b-build");
			expect(
				buildIDEvents.some(
					(event) => event.newID === "stale-redirect-a-build",
				),
			).toBe(false);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("ignores stale submit-redirect side effects when a newer user navigation commits first", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		try {
			const submitPromise = api.submit("/api/submit-redirect", {
				method: "POST",
			});

			await waitForRequestCount({ requests, count: 1 });
			requests[0]!.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/submit-redirect-target",
						},
					},
				),
			);
			await waitForRequestCount({ requests, count: 2 });

			const userNavigationPromise = api.vormaNavigate(
				"/user-navigation-winner",
			);
			await waitForRequestCount({ requests, count: 3 });

			requests[2]!.resolve(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "User Navigation Winner" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "user-navigation-winner-build",
						},
					},
				),
			);
			await userNavigationPromise;

			requests[1]!.resolve(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "Submit Redirect Stale" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "submit-redirect-stale-build",
						},
					},
				),
			);

			await submitPromise;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/user-navigation-winner");
			expect(document.title).toBe("User Navigation Winner");
			expect(api.getBuildID()).toBe("user-navigation-winner-build");
			expect(
				buildIDEvents.some(
					(event) => event.newID === "submit-redirect-stale-build",
				),
			).toBe(false);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("follows submit redirect headers even when submit response status is non-OK", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						status: 409,
						headers: {
							"X-Client-Redirect": "/submit-redirect-non-ok",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: {
						dangerousInnerHTML: "Submit Redirect Non-OK Winner",
					},
				}),
			);

		const result = await api.submit("/api/action", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/submit-redirect-non-ok");
		expect(document.title).toBe("Submit Redirect Non-OK Winner");
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

		firstDeferred.resolve(createJSONResponse({ id: "one" }));
		secondDeferred.resolve(createJSONResponse({ id: "two" }));

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

	it("ignores late stale deduped submit redirect responses after replacement submission wins", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise as any;
			})
			.mockResolvedValueOnce(createJSONResponse({ ok: "second" }));

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const firstSubmit = api.submit(
					"/api/resource",
					{ method: "POST" },
					{
						dedupeKey: "stale-submit",
						revalidate: false,
					},
				);

				const secondSubmit = api.submit(
					"/api/resource",
					{ method: "POST" },
					{
						dedupeKey: "stale-submit",
						revalidate: false,
					},
				);

				expect(firstSignal?.aborted).toBe(true);

				const secondResult = await secondSubmit;
				expect(secondResult).toEqual({
					success: true,
					data: { ok: "second" },
				});

				firstDeferred.resolve(
					createRouteDataResponse(
						{},
						{ headers: { "X-Client-Redirect": "/stale-redirect" } },
					),
				);
				const firstResult = await firstSubmit;
				expect(firstResult).toEqual({
					success: false,
					error: "Aborted",
				});

				await vi.runAllTimersAsync();
			},
		});

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("ignores late stale deduped submit hard-reload responses after replacement submission wins", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		const locationHrefStub = stubWindowLocationHref();

		try {
			vi.spyOn(window, "fetch")
				.mockImplementationOnce((_url, init) => {
					firstSignal = (init as RequestInit | undefined)
						?.signal as AbortSignal;
					return firstDeferred.promise as any;
				})
				.mockResolvedValueOnce(createJSONResponse({ ok: "second" }));

			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						const firstSubmit = api.submit(
							"/api/resource",
							{ method: "POST" },
							{
								dedupeKey: "stale-hard-reload",
								revalidate: false,
							},
						);

						const secondSubmit = api.submit(
							"/api/resource",
							{ method: "POST" },
							{
								dedupeKey: "stale-hard-reload",
								revalidate: false,
							},
						);

						expect(firstSignal?.aborted).toBe(true);

						const secondResult = await secondSubmit;
						expect(secondResult).toEqual({
							success: true,
							data: { ok: "second" },
						});

						firstDeferred.resolve(
							createRouteDataResponse(
								{},
								{
									headers: {
										"X-Vorma-Reload": "/stale-reload",
										"X-Vorma-Build-Id":
											"stale-reload-build",
									},
								},
							),
						);
						const firstResult = await firstSubmit;
						expect(firstResult).toEqual({
							success: false,
							error: "Aborted",
						});

						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(locationHrefStub.getHref()).not.toContain("/stale-reload");
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			locationHrefStub.restore();
		}
	});

	it("does not trigger revalidation from a late stale deduped submit response", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise as any;
			})
			.mockResolvedValueOnce(createJSONResponse({ ok: "second" }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Stale Revalidate Applied" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "stale-submit-revalidate",
			},
		);
		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "stale-submit-revalidate",
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(true);

		const secondResult = await secondSubmit;
		expect(secondResult).toEqual({
			success: true,
			data: { ok: "second" },
		});

		firstDeferred.resolve(createJSONResponse({ stale: true }));

		const firstResult = await firstSubmit;
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(document.title).not.toBe("Stale Revalidate Applied");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("aborts stale deduped submits that are superseded during JSON parsing", async () => {
		const api = await loadClientAPI();
		const firstJSONDeferred = createDeferred<unknown>();
		let firstSignal: AbortSignal | undefined;
		const firstResponse = createJSONResponse({ stale: true });

		Object.defineProperty(firstResponse, "json", {
			value: vi.fn(() => firstJSONDeferred.promise),
			configurable: true,
		});

		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return Promise.resolve(firstResponse) as any;
			})
			.mockResolvedValueOnce(createJSONResponse({ fresh: true }));

		const firstSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "stale-json-parse",
				revalidate: false,
			},
		);
		await Promise.resolve();
		await Promise.resolve();

		const secondSubmit = api.submit(
			"/api/resource",
			{ method: "POST" },
			{
				dedupeKey: "stale-json-parse",
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(true);
		const secondResult = await secondSubmit;

		firstJSONDeferred.resolve({ stale: true });
		const firstResult = await firstSubmit;

		expect(secondResult).toEqual({
			success: true,
			data: { fresh: true },
		});
		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
	});

	it("does not allow late stale deduped submits to change build ID", async () => {
		const api = await loadClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		const buildIDListener = vi.fn();
		const cleanupBuildIDListener = api.addBuildIDListener(buildIDListener);

		try {
			vi.spyOn(window, "fetch")
				.mockImplementationOnce((_url, init) => {
					firstSignal = (init as RequestInit | undefined)
						?.signal as AbortSignal;
					return firstDeferred.promise as any;
				})
				.mockResolvedValueOnce(createJSONResponse({ ok: "second" }));

			const firstSubmit = api.submit(
				"/api/resource",
				{ method: "POST" },
				{
					dedupeKey: "stale-build-id",
					revalidate: false,
				},
			);
			const secondSubmit = api.submit(
				"/api/resource",
				{ method: "POST" },
				{
					dedupeKey: "stale-build-id",
					revalidate: false,
				},
			);

			expect(firstSignal?.aborted).toBe(true);

			const secondResult = await secondSubmit;
			expect(secondResult).toEqual({
				success: true,
				data: { ok: "second" },
			});
			expect(api.getBuildID()).toBe("1");

			firstDeferred.resolve(
				createJSONResponse(
					{ stale: true },
					{
						headers: {
							"X-Vorma-Build-Id": "stale-build-id-999",
						},
					},
				),
			);
			const firstResult = await firstSubmit;
			await vi.runAllTimersAsync();

			expect(firstResult).toEqual({
				success: false,
				error: "Aborted",
			});
			expect(api.getBuildID()).toBe("1");
			expect(buildIDListener).not.toHaveBeenCalled();
		} finally {
			cleanupBuildIDListener();
		}
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
			.mockResolvedValueOnce(createJSONResponse({ updated: true }))
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
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createJSONResponse({ data: ["a"] }));

		const result = await api.submit("/api/search", { method: "GET" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: { data: ["a"] } });
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("returns success with undefined data for 204 non-GET submit responses", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 204,
				headers: {
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const result = await api.submit(
			"/api/no-content-submit",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("returns success with undefined data for null-body 200 submit responses without content type", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 200,
				headers: {
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const result = await api.submit(
			"/api/empty-200-submit",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("returns text data for successful non-JSON submit responses", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("plain-ok", {
				status: 200,
				headers: {
					"Content-Type": "text/plain",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		const result = await api.submit(
			"/api/text-submit",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: "plain-ok" });
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

	it("does not fetch route data for submit redirects that only change hash", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/after-submit");
		api.getUnsafeHistoryInstance();

		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse(
				{},
				{
					headers: {
						"X-Client-Redirect": "/after-submit#details",
					},
				},
			),
		);

		const result = await api.submit("/api/action", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: undefined });
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/after-submit");
		expect(window.location.hash).toBe("#details");
	});

	it("does not re-follow submit redirects to the current path", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/after-submit");
		document.title = "Before Submit";

		api.__vormaClientGlobal.set("matchedPatterns", ["/after-submit"]);
		api.__vormaClientGlobal.set("loadersData", [{ stale: true }]);
		api.__vormaClientGlobal.set("params", {});
		api.__vormaClientGlobal.set("splatValues", []);
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/after-submit" } },
				),
			);

		const result = await api.submit("/api/action", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result.success).toBe(true);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(document.title).toBe("Before Submit");
	});

	it("returns explicit error when submit soft redirect navigation fails", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/after-submit" } },
				),
			)
			.mockRejectedValueOnce(
				new Error("Redirect navigation request failed"),
			);

		const result = await api.submit("/api/action", { method: "POST" });
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: false,
			error: "Redirect failed",
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("performs hard reload redirect when submit response includes X-Vorma-Reload", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

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
			expect(locationHrefStub.getHref()).toContain("/force-reload");
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=reload-build-1",
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("performs hard redirect for external submit redirect targets", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

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
			expect(locationHrefStub.getHref()).toMatch(
				/^https:\/\/external\.com\/?$/,
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("prioritizes X-Vorma-Reload over X-Client-Redirect", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

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
			expect(locationHrefStub.getHref()).toContain(
				"/force-reload-priority",
			);
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=priority-build-1",
			);
			expect(locationHrefStub.getHref()).not.toContain(
				"/ignored-soft-redirect",
			);
		} finally {
			locationHrefStub.restore();
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

	it("includes x-deployment-id on submit requests when deployment ID is configured", async () => {
		const api = await loadClientAPI();
		api.__vormaClientGlobal.set("deploymentID", "deploy-42");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.submit(
			"/api/with-deployment",
			{
				method: "POST",
				headers: { "X-Test-Header": "kept" },
				body: JSON.stringify({ ok: true }),
			},
			{ revalidate: false },
		);

		const submitHeaders = fetchSpy.mock.calls[0]?.[1]?.headers as Headers;
		expect(submitHeaders.get("x-deployment-id")).toBe("deploy-42");
		expect(submitHeaders.get("X-Test-Header")).toBe("kept");
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

	it("treats non-http redirect targets as explicit navigation errors", async () => {
		const api = await loadClientAPI();
		document.title = "Before Redirect Error";
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
		expect(document.title).toBe("Before Redirect Error");
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

		fetchSpy.mockRejectedValueOnce("not-an-error-object");
		const unknownThrownResult = await api.submit(
			"/api/non-error-throw",
			{ method: "POST" },
			{ revalidate: false },
		);
		expect(unknownThrownResult).toEqual({
			success: false,
			error: "Unknown error",
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

import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createJSONResponse,
	createRouteDataResponse,
	expectStatusIdle,
	loadClientAPI,
	setupContractTestSuite,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

function asURL(input: RequestInfo | URL): URL {
	if (input instanceof URL) {
		return input;
	}
	if (typeof input === "string") {
		return new URL(input, window.location.href);
	}
	return new URL(input.url, window.location.href);
}

async function waitForRequestCount(props: {
	requests: unknown[];
	count: number;
}): Promise<void> {
	const { requests, count } = props;
	for (let i = 0; i < 300; i++) {
		if (requests.length >= count) {
			return;
		}
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(1);
	}
	throw new Error(`Timed out waiting for request count ${count}`);
}

function routeTitle(title: string): Response {
	return createRouteDataResponse({
		title: { dangerousInnerHTML: title },
	});
}

describe("client navigation state machine contracts", () => {
	it.each([
		[[2, 1, 0]],
		[[1, 2, 0]],
		[[0, 2, 1]],
	])(
		"keeps last-started user navigation authoritative despite late earlier completions (resolve order: %j)",
		async (resolveOrder) => {
			const api = await loadClientAPI();
			const { requests } = createAbortAwareFetchRecorder();

			const { unhandledRejections } = await withUnhandledRejectionCapture({
				run: async () => {
					const n1 = api.vormaNavigate("/model-a");
					const n2 = api.vormaNavigate("/model-b");
					const n3 = api.vormaNavigate("/model-c");

					await waitForRequestCount({ requests, count: 3 });

					const responses = [
						routeTitle("Model A"),
						routeTitle("Model B"),
						routeTitle("Model C"),
					];

					for (const idx of resolveOrder) {
						const request = requests[idx];
						if (!request) {
							throw new Error(
								`Missing request for resolve index ${idx}`,
							);
						}
						request.resolve(responses[idx]!);
						await Promise.resolve();
					}

					await Promise.all([n1, n2, n3]);
				},
			});

			expect(unhandledRejections).toEqual([]);
			expect(window.location.pathname).toBe("/model-c");
			expect(document.title).toBe("Model C");
			expectStatusIdle(api.getStatus());
		},
	);

	it("preserves invariants across mixed prefetch/navigate/submit/revalidate sequences", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } = await withUnhandledRejectionCapture(
			{
				run: async () => {
					// 1) Start prefetch and then navigate to same data target:
					// navigation should reuse in-flight work, not spawn another fetch.
					const handlers = api.__getPrefetchHandlers({
						href: "/prefetch-seed#one",
					});
					handlers?.start(new Event("mouseenter"));
					await vi.advanceTimersByTimeAsync(100);
					await waitForRequestCount({ requests, count: 1 });
					const prefetchRequest = requests[0]!;

					const navFromPrefetch = api.vormaNavigate("/prefetch-seed#two");
					await Promise.resolve();
					expect(requests).toHaveLength(1);

					prefetchRequest.resolve(routeTitle("Prefetch Seed"));
					await navFromPrefetch;

					// 2) Start two user navigations; late completion from first
					// must not clobber second.
					const navFirst = api.vormaNavigate("/race-first");
					await waitForRequestCount({ requests, count: 2 });
					const firstRequest = requests[1]!;

					const navSecond = api.vormaNavigate("/race-second");
					await waitForRequestCount({ requests, count: 3 });
					const secondRequest = requests[2]!;
					expect(api.getStatus().isNavigating).toBe(true);

					secondRequest.resolve(routeTitle("Race Second"));
					await navSecond;

					firstRequest.resolve(routeTitle("Race First Stale"));
					await navFirst;

					expect(window.location.pathname).toBe("/race-second");
					expect(document.title).toBe("Race Second");

					// 3) Same-key dedupe submit: first result must abort, second
					// succeeds and triggers revalidation fetch.
					const submitOne = api.submit(
						"/mutation",
						{ method: "POST", body: JSON.stringify({ step: 1 }) },
						{ dedupeKey: "model-seq" },
					);
					await waitForRequestCount({ requests, count: 4 });

					const submitTwo = api.submit(
						"/mutation",
						{ method: "POST", body: JSON.stringify({ step: 2 }) },
						{ dedupeKey: "model-seq" },
					);
					await waitForRequestCount({ requests, count: 5 });
					expect(api.getStatus().isSubmitting).toBe(true);

					const submitTwoRequest = requests[4]!;
					const submitTwoURL = asURL(submitTwoRequest.input);
					expect(submitTwoURL.pathname).toBe("/mutation");
					expect(submitTwoRequest.init?.method).toBe("POST");

					submitTwoRequest.resolve(createJSONResponse({ ok: true }));

					await waitForRequestCount({ requests, count: 6 });
					const revalidationRequest = requests[5]!;
					const revalidationURL = asURL(revalidationRequest.input);
					expect(revalidationURL.pathname).toBe("/race-second");
					expect(revalidationURL.searchParams.get("vorma_json")).toBe("1");

					// 4) Navigate away and then complete stale revalidation late:
					// late revalidation must not overwrite newer destination.
					const navThird = api.vormaNavigate("/race-third");
					await waitForRequestCount({ requests, count: 7 });
					const navThirdRequest = requests[6]!;

					navThirdRequest.resolve(routeTitle("Race Third"));
					await navThird;

					revalidationRequest.resolve(routeTitle("Stale Revalidation"));

					const [submitOneResult, submitTwoResult] = await Promise.all([
						submitOne,
						submitTwo,
					]);

					return { submitOneResult, submitTwoResult };
				},
			},
		);

		expect(unhandledRejections).toEqual([]);
		expect(result.submitOneResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(result.submitTwoResult).toEqual({
			success: true,
			data: { ok: true },
		});
		expect(window.location.pathname).toBe("/race-third");
		expect(document.title).toBe("Race Third");
		expectStatusIdle(api.getStatus());
	});
});

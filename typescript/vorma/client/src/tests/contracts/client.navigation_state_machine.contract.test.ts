import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createJSONResponse,
	createRouteDataResponse,
	expectStatusIdle,
	loadClientAPI,
	requestInputToURL,
	setupContractTestSuite,
	waitForRequestCount,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

function routeTitle(title: string): Response {
	return createRouteDataResponse({
		title: { dangerousInnerHTML: title },
	});
}

function createSeededRandom(seed: number): () => number {
	let state = seed >>> 0;
	return () => {
		state = (state * 1664525 + 1013904223) >>> 0;
		return state / 0x100000000;
	};
}

function shuffledIndices(props: { length: number; seed: number }): number[] {
	const { length, seed } = props;
	const random = createSeededRandom(seed);
	const indices = Array.from({ length }, (_, index) => index);
	for (let i = indices.length - 1; i > 0; i--) {
		const j = Math.floor(random() * (i + 1));
		const valueAtI = indices[i];
		indices[i] = indices[j]!;
		indices[j] = valueAtI!;
	}
	return indices;
}

type GeneratedOperation =
	| {
			kind: "prefetch";
			href: string;
	  }
	| {
			kind: "navigate";
			href: string;
	  };

function buildGeneratedNavigationModelSequence(seed: number): {
	operations: GeneratedOperation[];
	finalNavigationHref: string;
} {
	const random = createSeededRandom(seed);
	const operations: GeneratedOperation[] = [];
	const basePathnames = ["/model-seq-a", "/model-seq-b", "/model-seq-c"];
	const generatedCount = 14;

	for (let i = 0; i < generatedCount; i++) {
		const pathname =
			basePathnames[Math.floor(random() * basePathnames.length)]!;
		const hash = random() < 0.5 ? "" : `#h${Math.floor(random() * 4) + 1}`;
		const href = `${pathname}${hash}`;
		const kind = random() < 0.5 ? "prefetch" : "navigate";
		operations.push({ kind, href } as GeneratedOperation);
	}

	const finalNavigationHref = `/model-seq-final-${seed}#winner`;
	operations.push({
		kind: "navigate",
		href: finalNavigationHref,
	});

	return { operations, finalNavigationHref };
}

describe("client navigation state machine contracts", () => {
	it.each([[[2, 1, 0]], [[1, 2, 0]], [[0, 2, 1]]])(
		"keeps last-started user navigation authoritative despite late earlier completions (resolve order: %j)",
		async (resolveOrder) => {
			const api = await loadClientAPI();
			const { requests } = createAbortAwareFetchRecorder();

			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
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
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(window.location.pathname).toBe("/model-c");
			expect(document.title).toBe("Model C");
			expectStatusIdle(api.getStatus());
		},
	);

	it.each([11, 29, 47, 83, 131])(
		"keeps last-started navigation authoritative across generated resolve permutations (seed=%d)",
		async (seed) => {
			const api = await loadClientAPI();
			const { requests } = createAbortAwareFetchRecorder();

			const { result, unhandledRejections } =
				await withUnhandledRejectionCapture({
					run: async () => {
						const navigationCount = 6;
						const navigationPromises: Array<Promise<void>> = [];

						for (let i = 0; i < navigationCount; i++) {
							navigationPromises.push(
								api.vormaNavigate(`/generated-${seed}-${i}`),
							);
						}

						await waitForRequestCount({
							requests,
							count: navigationCount,
						});

						const resolveOrder = shuffledIndices({
							length: navigationCount,
							seed,
						});
						for (const requestIndex of resolveOrder) {
							const request = requests[requestIndex];
							if (!request) {
								throw new Error(
									`Missing generated request at index ${requestIndex}`,
								);
							}
							request.resolve(
								routeTitle(`Generated ${requestIndex}`),
							);
							await Promise.resolve();
						}

						await Promise.all(navigationPromises);
						return {
							expectedPathname: `/generated-${seed}-${navigationCount - 1}`,
							expectedTitle: `Generated ${navigationCount - 1}`,
						};
					},
				});

			expect(unhandledRejections).toEqual([]);
			expect(window.location.pathname).toBe(result.expectedPathname);
			expect(document.title).toBe(result.expectedTitle);
			expectStatusIdle(api.getStatus());
		},
	);

	it("preserves invariants across mixed prefetch/navigate/submit/revalidate sequences", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
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

					const navFromPrefetch =
						api.vormaNavigate("/prefetch-seed#two");
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
					const submitTwoURL = requestInputToURL(
						submitTwoRequest.input,
					);
					expect(submitTwoURL.pathname).toBe("/mutation");
					expect(submitTwoRequest.init?.method).toBe("POST");

					submitTwoRequest.resolve(createJSONResponse({ ok: true }));

					await waitForRequestCount({ requests, count: 6 });
					const revalidationRequest = requests[5]!;
					const revalidationURL = requestInputToURL(
						revalidationRequest.input,
					);
					expect(revalidationURL.pathname).toBe("/race-second");
					expect(revalidationURL.searchParams.get("vorma_json")).toBe(
						"1",
					);

					// 4) Navigate away and then complete stale revalidation late:
					// late revalidation must not overwrite newer destination.
					const navThird = api.vormaNavigate("/race-third");
					await waitForRequestCount({ requests, count: 7 });
					const navThirdRequest = requests[6]!;

					navThirdRequest.resolve(routeTitle("Race Third"));
					await navThird;

					revalidationRequest.resolve(
						routeTitle("Stale Revalidation"),
					);

					const [submitOneResult, submitTwoResult] =
						await Promise.all([submitOne, submitTwo]);

					return { submitOneResult, submitTwoResult };
				},
			});

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

	it("preserves last-navigation authority across a seeded randomized model sequence", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();
		const modelSeed = 20260212;
		const { operations, finalNavigationHref } =
			buildGeneratedNavigationModelSequence(modelSeed);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const navigationPromises: Array<Promise<void>> = [];

				for (const operation of operations) {
					if (operation.kind === "prefetch") {
						const handlers = api.__getPrefetchHandlers({
							href: operation.href,
							delayMs: 0,
						});
						handlers?.start(new Event("mouseenter"));
						await vi.advanceTimersByTimeAsync(1);
						continue;
					}

					navigationPromises.push(api.vormaNavigate(operation.href));
					await Promise.resolve();
				}

				await vi.advanceTimersByTimeAsync(20);
				await Promise.resolve();
				expect(requests.length).toBeGreaterThan(0);

				const resolveOrder = shuffledIndices({
					length: requests.length,
					seed: modelSeed ^ 0x5a5a,
				});
				for (const requestIndex of resolveOrder) {
					const request = requests[requestIndex];
					if (!request) {
						throw new Error(
							`Missing randomized model request at index ${requestIndex}`,
						);
					}

					const requestURL = requestInputToURL(request.input);
					request.resolve(
						routeTitle(`Model Sequence ${requestURL.pathname}`),
					);
					await Promise.resolve();
				}

				await Promise.all(navigationPromises);
			},
		});

		const expectedFinalURL = new URL(
			finalNavigationHref,
			window.location.origin,
		);

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe(expectedFinalURL.pathname);
		expect(window.location.hash).toBe(expectedFinalURL.hash);
		expect(document.title).toBe(
			`Model Sequence ${expectedFinalURL.pathname}`,
		);
		expectStatusIdle(api.getStatus());
	});

	it("records terminal lifecycle states with explicit reasons for settled operations", async () => {
		const api = await loadClientAPI();
		api.__clearNavigationDebugJournal();
		const { requests } = createAbortAwareFetchRecorder();

		const prefetchHandlers = api.__getPrefetchHandlers({
			href: "/journal-prefetch",
			delayMs: 0,
		});
		prefetchHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		await waitForRequestCount({ requests, count: 1 });

		const navigatePromise = api.vormaNavigate("/journal-target");
		await waitForRequestCount({ requests, count: 2 });
		requests[1]?.resolve(routeTitle("Journal Target"));
		await navigatePromise;
		requests[0]?.resolve(routeTitle("Journal Prefetch Late"));

		const submitPromise = api.submit(
			"/journal-submit",
			{ method: "POST" },
			{ revalidate: false },
		);
		await waitForRequestCount({ requests, count: 3 });
		requests[2]?.resolve(createJSONResponse({ ok: true }));
		await submitPromise;

		await vi.runAllTimersAsync();
		prefetchHandlers?.stop();

		const journal = api.__getNavigationDebugJournal();
		expect(journal.length).toBeGreaterThan(0);
		expect(
			journal.every(
				(entry) =>
					typeof entry.reason === "string" && entry.reason.length > 0,
			),
		).toBe(true);

		const terminalToStates = new Set([
			"removed",
			"complete",
			"failed",
			"aborted",
		]);
		const latestEntryByOperationID = new Map<number, (typeof journal)[0]>();
		for (const entry of journal) {
			if (entry.operationID !== null) {
				latestEntryByOperationID.set(entry.operationID, entry);
			}
		}
		expect(latestEntryByOperationID.size).toBeGreaterThan(0);
		for (const latestEntry of latestEntryByOperationID.values()) {
			expect(terminalToStates.has(latestEntry.toState)).toBe(true);
		}

		expectStatusIdle(api.getStatus());
	});
});

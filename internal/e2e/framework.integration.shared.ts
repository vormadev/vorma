import * as fs from "node:fs/promises";
import * as path from "node:path";
import { expect, test, type Page } from "@playwright/test";
import {
	startFixtureSiteForE2E,
	type E2ERunMode,
	type E2EUIAdapter,
	type RunningFixtureSite,
} from "./runtime_harness.ts";

/////////////////////////////////////////////////////////////////////
/////// Types
/////////////////////////////////////////////////////////////////////

type UserRouteCase = {
	caseLabel: string;
	userID: string;
	queryValue: string;
};

type BridgeRandomizedSequenceSummary = {
	expectedCount: number;
	failedSteps: number;
	completedSteps: number;
	endingPathname: string;
};

type BridgeConcurrencySummary = {
	mutationFailureCount: number;
	rejectedNavigationCount: number;
	highestCount: number;
	mutationResultCount: number;
};

type BridgeNetworkChaosSummary = {
	failedMutations: number;
	failedNavigations: number;
	completedOperations: number;
	expectedCount: number;
	minimumPossibleCount: number;
	maximumPossibleCount: number;
};

type BridgeSharedSessionSummary = {
	successfulMutations: number;
	highestCount: number;
	endingPathname: string;
};

/////////////////////////////////////////////////////////////////////
/////// Constants
/////////////////////////////////////////////////////////////////////

const randomizedSweepSeedCount = 3;
const randomizedSweepStepCount = 48;
const randomizedSoakStepCount = 96;
const concurrencyBurstMutationCount = 24;
const networkChaosOperationCount = 60;
const sharedSessionOperationCount = 42;
const hmrProbeBaselineToken = "hmr-probe-baseline";

const userRouteCases: ReadonlyArray<UserRouteCase> = [
	{ caseLabel: "plain", userID: "alpha", queryValue: "first" },
	{
		caseLabel: "mixed-char",
		userID: "UPPER_123",
		queryValue: "space value",
	},
	{
		caseLabel: "symbols",
		userID: "mix-ed_value42",
		queryValue: "symbols-[]{}!@",
	},
];

/////////////////////////////////////////////////////////////////////
/////// Test Matrix
/////////////////////////////////////////////////////////////////////

export function defineFrameworkIntegrationSuite(input: {
	runMode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
	suiteName: string;
}): void {
	const runMode = input.runMode;
	const uiAdapter = input.uiAdapter;
	test.describe(input.suiteName, () => {
		let runningFixtureSite: RunningFixtureSite | null = null;

		function mustGetRunningFixtureSite(): RunningFixtureSite {
			if (runningFixtureSite === null) {
				throw new Error(
					"fixture site must be started before test execution",
				);
			}
			return runningFixtureSite;
		}

		test.beforeAll(async () => {
			runningFixtureSite = await startFixtureSiteForE2E({
				mode: runMode,
				uiAdapter,
			});
		});

		test.afterAll(async () => {
			if (runningFixtureSite === null) {
				return;
			}
			await runningFixtureSite.stop();
		});

		test.beforeEach(async () => {
			await resetCounterStateForFixture({
				runningFixtureSite: mustGetRunningFixtureSite(),
			});
		});

		test.afterEach(async ({ page: _unusedPage }, testInfo) => {
			void _unusedPage;
			if (runningFixtureSite === null) {
				return;
			}
			if (testInfo.status === testInfo.expectedStatus) {
				return;
			}

			await testInfo.attach("fixture-recent-output.log", {
				body: Buffer.from(
					runningFixtureSite.readRecentOutput(),
					"utf8",
				),
				contentType: "text/plain",
			});
		});

		test("renders root and nested index loader data", async ({ page }) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/",
			});

			await expect(page.locator("#e2e-runtime-mode")).toHaveText(runMode);
			await expect(page.locator("#e2e-runtime-adapter")).toHaveText(
				uiAdapter,
			);
			await expect(page.locator("#e2e-home-message")).toHaveText(
				"Framework E2E stress harness",
			);
			await expect(page.locator("#e2e-home-count")).toHaveText("0");
			await assertRuntimeStatusIdle({ page });
		});

		test("resolves dynamic user routes with decoded query values", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			for (const userRouteCase of userRouteCases) {
				await visitFixturePath({
					page,
					runningFixtureSite: activeFixtureSite,
					pathname: buildUserRoutePath({
						userID: userRouteCase.userID,
						queryValue: userRouteCase.queryValue,
					}),
				});

				await expect(
					page.locator("#e2e-user-id"),
					userRouteCase.caseLabel,
				).toHaveText(userRouteCase.userID);
				await expect(
					page.locator("#e2e-user-query"),
					userRouteCase.caseLabel,
				).toHaveText(userRouteCase.queryValue);
				await expect(
					page.locator("#e2e-user-mode"),
					userRouteCase.caseLabel,
				).toHaveText(runMode);
			}
			await assertRuntimeStatusIdle({ page });
		});

		test("mutations persist server state across client-side navigation", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/mutation-lab",
			});

			await page.click("#e2e-increment-button");
			await page.click("#e2e-increment-button");
			await expect(page.locator("#e2e-increment-result")).toHaveText("2");

			await page.click("#e2e-nav-home");
			await expect(page.locator("#e2e-home-count")).toHaveText("2");
			await assertRuntimeStatusIdle({ page });
		});

		test("surfaces backend loader failures through route error boundaries", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/explode",
			});

			await expect(page.locator("#e2e-explode-boundary")).toContainText(
				"explode-boundary:",
			);
			await expect(page.locator("#e2e-explode-unreachable")).toHaveCount(
				0,
			);
			await assertRuntimeStatusIdle({ page });
		});

		test("keeps latest navigation authoritative during slow/fast race", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/navigation-race",
			});

			await page.click("#e2e-run-nav-race-slow-then-fast");
			await expect(page).toHaveURL(/\/slow\/beta\?delay=35&token=fast$/);
			await expect(page.locator("#e2e-slow-bucket")).toHaveText("beta");
			await expect(page.locator("#e2e-slow-token")).toHaveText("fast");
			await assertRuntimeStatusIdle({ page });
		});

		test("slow navigation settles to idle and does not get stuck", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/navigation-race",
			});

			await page.click("#e2e-run-nav-race-fast-then-slow");
			await expect(page).toHaveURL(
				/\/slow\/alpha\?delay=900&token=slow-last$/,
			);
			await expect(page.locator("#e2e-slow-bucket")).toHaveText("alpha");
			await expect(page.locator("#e2e-slow-token")).toHaveText(
				"slow-last",
			);
			await assertRuntimeStatusIdle({ page });
		});

		test("runs multi-seed randomized sweeps and soak segments without sticky runtime state", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/",
			});
			await waitForE2EBridge({ page });

			const seedSweep = buildDeterministicSeedSweep({
				baseSeed: resolveDeterministicSeed({ runMode, uiAdapter }),
				seedCount: randomizedSweepSeedCount,
			});

			for (const [seedIndex, seed] of seedSweep.entries()) {
				const stepCount =
					seedIndex === 0
						? randomizedSoakStepCount
						: randomizedSweepStepCount;
				const randomizedSummary = await runBridgeRandomizedSequence({
					page,
					seed,
					stepCount,
					jitterMaxMilliseconds: 12,
				});
				expect(
					randomizedSummary.failedSteps,
					`seed index ${seedIndex} failed`,
				).toBe(0);
				expect(
					randomizedSummary.completedSteps,
					`seed index ${seedIndex} completed step mismatch`,
				).toBe(stepCount);
				expect(
					randomizedSummary.endingPathname,
					`seed index ${seedIndex} ending pathname mismatch`,
				).toBe("/");

				await expect(page.locator("#e2e-home-count")).toHaveText(
					String(randomizedSummary.expectedCount),
				);
				await assertRuntimeStatusIdle({ page });
			}
		});

		test("survives broader concurrency chaos without stale runtime state", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/",
			});
			await waitForE2EBridge({ page });

			const concurrencySummary = (await page.evaluate(
				async ({ seed, mutationCount }) => {
					const bridge = (
						window as Window & {
							__vormaE2EBridge?: {
								api: {
									mutate: (props: any) => Promise<any>;
								};
								navigate: (props: any) => Promise<any>;
							};
						}
					).__vormaE2EBridge;
					if (bridge === undefined) {
						throw new Error("missing __vormaE2EBridge on window");
					}

					let deterministicState = seed >>> 0;
					const nextRandomUInt32 = () => {
						deterministicState =
							(deterministicState * 1_664_525 + 1_013_904_223) >>>
							0;
						return deterministicState;
					};
					const nextRandomInt = (maxExclusive: number) => {
						return nextRandomUInt32() % maxExclusive;
					};

					const mutationPromises: Array<Promise<any>> = [];
					const navigationPromises: Array<Promise<any>> = [];

					for (
						let mutationIndex = 0;
						mutationIndex < mutationCount;
						mutationIndex += 1
					) {
						const randomDelay = 20 + nextRandomInt(230);
						mutationPromises.push(
							bridge.api.mutate({
								pattern: "/slow-increment",
								input: {
									delayMs: randomDelay,
									tag: `burst-${mutationIndex}`,
								},
							}),
						);

						if (mutationIndex % 3 === 0) {
							const randomNavDelay = nextRandomInt(120);
							navigationPromises.push(
								bridge.navigate({
									pattern: "/slow/:bucket",
									params: {
										bucket:
											mutationIndex % 2 === 0
												? "alpha"
												: "beta",
									},
									search: `?delay=${randomNavDelay}&token=concurrency-${mutationIndex}`,
								}),
							);
						}
					}

					const mutationResults = await Promise.all(mutationPromises);
					const navigationResults =
						await Promise.allSettled(navigationPromises);

					let mutationFailureCount = 0;
					let highestCount = 0;
					for (const mutationResult of mutationResults) {
						if (!mutationResult.success) {
							mutationFailureCount += 1;
							continue;
						}
						if (mutationResult.data.count > highestCount) {
							highestCount = mutationResult.data.count;
						}
					}

					const rejectedNavigationCount = navigationResults.filter(
						(result) => result.status === "rejected",
					).length;

					await bridge.navigate({ pattern: "/" });
					return {
						mutationFailureCount,
						rejectedNavigationCount,
						highestCount,
						mutationResultCount: mutationResults.length,
					};
				},
				{
					seed:
						resolveDeterministicSeed({ runMode, uiAdapter }) ^
						0x9e3779b9,
					mutationCount: concurrencyBurstMutationCount,
				},
			)) as BridgeConcurrencySummary;

			expect(concurrencySummary.mutationFailureCount).toBe(0);
			expect(concurrencySummary.rejectedNavigationCount).toBe(0);
			expect(concurrencySummary.mutationResultCount).toBe(
				concurrencyBurstMutationCount,
			);
			await expect(page.locator("#e2e-home-count")).toHaveText(
				String(concurrencySummary.highestCount),
			);
			await assertRuntimeStatusIdle({ page });
		});

		if (runMode === "dev") {
			test("handles rapid dev-time file-change races and HMR settles to latest", async ({
				page,
			}) => {
				const activeFixtureSite = mustGetRunningFixtureSite();
				const hmrProbeSourcePath = path.join(
					activeFixtureSite.frontendSourceDir,
					"components",
					"hmr_probe.tsx",
				);
				const originalHMRProbeSource = await fs.readFile(
					hmrProbeSourcePath,
					"utf8",
				);

				await visitFixturePath({
					page,
					runningFixtureSite: activeFixtureSite,
					pathname: "/hmr-probe",
				});
				const hmrTokenLocator = page.locator("#e2e-hmr-probe-token");
				await expect(hmrTokenLocator).toHaveText(hmrProbeBaselineToken);

				const firstHMRToken = `hmr-${uiAdapter}-first-${Date.now()}`;
				const secondHMRToken = `hmr-${uiAdapter}-second-${Date.now()}`;

				try {
					await writeFileReplacingToken({
						filePath: hmrProbeSourcePath,
						currentToken: hmrProbeBaselineToken,
						nextToken: firstHMRToken,
					});
					await writeFileReplacingToken({
						filePath: hmrProbeSourcePath,
						currentToken: firstHMRToken,
						nextToken: secondHMRToken,
					});

					await expect(hmrTokenLocator).toHaveText(secondHMRToken, {
						timeout: 45_000,
					});
					await assertRuntimeStatusIdle({ page });
				} finally {
					await fs.writeFile(
						hmrProbeSourcePath,
						originalHMRProbeSource,
						"utf8",
					);
					await expect(hmrTokenLocator).toHaveText(
						hmrProbeBaselineToken,
						{
							timeout: 45_000,
						},
					);
				}
			});
		}

		test("runs edge-case mutation workflows including deterministic stress matrix", async ({
			page,
		}) => {
			test.setTimeout(120_000);
			const activeFixtureSite = mustGetRunningFixtureSite();

			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/mutation-lab",
			});

			await page.fill("#e2e-echo-value", "payload-[test]-42");
			await page.fill("#e2e-echo-amount", "42");
			await page.click("#e2e-echo-button");
			await expect(page.locator("#e2e-echo-result")).toHaveText(
				"payload-[test]-42|42",
			);

			await page.click("#e2e-forced-failure-button");
			await expect(
				page.locator("#e2e-forced-failure-result"),
			).toContainText("failed:500");
			await page.click("#e2e-run-slow-increment-race-button");
			await expect(
				page.locator("#e2e-slow-increment-race-result"),
			).toHaveText("slow:2|fast:1");

			await page.click("#e2e-run-stress-button");
			await expect(page.locator("#e2e-stress-summary")).toHaveText(
				"completed:101",
			);
			await expect(page.locator("#e2e-stress-total-count")).toHaveText(
				"101",
			);
			await expect(page.locator("#e2e-stress-failure-count")).toHaveText(
				"0",
			);
			await expect(
				page.locator("#e2e-stress-results-table tbody tr"),
			).toHaveCount(101);
			await assertRuntimeStatusIdle({ page });
		});

		test("handles form action redirects into loader-backed routes", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/mutation-lab",
			});
			await assertRuntimeStatusIdle({ page });

			await page.fill(
				"#e2e-redirect-target",
				"/users/redirected-from-action?q=redirected",
			);
			await Promise.all([
				page.waitForURL(
					/\/users\/redirected-from-action\?q=redirected$/,
					{ timeout: 15_000 },
				),
				page.evaluate(() => {
					const redirectForm =
						document.getElementById("e2e-redirect-form");
					if (!(redirectForm instanceof HTMLFormElement)) {
						throw new Error(
							"redirect form missing before redirect action submit",
						);
					}
					redirectForm.requestSubmit();
				}),
			]);

			await expect(page.locator("#e2e-user-id")).toHaveText(
				"redirected-from-action",
			);
			await expect(page.locator("#e2e-user-query")).toHaveText(
				"redirected",
			);
			await assertRuntimeStatusIdle({ page });
		});

		test("follows loader redirect chains and lands on terminal route data", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/redirect-chain/start",
			});

			await expect(page).toHaveURL(
				/\/redirect-chain\/end\?hop=from-middle$/,
			);
			await expect(page.locator("#e2e-redirect-chain-hop")).toHaveText(
				"from-middle",
			);
			await expect(
				page.locator("#e2e-redirect-chain-terminal"),
			).toHaveText("redirect-chain-end");
			await assertRuntimeStatusIdle({ page });
		});

		test("renders odd-shape loader payloads without stale coercion", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/odd-shapes",
			});

			await expect(page.locator("#e2e-odd-empty-words-len")).toHaveText(
				"0",
			);
			await expect(
				page.locator("#e2e-odd-number-matrix-shape"),
			).toHaveText("2,0,3");
			await expect(page.locator("#e2e-odd-optional-note")).toHaveText(
				"null",
			);
			await expect(page.locator("#e2e-odd-nested-beta-len")).toHaveText(
				"2",
			);
			await expect(page.locator("#e2e-odd-metadata-source")).toHaveText(
				"loader",
			);
			await expect(page.locator("#e2e-odd-mixed-negative")).toHaveText(
				"-42.75",
			);
			await expect(
				page.locator("#e2e-odd-timeline-signature"),
			).toHaveText("boot:1|load:2|render:3");
			await assertRuntimeStatusIdle({ page });
		});

		test("survives forced request abort and timeout chaos without sticky loading state", async ({
			page,
		}) => {
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/",
			});
			await waitForE2EBridge({ page });

			let interceptedRequestCount = 0;
			let abortedRequestCount = 0;
			let syntheticTimeoutCount = 0;
			const networkChaosSeed =
				resolveDeterministicSeed({ runMode, uiAdapter }) ^ 0xa5a5a5a5;
			let deterministicState = networkChaosSeed >>> 0;
			const nextRandomInt = (maxExclusive: number) => {
				deterministicState =
					(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
				return deterministicState % maxExclusive;
			};
			const requestChaosRouteHandler = async (route: any) => {
				const request = route.request();
				const requestResourceType = request.resourceType();
				const isDataRequest =
					requestResourceType === "fetch" ||
					requestResourceType === "xhr";
				if (!isDataRequest) {
					await route.continue();
					return;
				}

				interceptedRequestCount += 1;
				const requestChaosMode = nextRandomInt(10);
				if (requestChaosMode === 0) {
					abortedRequestCount += 1;
					await route.abort("failed");
					return;
				}
				if (requestChaosMode === 1) {
					syntheticTimeoutCount += 1;
					await route.fulfill({
						status: 504,
						contentType: "text/plain",
						body: "forced-timeout",
					});
					return;
				}
				await route.continue();
			};

			await page.route("**/*", requestChaosRouteHandler);
			let networkChaosSummary: BridgeNetworkChaosSummary | null = null;
			try {
				networkChaosSummary = await runBridgeNetworkChaosSequence({
					page,
					seed: networkChaosSeed ^ 0x7f4a7c15,
					operationCount: networkChaosOperationCount,
				});
			} finally {
				await page.unroute("**/*", requestChaosRouteHandler);
			}

			if (networkChaosSummary === null) {
				throw new Error("network chaos summary unexpectedly missing");
			}

			expect(interceptedRequestCount).toBeGreaterThan(0);
			expect(abortedRequestCount + syntheticTimeoutCount).toBeGreaterThan(
				0,
			);
			expect(networkChaosSummary.completedOperations).toBe(
				networkChaosOperationCount,
			);
			expect(
				networkChaosSummary.failedMutations +
					networkChaosSummary.failedNavigations,
			).toBeGreaterThan(0);

			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/",
			});
			await assertRuntimeStatusIdle({ page });
			const finalObservedCount = await readHomeCountFromPage({ page });
			expect(finalObservedCount).toBeGreaterThanOrEqual(
				networkChaosSummary.minimumPossibleCount,
			);
			expect(finalObservedCount).toBeLessThanOrEqual(
				networkChaosSummary.maximumPossibleCount,
			);
		});

		test("recovers from backend restart during in-flight navigation", async ({
			page,
		}) => {
			test.setTimeout(120_000);
			const activeFixtureSite = mustGetRunningFixtureSite();
			await visitFixturePath({
				page,
				runningFixtureSite: activeFixtureSite,
				pathname: "/navigation-race",
			});

			await expect(
				page.locator("#e2e-run-nav-race-fast-then-slow"),
			).toBeVisible();
			await triggerNavigationRaceWithoutActionabilityWait({ page });
			await activeFixtureSite.restartRuntime();

			const postRestartPage = await page.context().newPage();
			try {
				await visitFixturePath({
					page: postRestartPage,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});
				await waitForE2EBridge({ page: postRestartPage });

				const postRestartSummary = await runBridgeSharedSessionSequence(
					{
						page: postRestartPage,
						seed:
							resolveDeterministicSeed({ runMode, uiAdapter }) ^
							0x13579bdf,
						operationCount: 28,
					},
				);
				expect(postRestartSummary.successfulMutations).toBeGreaterThan(
					0,
				);
				await visitFixturePath({
					page: postRestartPage,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});
				await expect(
					postRestartPage.locator("#e2e-home-count"),
				).toHaveText(/^\d+$/);
				await assertRuntimeStatusIdle({ page: postRestartPage });
			} finally {
				await postRestartPage.close();
			}
		});

		test("keeps multi-tab shared session state coherent under concurrent mutations", async ({
			page,
		}) => {
			test.setTimeout(120_000);
			const activeFixtureSite = mustGetRunningFixtureSite();
			const secondaryPage = await page.context().newPage();
			let primaryVerificationPage: Page | null = null;
			let secondaryVerificationPage: Page | null = null;
			try {
				await visitFixturePath({
					page,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});
				await visitFixturePath({
					page: secondaryPage,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});

				await waitForE2EBridge({ page });
				await waitForE2EBridge({ page: secondaryPage });

				const sharedSessionSeed = resolveDeterministicSeed({
					runMode,
					uiAdapter,
				});
				const [primarySummary, secondarySummary] = await Promise.all([
					runBridgeSharedSessionSequence({
						page,
						seed: sharedSessionSeed ^ 0x11111111,
						operationCount: sharedSessionOperationCount,
					}),
					runBridgeSharedSessionSequence({
						page: secondaryPage,
						seed: sharedSessionSeed ^ 0x22222222,
						operationCount: sharedSessionOperationCount,
					}),
				]);

				expect(primarySummary.successfulMutations).toBeGreaterThan(0);
				expect(secondarySummary.successfulMutations).toBeGreaterThan(0);

				primaryVerificationPage = await page.context().newPage();
				secondaryVerificationPage = await page.context().newPage();
				await Promise.all([
					visitFixturePath({
						page: primaryVerificationPage,
						runningFixtureSite: activeFixtureSite,
						pathname: "/",
					}),
					visitFixturePath({
						page: secondaryVerificationPage,
						runningFixtureSite: activeFixtureSite,
						pathname: "/",
					}),
				]);
				await waitForE2EBridge({
					page: primaryVerificationPage,
				});
				await waitForE2EBridge({
					page: secondaryVerificationPage,
				});

				const authoritativeCount =
					await runAuthoritativeSharedSessionSyncMutation({
						page: primaryVerificationPage,
						runningFixtureSite: activeFixtureSite,
					});

				const convergedCount =
					await waitForSharedSessionCountsToConverge({
						primaryPage: primaryVerificationPage,
						secondaryPage: secondaryVerificationPage,
						runningFixtureSite: activeFixtureSite,
						minimumExpectedCount: authoritativeCount,
					});
				expect(convergedCount).toBeGreaterThan(0);

				const [primaryCount, secondaryCount] = await Promise.all([
					readHomeCountFromPage({
						page: primaryVerificationPage,
					}),
					readHomeCountFromPage({
						page: secondaryVerificationPage,
					}),
				]);
				expect(primaryCount).toBe(secondaryCount);
				expect(primaryCount).toBeGreaterThan(0);
			} finally {
				if (secondaryVerificationPage !== null) {
					await secondaryVerificationPage.close();
				}
				if (primaryVerificationPage !== null) {
					await primaryVerificationPage.close();
				}
				await secondaryPage.close();
			}
		});
	});
}

/////////////////////////////////////////////////////////////////////
/////// Helpers
/////////////////////////////////////////////////////////////////////

function resolveDeterministicSeed(input: {
	runMode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): number {
	let seed = 0x5eedb0b5;
	for (const char of `${input.runMode}:${input.uiAdapter}`) {
		seed = (seed * 31 + char.charCodeAt(0)) >>> 0;
	}
	return seed >>> 0;
}

function buildDeterministicSeedSweep(input: {
	baseSeed: number;
	seedCount: number;
}): number[] {
	const seeds: number[] = [];
	let state = input.baseSeed >>> 0;
	for (let seedIndex = 0; seedIndex < input.seedCount; seedIndex += 1) {
		state = nextDeterministicState({
			state: state ^ ((seedIndex + 1) * 0x9e3779b9),
		});
		seeds.push(state >>> 0);
	}
	return seeds;
}

function nextDeterministicState(input: { state: number }): number {
	return (input.state * 1_664_525 + 1_013_904_223) >>> 0;
}

async function runBridgeRandomizedSequence(input: {
	page: Page;
	seed: number;
	stepCount: number;
	jitterMaxMilliseconds: number;
}): Promise<BridgeRandomizedSequenceSummary> {
	return (await input.page.evaluate(
		async ({ seed, stepCount, jitterMaxMilliseconds }) => {
			const bridge = (
				window as Window & {
					__vormaE2EBridge?: {
						api: {
							mutate: (props: any) => Promise<any>;
						};
						navigate: (props: any) => Promise<any>;
					};
				}
			).__vormaE2EBridge;
			if (bridge === undefined) {
				throw new Error("missing __vormaE2EBridge on window");
			}

			let deterministicState = seed >>> 0;
			let expectedCount = 0;
			let failedSteps = 0;
			const nextRandomInt = (maxExclusive: number) => {
				deterministicState =
					(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
				return deterministicState % maxExclusive;
			};

			for (let stepIndex = 0; stepIndex < stepCount; stepIndex += 1) {
				const randomOperationType = nextRandomInt(6);
				try {
					switch (randomOperationType) {
						case 0: {
							const incrementResult = await bridge.api.mutate({
								pattern: "/increment-count",
							});
							if (!incrementResult.success) {
								throw new Error(
									"increment-count mutation failed",
								);
							}
							expectedCount = incrementResult.data.count;
							break;
						}
						case 1: {
							const resetResult = await bridge.api.mutate({
								pattern: "/reset-count",
							});
							if (!resetResult.success) {
								throw new Error("reset-count mutation failed");
							}
							expectedCount = resetResult.data.count;
							break;
						}
						case 2: {
							const echoResult = await bridge.api.mutate({
								pattern: "/echo-body",
								input: {
									value: `echo-${stepIndex}-${nextRandomInt(10000)}`,
									amount: nextRandomInt(2000) - 1000,
								},
							});
							if (!echoResult.success) {
								throw new Error("echo-body mutation failed");
							}
							break;
						}
						case 3: {
							await bridge.navigate({
								pattern: "/users/:id",
								params: {
									id: `u-${nextRandomInt(400)}`,
								},
								search: `?q=q-${nextRandomInt(9000)}`,
							});
							break;
						}
						case 4: {
							const randomDelay = nextRandomInt(120);
							const randomBucket =
								nextRandomInt(2) === 0 ? "alpha" : "beta";
							await bridge.navigate({
								pattern: "/slow/:bucket",
								params: { bucket: randomBucket },
								search: `?delay=${randomDelay}&token=rnd-${stepIndex}`,
							});
							break;
						}
						default: {
							await bridge.navigate({ pattern: "/" });
						}
					}
				} catch {
					failedSteps += 1;
				}

				if (jitterMaxMilliseconds > 0) {
					await new Promise((resolve) => {
						setTimeout(
							resolve,
							nextRandomInt(jitterMaxMilliseconds),
						);
					});
				}
			}

			await bridge.navigate({ pattern: "/" });
			return {
				expectedCount,
				failedSteps,
				completedSteps: stepCount,
				endingPathname: `${window.location.pathname}${window.location.search}`,
			};
		},
		{
			seed: input.seed,
			stepCount: input.stepCount,
			jitterMaxMilliseconds: input.jitterMaxMilliseconds,
		},
	)) as BridgeRandomizedSequenceSummary;
}

async function runBridgeNetworkChaosSequence(input: {
	page: Page;
	seed: number;
	operationCount: number;
}): Promise<BridgeNetworkChaosSummary> {
	let deterministicState = input.seed >>> 0;
	let expectedCount = 0;
	let minimumPossibleCount = 0;
	let maximumPossibleCount = 0;
	let failedMutations = 0;
	let failedNavigations = 0;
	const nextRandomInt = (maxExclusive: number) => {
		deterministicState =
			(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
		return deterministicState % maxExclusive;
	};

	for (
		let operationIndex = 0;
		operationIndex < input.operationCount;
		operationIndex += 1
	) {
		const operationKind = nextRandomInt(6);
		if (operationKind <= 3) {
			let mutationRequest: any = { pattern: "/increment-count" };
			const isIncrementLikeMutationOperation = (): boolean => {
				return operationKind === 0 || operationKind === 2;
			};
			const isResetMutationOperation = (): boolean => {
				return operationKind === 1;
			};
			switch (operationKind) {
				case 0: {
					mutationRequest = {
						pattern: "/increment-count",
					};
					break;
				}
				case 1: {
					mutationRequest = {
						pattern: "/reset-count",
					};
					break;
				}
				case 2: {
					mutationRequest = {
						pattern: "/slow-increment",
						input: {
							delayMs: 10 + nextRandomInt(70),
							tag: `chaos-${operationIndex}`,
						},
					};
					break;
				}
				default: {
					mutationRequest = {
						pattern: "/echo-body",
						input: {
							value: `chaos-${operationIndex}`,
							amount: nextRandomInt(200) - 100,
						},
					};
					break;
				}
			}

			const mutationResult = await runBridgeMutationWithContextRecovery({
				page: input.page,
				requestProps: mutationRequest,
			});
			if (!mutationResult.success) {
				failedMutations += 1;
				if (isIncrementLikeMutationOperation()) {
					maximumPossibleCount += 1;
				}
				if (isResetMutationOperation()) {
					minimumPossibleCount = 0;
				}
				continue;
			}
			if (mutationResult.count !== undefined) {
				expectedCount = mutationResult.count;
				minimumPossibleCount = mutationResult.count;
				maximumPossibleCount = mutationResult.count;
				continue;
			}
			if (isIncrementLikeMutationOperation()) {
				minimumPossibleCount += 1;
				maximumPossibleCount += 1;
				expectedCount = minimumPossibleCount;
				continue;
			}
			if (isResetMutationOperation()) {
				minimumPossibleCount = 0;
				maximumPossibleCount = 0;
				expectedCount = 0;
			}
			continue;
		}

		const navigationRequest =
			operationKind === 4
				? {
						pattern: "/users/:id",
						params: { id: `chaos-${nextRandomInt(80)}` },
						search: `?q=q-${nextRandomInt(4000)}`,
					}
				: {
						pattern: "/slow/:bucket",
						params: {
							bucket: nextRandomInt(2) === 0 ? "alpha" : "beta",
						},
						search: `?delay=${nextRandomInt(100)}&token=chaos-nav-${operationIndex}`,
					};
		const navigationSucceeded =
			await runBridgeNavigationWithContextRecovery({
				page: input.page,
				requestProps: navigationRequest,
			});
		if (!navigationSucceeded) {
			failedNavigations += 1;
		}
	}

	return {
		failedMutations,
		failedNavigations,
		completedOperations: input.operationCount,
		expectedCount,
		minimumPossibleCount,
		maximumPossibleCount,
	};
}

async function runBridgeMutationWithContextRecovery(input: {
	page: Page;
	requestProps: any;
}): Promise<{ success: boolean; count?: number }> {
	try {
		return await input.page.evaluate(
			async ({ requestProps }) => {
				const bridge = (
					window as Window & {
						__vormaE2EBridge?: {
							api: {
								mutate: (props: any) => Promise<any>;
							};
						};
					}
				).__vormaE2EBridge;
				if (bridge === undefined) {
					throw new Error("missing __vormaE2EBridge on window");
				}

				try {
					const mutationResult =
						await bridge.api.mutate(requestProps);
					if (!mutationResult.success) {
						return { success: false };
					}
					if (
						mutationResult.data !== null &&
						typeof mutationResult.data === "object" &&
						"count" in mutationResult.data &&
						typeof mutationResult.data.count === "number"
					) {
						return {
							success: true,
							count: mutationResult.data.count,
						};
					}
					return { success: true };
				} catch {
					return { success: false };
				}
			},
			{
				requestProps: input.requestProps,
			},
		);
	} catch (error) {
		if (!isBridgeExecutionContextResetError({ error })) {
			throw error;
		}
		return { success: false };
	}
}

async function runBridgeNavigationWithContextRecovery(input: {
	page: Page;
	requestProps: any;
}): Promise<boolean> {
	try {
		return await input.page.evaluate(
			async ({ requestProps }) => {
				const bridge = (
					window as Window & {
						__vormaE2EBridge?: {
							navigate: (props: any) => Promise<any>;
						};
					}
				).__vormaE2EBridge;
				if (bridge === undefined) {
					throw new Error("missing __vormaE2EBridge on window");
				}

				try {
					await bridge.navigate(requestProps);
					return true;
				} catch {
					return false;
				}
			},
			{
				requestProps: input.requestProps,
			},
		);
	} catch (error) {
		if (!isBridgeExecutionContextResetError({ error })) {
			throw error;
		}
		return false;
	}
}

function isBridgeExecutionContextResetError(input: {
	error: unknown;
}): boolean {
	const errorMessage = describeErrorForTestOutput({
		error: input.error,
	}).toLowerCase();
	return (
		errorMessage.includes("execution context was destroyed") ||
		errorMessage.includes("cannot find context with specified id") ||
		errorMessage.includes("missing __vormae2ebridge on window")
	);
}

async function runBridgeSharedSessionSequence(input: {
	page: Page;
	seed: number;
	operationCount: number;
}): Promise<BridgeSharedSessionSummary> {
	let deterministicState = input.seed >>> 0;
	let successfulMutations = 0;
	let highestCount = 0;
	const nextRandomInt = (maxExclusive: number) => {
		deterministicState =
			(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
		return deterministicState % maxExclusive;
	};

	for (
		let operationIndex = 0;
		operationIndex < input.operationCount;
		operationIndex += 1
	) {
		const operationKind = nextRandomInt(5);
		if (operationKind <= 1) {
			const mutationRequest =
				operationKind === 0
					? { pattern: "/increment-count" }
					: {
							pattern: "/slow-increment",
							input: {
								delayMs: 5 + nextRandomInt(40),
								tag: `tab-${operationIndex}`,
							},
						};
			const mutationResult = await runBridgeMutationWithContextRecovery({
				page: input.page,
				requestProps: mutationRequest,
			});
			if (mutationResult.success) {
				successfulMutations += 1;
				if (
					mutationResult.count !== undefined &&
					mutationResult.count > highestCount
				) {
					highestCount = mutationResult.count;
				}
			}
			continue;
		}

		switch (operationKind) {
			case 2: {
				await runBridgeNavigationWithContextRecovery({
					page: input.page,
					requestProps: {
						pattern: "/users/:id",
						params: {
							id: `tab-user-${nextRandomInt(90)}`,
						},
						search: `?q=tab-${nextRandomInt(5000)}`,
					},
				});
				break;
			}
			case 3: {
				await runBridgeNavigationWithContextRecovery({
					page: input.page,
					requestProps: {
						pattern: "/slow/:bucket",
						params: {
							bucket: nextRandomInt(2) === 0 ? "alpha" : "beta",
						},
						search: `?delay=${nextRandomInt(80)}&token=tab-nav-${operationIndex}`,
					},
				});
				break;
			}
			default: {
				await runBridgeNavigationWithContextRecovery({
					page: input.page,
					requestProps: { pattern: "/" },
				});
			}
		}
	}

	await runBridgeNavigationWithContextRecovery({
		page: input.page,
		requestProps: { pattern: "/" },
	});
	const endingPathname = await readCurrentPathnameAndSearch({
		page: input.page,
	});

	return {
		successfulMutations,
		highestCount,
		endingPathname,
	};
}

async function readCurrentPathnameAndSearch(input: {
	page: Page;
}): Promise<string> {
	try {
		return await input.page.evaluate(() => {
			return `${window.location.pathname}${window.location.search}`;
		});
	} catch (error) {
		if (!isBridgeExecutionContextResetError({ error })) {
			throw error;
		}
		const fallbackURL = new URL(input.page.url());
		return `${fallbackURL.pathname}${fallbackURL.search}`;
	}
}

async function readHomeCountFromPage(input: { page: Page }): Promise<number> {
	const rawCount = await input.page.locator("#e2e-home-count").textContent();
	if (rawCount === null) {
		throw new Error("home count text was null");
	}
	const parsedCount = Number.parseInt(rawCount, 10);
	if (Number.isNaN(parsedCount)) {
		throw new Error(`unable to parse home count from "${rawCount}"`);
	}
	return parsedCount;
}

async function triggerNavigationRaceWithoutActionabilityWait(input: {
	page: Page;
}): Promise<void> {
	try {
		await input.page.evaluate(() => {
			const raceTriggerButton = document.getElementById(
				"e2e-run-nav-race-fast-then-slow",
			);
			if (!(raceTriggerButton instanceof HTMLButtonElement)) {
				throw new Error(
					"navigation race trigger button missing before restart chaos step",
				);
			}
			raceTriggerButton.click();
		});
	} catch (error) {
		if (isBridgeExecutionContextResetError({ error })) {
			// Navigation can replace execution context immediately after click.
			return;
		}
		throw error;
	}
}

async function waitForE2EBridge(input: { page: Page }): Promise<void> {
	await expect
		.poll(
			async () => {
				try {
					return await input.page.evaluate(() => {
						return (
							(
								window as Window & {
									__vormaE2EBridge?: unknown;
								}
							).__vormaE2EBridge !== undefined
						);
					});
				} catch (error) {
					if (isBridgeExecutionContextResetError({ error })) {
						return false;
					}
					throw error;
				}
			},
			{ timeout: 30_000 },
		)
		.toBe(true);
}

async function writeFileReplacingToken(input: {
	filePath: string;
	currentToken: string;
	nextToken: string;
}): Promise<void> {
	const currentFileContents = await fs.readFile(input.filePath, "utf8");
	if (!currentFileContents.includes(input.currentToken)) {
		throw new Error(
			`token ${input.currentToken} not found in ${input.filePath}; cannot apply HMR file mutation`,
		);
	}

	const nextFileContents = currentFileContents.replace(
		input.currentToken,
		input.nextToken,
	);
	await fs.writeFile(input.filePath, nextFileContents, "utf8");
}

async function visitFixturePath(input: {
	page: Page;
	runningFixtureSite: RunningFixtureSite;
	pathname: string;
}): Promise<void> {
	const targetURL = new URL(
		input.pathname,
		input.runningFixtureSite.baseURL,
	).toString();
	const readinessDeadlineMS = Date.now() + 90_000;
	const perAttemptNavigationTimeoutMS = 3_000;
	let attemptCount = 0;
	let lastNavigationErrorMessage = "none";

	while (Date.now() < readinessDeadlineMS) {
		attemptCount += 1;
		try {
			await input.page.goto(targetURL, {
				waitUntil: "domcontentloaded",
				timeout: perAttemptNavigationTimeoutMS,
			});
		} catch (error) {
			lastNavigationErrorMessage = describeErrorForTestOutput({
				error,
			});
			const pageAlreadyReachedTarget =
				await didPageReachTargetRootShellAfterNavigationAbort({
					page: input.page,
					targetURL,
				});
			if (pageAlreadyReachedTarget) {
				return;
			}
			await input.page.waitForTimeout(250);
			continue;
		}

		const rootShellAppeared = await didRootShellAppear({
			page: input.page,
			timeoutMS: 1_250,
		});
		if (rootShellAppeared) {
			return;
		}

		await input.page.waitForTimeout(250);
	}

	const lastPageURL = input.page.url();
	throw new Error(
		`timed out waiting for hydrated root shell at ${targetURL}; attempts=${attemptCount}; lastURL=${lastPageURL}; lastNavigationError=${lastNavigationErrorMessage}`,
	);
}

async function didPageReachTargetRootShellAfterNavigationAbort(input: {
	page: Page;
	targetURL: string;
}): Promise<boolean> {
	if (
		!doURLsMatchIgnoringHash({
			firstURL: input.page.url(),
			secondURL: input.targetURL,
		})
	) {
		return false;
	}

	return await didRootShellAppear({
		page: input.page,
		timeoutMS: 250,
	});
}

function doURLsMatchIgnoringHash(input: {
	firstURL: string;
	secondURL: string;
}): boolean {
	try {
		const first = new URL(input.firstURL);
		const second = new URL(input.secondURL);
		return (
			first.origin === second.origin &&
			first.pathname === second.pathname &&
			first.search === second.search
		);
	} catch {
		return false;
	}
}

async function didRootShellAppear(input: {
	page: Page;
	timeoutMS: number;
}): Promise<boolean> {
	try {
		await input.page.locator("#e2e-root-shell").first().waitFor({
			state: "attached",
			timeout: input.timeoutMS,
		});
		return true;
	} catch {
		return false;
	}
}

function buildUserRoutePath(input: {
	userID: string;
	queryValue: string;
}): string {
	const encodedUserID = encodeURIComponent(input.userID);
	const encodedQueryValue = encodeURIComponent(input.queryValue);
	return `/users/${encodedUserID}?q=${encodedQueryValue}`;
}

async function resetCounterStateForFixture(input: {
	runningFixtureSite: RunningFixtureSite;
}): Promise<void> {
	const attemptedPaths = ["/api/reset-count", "/reset-count"];

	for (const attemptedPath of attemptedPaths) {
		const resetResponse = await fetch(
			new URL(attemptedPath, input.runningFixtureSite.baseURL).toString(),
			{ method: "POST" },
		);
		if (resetResponse.ok) {
			return;
		}
	}

	throw new Error(
		`unable to reset request count in ${input.runningFixtureSite.mode} mode via ${attemptedPaths.join(", ")}`,
	);
}

async function assertRuntimeStatusIdle(input: { page: Page }): Promise<void> {
	await expect(input.page.locator("#e2e-status-navigating")).toHaveText("0");
	await expect(input.page.locator("#e2e-status-submitting")).toHaveText("0");
	await expect(input.page.locator("#e2e-status-revalidating")).toHaveText(
		"0",
	);
	await expect(input.page.locator("html")).toHaveAttribute(
		"data-e2e-loading",
		"0",
	);
}

async function waitForSharedSessionCountsToConverge(input: {
	primaryPage: Page;
	secondaryPage: Page;
	runningFixtureSite: RunningFixtureSite;
	minimumExpectedCount: number;
}): Promise<number> {
	const deadlineTimestampMS = Date.now() + 45_000;
	let lastObservedPrimaryCount = -1;
	let lastObservedSecondaryCount = -1;
	let lastObservedError = "none";
	let attemptCount = 0;

	while (Date.now() < deadlineTimestampMS) {
		attemptCount += 1;
		try {
			await Promise.all([
				visitFixturePath({
					page: input.primaryPage,
					runningFixtureSite: input.runningFixtureSite,
					pathname: "/",
				}),
				visitFixturePath({
					page: input.secondaryPage,
					runningFixtureSite: input.runningFixtureSite,
					pathname: "/",
				}),
			]);
			await Promise.all([
				assertRuntimeStatusIdle({ page: input.primaryPage }),
				assertRuntimeStatusIdle({ page: input.secondaryPage }),
			]);

			const [primaryCount, secondaryCount] = await Promise.all([
				readHomeCountFromPage({
					page: input.primaryPage,
				}),
				readHomeCountFromPage({
					page: input.secondaryPage,
				}),
			]);
			lastObservedPrimaryCount = primaryCount;
			lastObservedSecondaryCount = secondaryCount;
			lastObservedError = "none";

			if (
				primaryCount === secondaryCount &&
				primaryCount >= input.minimumExpectedCount
			) {
				return primaryCount;
			}
		} catch (error) {
			lastObservedError = describeErrorForTestOutput({
				error,
			});
		}

		await input.primaryPage.waitForTimeout(250);
	}

	throw new Error(
		[
			"timed out waiting for shared-session home counts to converge",
			`attempts=${attemptCount}`,
			`primary=${lastObservedPrimaryCount}`,
			`secondary=${lastObservedSecondaryCount}`,
			`minimumExpected=${input.minimumExpectedCount}`,
			`lastError=${lastObservedError}`,
		].join("; "),
	);
}

async function runAuthoritativeSharedSessionSyncMutation(input: {
	page: Page;
	runningFixtureSite: RunningFixtureSite;
}): Promise<number> {
	const deadlineTimestampMS = Date.now() + 45_000;
	let attemptCount = 0;
	let lastAttemptError = "none";

	while (Date.now() < deadlineTimestampMS) {
		attemptCount += 1;
		try {
			await visitFixturePath({
				page: input.page,
				runningFixtureSite: input.runningFixtureSite,
				pathname: "/",
			});
			await waitForE2EBridge({ page: input.page });

			const authoritativeMutationResult =
				await runBridgeMutationWithContextRecovery({
					page: input.page,
					requestProps: {
						pattern: "/increment-count",
					},
				});
			if (
				!authoritativeMutationResult.success ||
				authoritativeMutationResult.count === undefined
			) {
				lastAttemptError =
					"authoritative mutation did not return a count";
				await input.page.waitForTimeout(250);
				continue;
			}

			await visitFixturePath({
				page: input.page,
				runningFixtureSite: input.runningFixtureSite,
				pathname: "/",
			});
			await assertRuntimeStatusIdle({ page: input.page });
			const observedHomeCountAfterMutation = await readHomeCountFromPage({
				page: input.page,
			});
			if (observedHomeCountAfterMutation > 0) {
				return observedHomeCountAfterMutation;
			}
			lastAttemptError =
				"observed non-positive home count after authoritative mutation";
		} catch (error) {
			lastAttemptError = describeErrorForTestOutput({
				error,
			});
		}

		await input.page.waitForTimeout(250);
	}

	throw new Error(
		`timed out resolving authoritative shared-session sync mutation result after ${attemptCount} attempts; lastError=${lastAttemptError}`,
	);
}

function describeErrorForTestOutput(input: { error: unknown }): string {
	if (input.error instanceof Error) {
		return input.error.message;
	}
	return String(input.error);
}

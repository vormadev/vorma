import { expect, test, type Page } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";
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

for (const runMode of resolveRunModesForExecution()) {
	for (const uiAdapter of resolveUIAdaptersForExecution()) {
		test.describe(`wave-vorma-runtime-${runMode}-${uiAdapter}`, () => {
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

			test("renders root and nested index loader data", async ({
				page,
			}) => {
				const activeFixtureSite = mustGetRunningFixtureSite();
				await visitFixturePath({
					page,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});

				await expect(page.locator("#e2e-runtime-mode")).toHaveText(
					runMode,
				);
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
				await expect(page.locator("#e2e-increment-result")).toHaveText(
					"2",
				);

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

				await expect(
					page.locator("#e2e-explode-boundary"),
				).toContainText("explode-boundary:");
				await expect(
					page.locator("#e2e-explode-unreachable"),
				).toHaveCount(0);
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
				await expect(page).toHaveURL(
					/\/slow\/beta\?delay=35&token=fast$/,
				);
				await expect(page.locator("#e2e-slow-bucket")).toHaveText(
					"beta",
				);
				await expect(page.locator("#e2e-slow-token")).toHaveText(
					"fast",
				);
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
				await expect(page.locator("#e2e-slow-bucket")).toHaveText(
					"alpha",
				);
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
					const randomizedSummary = await runBridgeRandomizedSequence(
						{
							page,
							seed,
							stepCount,
							jitterMaxMilliseconds: 12,
						},
					);
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
							throw new Error(
								"missing __vormaE2EBridge on window",
							);
						}

						let deterministicState = seed >>> 0;
						const nextRandomUInt32 = () => {
							deterministicState =
								(deterministicState * 1_664_525 +
									1_013_904_223) >>>
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

						const mutationResults =
							await Promise.all(mutationPromises);
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

						const rejectedNavigationCount =
							navigationResults.filter(
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
					const hmrTokenLocator = page.locator(
						"#e2e-hmr-probe-token",
					);
					await expect(hmrTokenLocator).toHaveText(
						hmrProbeBaselineToken,
					);

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

						await expect(hmrTokenLocator).toHaveText(
							secondHMRToken,
							{
								timeout: 45_000,
							},
						);
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

				test("recovers HMR stream after forced websocket transport drop", async ({
					page,
				}) => {
					test.setTimeout(120_000);
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

					await installWebSocketTracker({ page });
					await visitFixturePath({
						page,
						runningFixtureSite: activeFixtureSite,
						pathname: "/hmr-probe",
					});
					const hmrTokenLocator = page.locator(
						"#e2e-hmr-probe-token",
					);
					await expect(hmrTokenLocator).toHaveText(
						hmrProbeBaselineToken,
					);

					const droppedSocketCount =
						await forceCloseTrackedWebSockets({
							page,
						});
					expect(droppedSocketCount).toBeGreaterThan(0);

					const recoveryToken = `hmr-${uiAdapter}-recovered-${Date.now()}`;
					try {
						await writeFileReplacingToken({
							filePath: hmrProbeSourcePath,
							currentToken: hmrProbeBaselineToken,
							nextToken: recoveryToken,
						});

						await expect(hmrTokenLocator).toHaveText(
							recoveryToken,
							{
								timeout: 45_000,
							},
						);
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
				await expect(
					page.locator("#e2e-stress-total-count"),
				).toHaveText("101");
				await expect(
					page.locator("#e2e-stress-failure-count"),
				).toHaveText("0");
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

				await page.fill(
					"#e2e-redirect-target",
					"/users/redirected-from-action?q=redirected",
				);
				await page.click("#e2e-redirect-submit");

				await expect(page).toHaveURL(
					/\/users\/redirected-from-action\?q=redirected$/,
				);
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
				await expect(
					page.locator("#e2e-redirect-chain-hop"),
				).toHaveText("from-middle");
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

				await expect(
					page.locator("#e2e-odd-empty-words-len"),
				).toHaveText("0");
				await expect(
					page.locator("#e2e-odd-number-matrix-shape"),
				).toHaveText("2,0,3");
				await expect(page.locator("#e2e-odd-optional-note")).toHaveText(
					"null",
				);
				await expect(
					page.locator("#e2e-odd-nested-beta-len"),
				).toHaveText("2");
				await expect(
					page.locator("#e2e-odd-metadata-source"),
				).toHaveText("loader");
				await expect(
					page.locator("#e2e-odd-mixed-negative"),
				).toHaveText("-42.75");
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
					resolveDeterministicSeed({ runMode, uiAdapter }) ^
					0xa5a5a5a5;
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
				let networkChaosSummary: BridgeNetworkChaosSummary | null =
					null;
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
					throw new Error(
						"network chaos summary unexpectedly missing",
					);
				}

				expect(interceptedRequestCount).toBeGreaterThan(0);
				expect(
					abortedRequestCount + syntheticTimeoutCount,
				).toBeGreaterThan(0);
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
				await expect(page.locator("#e2e-home-count")).toHaveText(
					String(networkChaosSummary.expectedCount),
				);
				await assertRuntimeStatusIdle({ page });
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

				await visitFixturePath({
					page,
					runningFixtureSite: activeFixtureSite,
					pathname: "/",
				});
				await waitForE2EBridge({ page });

				const postRestartSummary = await runBridgeRandomizedSequence({
					page,
					seed:
						resolveDeterministicSeed({ runMode, uiAdapter }) ^
						0x13579bdf,
					stepCount: 28,
					jitterMaxMilliseconds: 8,
				});
				expect(postRestartSummary.failedSteps).toBe(0);
				await expect(page.locator("#e2e-home-count")).toHaveText(
					String(postRestartSummary.expectedCount),
				);
				await assertRuntimeStatusIdle({ page });
			});

			test("keeps multi-tab shared session state coherent under concurrent mutations", async ({
				page,
			}) => {
				test.setTimeout(120_000);
				const activeFixtureSite = mustGetRunningFixtureSite();
				const secondaryPage = await page.context().newPage();
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
					const [primarySummary, secondarySummary] =
						await Promise.all([
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

					expect(primarySummary.successfulMutations).toBeGreaterThan(
						0,
					);
					expect(
						secondarySummary.successfulMutations,
					).toBeGreaterThan(0);
					expect(primarySummary.endingPathname).toBe("/");
					expect(secondarySummary.endingPathname).toBe("/");

					const minimumExpectedCount = Math.max(
						primarySummary.highestCount,
						secondarySummary.highestCount,
					);
					const convergedCount =
						await waitForSharedSessionCountsToConverge({
							primaryPage: page,
							secondaryPage,
							runningFixtureSite: activeFixtureSite,
							minimumExpectedCount,
						});
					expect(convergedCount).toBeGreaterThan(0);

					const [primaryCount, secondaryCount] = await Promise.all([
						readHomeCountFromPage({ page }),
						readHomeCountFromPage({
							page: secondaryPage,
						}),
					]);
					expect(primaryCount).toBe(secondaryCount);
					expect(primaryCount).toBeGreaterThanOrEqual(
						minimumExpectedCount,
					);
				} finally {
					await secondaryPage.close();
				}
			});
		});
	}
}

/////////////////////////////////////////////////////////////////////
/////// Helpers
/////////////////////////////////////////////////////////////////////

function resolveRunModesForExecution(): ReadonlyArray<E2ERunMode> {
	const envMode = process.env.VORMA_E2E_MODE;
	if (envMode === undefined || envMode === "" || envMode === "all") {
		return ["dev", "prod"];
	}

	if (envMode === "dev" || envMode === "prod") {
		return [envMode];
	}

	throw new Error(
		`unsupported VORMA_E2E_MODE=${envMode}; expected "dev", "prod", or "all"`,
	);
}

function resolveUIAdaptersForExecution(): ReadonlyArray<E2EUIAdapter> {
	const envAdapters =
		process.env.VORMA_E2E_UI_ADAPTERS ?? process.env.VORMA_E2E_UI_ADAPTER;
	if (
		envAdapters === undefined ||
		envAdapters === "" ||
		envAdapters === "all"
	) {
		return ["solid", "react", "preact"];
	}

	const parsedAdapters = envAdapters
		.split(",")
		.map((adapter) => adapter.trim())
		.filter((adapter) => adapter !== "");
	if (parsedAdapters.length === 0) {
		throw new Error(
			`unsupported VORMA_E2E_UI_ADAPTERS=${envAdapters}; expected comma-separated adapters`,
		);
	}

	const normalizedAdapters = Array.from(new Set(parsedAdapters));
	for (const adapter of normalizedAdapters) {
		if (
			adapter !== "solid" &&
			adapter !== "react" &&
			adapter !== "preact"
		) {
			throw new Error(
				`unsupported UI adapter=${adapter}; expected "solid", "react", or "preact"`,
			);
		}
	}

	return normalizedAdapters as E2EUIAdapter[];
}

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
	return (await input.page.evaluate(
		async ({ seed, operationCount }) => {
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
			let failedMutations = 0;
			let failedNavigations = 0;
			const nextRandomInt = (maxExclusive: number) => {
				deterministicState =
					(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
				return deterministicState % maxExclusive;
			};

			for (
				let operationIndex = 0;
				operationIndex < operationCount;
				operationIndex += 1
			) {
				const operationKind = nextRandomInt(6);
				if (operationKind <= 3) {
					try {
						let mutationResult: any;
						switch (operationKind) {
							case 0: {
								mutationResult = await bridge.api.mutate({
									pattern: "/increment-count",
								});
								break;
							}
							case 1: {
								mutationResult = await bridge.api.mutate({
									pattern: "/reset-count",
								});
								break;
							}
							case 2: {
								mutationResult = await bridge.api.mutate({
									pattern: "/slow-increment",
									input: {
										delayMs: 10 + nextRandomInt(70),
										tag: `chaos-${operationIndex}`,
									},
								});
								break;
							}
							default: {
								mutationResult = await bridge.api.mutate({
									pattern: "/echo-body",
									input: {
										value: `chaos-${operationIndex}`,
										amount: nextRandomInt(200) - 100,
									},
								});
							}
						}
						if (!mutationResult.success) {
							failedMutations += 1;
							continue;
						}
						if ("count" in mutationResult.data) {
							expectedCount = mutationResult.data.count;
						}
					} catch {
						failedMutations += 1;
					}
					continue;
				}

				try {
					if (operationKind === 4) {
						await bridge.navigate({
							pattern: "/users/:id",
							params: { id: `chaos-${nextRandomInt(80)}` },
							search: `?q=q-${nextRandomInt(4000)}`,
						});
					} else {
						await bridge.navigate({
							pattern: "/slow/:bucket",
							params: {
								bucket:
									nextRandomInt(2) === 0 ? "alpha" : "beta",
							},
							search: `?delay=${nextRandomInt(100)}&token=chaos-nav-${operationIndex}`,
						});
					}
				} catch {
					failedNavigations += 1;
				}
			}

			await bridge.navigate({ pattern: "/" });
			return {
				failedMutations,
				failedNavigations,
				completedOperations: operationCount,
				expectedCount,
			};
		},
		{
			seed: input.seed,
			operationCount: input.operationCount,
		},
	)) as BridgeNetworkChaosSummary;
}

async function runBridgeSharedSessionSequence(input: {
	page: Page;
	seed: number;
	operationCount: number;
}): Promise<BridgeSharedSessionSummary> {
	return (await input.page.evaluate(
		async ({ seed, operationCount }) => {
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
			let successfulMutations = 0;
			let highestCount = 0;
			const nextRandomInt = (maxExclusive: number) => {
				deterministicState =
					(deterministicState * 1_664_525 + 1_013_904_223) >>> 0;
				return deterministicState % maxExclusive;
			};

			for (
				let operationIndex = 0;
				operationIndex < operationCount;
				operationIndex += 1
			) {
				const operationKind = nextRandomInt(5);
				if (operationKind <= 1) {
					try {
						const mutationResult = await bridge.api.mutate({
							pattern:
								operationKind === 0
									? "/increment-count"
									: "/slow-increment",
							input:
								operationKind === 0
									? undefined
									: {
											delayMs: 5 + nextRandomInt(40),
											tag: `tab-${operationIndex}`,
										},
						});
						if (mutationResult.success) {
							successfulMutations += 1;
							if (
								"count" in mutationResult.data &&
								mutationResult.data.count > highestCount
							) {
								highestCount = mutationResult.data.count;
							}
						}
					} catch {
						// continue sequence; failures are expected in chaotic runs.
					}
					continue;
				}

				try {
					switch (operationKind) {
						case 2: {
							await bridge.navigate({
								pattern: "/users/:id",
								params: {
									id: `tab-user-${nextRandomInt(90)}`,
								},
								search: `?q=tab-${nextRandomInt(5000)}`,
							});
							break;
						}
						case 3: {
							await bridge.navigate({
								pattern: "/slow/:bucket",
								params: {
									bucket:
										nextRandomInt(2) === 0
											? "alpha"
											: "beta",
								},
								search: `?delay=${nextRandomInt(80)}&token=tab-nav-${operationIndex}`,
							});
							break;
						}
						default: {
							await bridge.navigate({ pattern: "/" });
						}
					}
				} catch {
					// continue sequence; failures are expected in chaotic runs.
				}
			}

			await bridge.navigate({ pattern: "/" });
			return {
				successfulMutations,
				highestCount,
				endingPathname: `${window.location.pathname}${window.location.search}`,
			};
		},
		{
			seed: input.seed,
			operationCount: input.operationCount,
		},
	)) as BridgeSharedSessionSummary;
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

async function installWebSocketTracker(input: { page: Page }): Promise<void> {
	await input.page.addInitScript(() => {
		const browserWindow = window as Window & {
			__e2eCloseTrackedWebSockets?: () => number;
			__e2eWebSocketTrackingInstalled?: boolean;
		};
		if (browserWindow.__e2eWebSocketTrackingInstalled) {
			return;
		}
		browserWindow.__e2eWebSocketTrackingInstalled = true;

		const trackedSockets = new Set<WebSocket>();
		const NativeWebSocket = window.WebSocket;
		class TrackingWebSocket extends NativeWebSocket {
			constructor(url: string | URL, protocols?: string | string[]) {
				if (protocols === undefined) {
					super(url);
				} else {
					super(url, protocols);
				}
				trackedSockets.add(this);
				const clearSocket = () => {
					trackedSockets.delete(this);
				};
				this.addEventListener("close", clearSocket);
				this.addEventListener("error", clearSocket);
			}
		}

		window.WebSocket = TrackingWebSocket;
		browserWindow.__e2eCloseTrackedWebSockets = () => {
			const socketsToClose = Array.from(trackedSockets);
			for (const trackedSocket of socketsToClose) {
				try {
					trackedSocket.close(4_101, "e2e-forced-drop");
				} catch {
					// ignore close errors for sockets already closing.
				}
			}
			return socketsToClose.length;
		};
	});
}

async function forceCloseTrackedWebSockets(input: {
	page: Page;
}): Promise<number> {
	return await input.page.evaluate(() => {
		const browserWindow = window as Window & {
			__e2eCloseTrackedWebSockets?: () => number;
		};
		return browserWindow.__e2eCloseTrackedWebSockets?.() ?? 0;
	});
}

async function triggerNavigationRaceWithoutActionabilityWait(input: {
	page: Page;
}): Promise<void> {
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
}

async function waitForE2EBridge(input: { page: Page }): Promise<void> {
	await expect
		.poll(
			async () => {
				return input.page.evaluate(() => {
					return (
						(
							window as Window & {
								__vormaE2EBridge?: unknown;
							}
						).__vormaE2EBridge !== undefined
					);
				});
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
	let latestConvergedCount = -1;
	await expect
		.poll(
			async () => {
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
				if (
					primaryCount === secondaryCount &&
					primaryCount >= input.minimumExpectedCount
				) {
					latestConvergedCount = primaryCount;
					return true;
				}
				return false;
			},
			{
				timeout: 30_000,
			},
		)
		.toBe(true);
	return latestConvergedCount;
}

function describeErrorForTestOutput(input: { error: unknown }): string {
	if (input.error instanceof Error) {
		return input.error.message;
	}
	return String(input.error);
}

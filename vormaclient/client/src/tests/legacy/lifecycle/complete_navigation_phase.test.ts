// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { vormaNavigate } from "../../../client";

import { addBuildIDListener } from "../../../platform/events.ts";

import { __vormaClientGlobal } from "../../../app/context.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "../client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("2. Navigation Lifecycle", () => {
		describe("2.3 Complete Navigation Phase", () => {
			it("should handle redirect data result", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/redirect-target",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Redirect Target" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/start");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Redirect Target");
			});

			it("should dispatch build-id event on change", async () => {
				const buildIdListener = vi.fn();
				const cleanup = addListener(
					addBuildIDListener,
					buildIdListener,
				);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(
						{ importURLs: [], cssBundles: [] },
						{ headers: { "X-Vorma-Build-Id": "new-build-456" } },
					),
				);

				await vormaNavigate("/new-build");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: {
							oldID: "1",
							newID: "new-build-456",
						},
					}),
				);

				cleanup();
			});

			it("should wait for client data before rendering", async () => {
				const clientData = { processed: true };
				const waitFn = vi.fn().mockResolvedValue(clientData);

				setupGlobalVormaContext({
					patternToWaitFnMap: { "/": waitFn },
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: [],
						matchedPatterns: ["/"],
						loadersData: [{}],
					}),
				);

				await vormaNavigate("/wait-test");
				await vi.runAllTimersAsync();

				expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
					clientData,
				]);
			});
		});
	});
});

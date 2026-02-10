// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { getHistoryInstance, vormaNavigate } from "./client";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("1. Core Navigation", () => {
		describe("1.4 Programmatic Navigation", () => {
			it("should support navigate() with replace option", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				const replaceSpy = vi.spyOn(history, "replace");
				const pushSpy = vi.spyOn(history, "push");

				await vormaNavigate("/replace-test", { replace: true });
				await vi.runAllTimersAsync();

				// Should have called replace, not push
				expect(replaceSpy).toHaveBeenCalled();
				expect(pushSpy).not.toHaveBeenCalled();

				// Verify it was called with a URL containing our path
				expect(replaceSpy).toHaveBeenCalledWith(
					expect.stringContaining("/replace-test"),
					undefined,
				);
			});
		});
	});
});

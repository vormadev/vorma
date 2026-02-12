import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	analyzeHistoryListenerPrelude,
	customHistoryListener,
	HistoryManager,
} from "../../platform/history.ts";
import { setNavigationStateAccess } from "../../app/context.ts";
import { addLocationListener } from "../../platform/events.ts";

type HistoryLikeLocation = {
	pathname: string;
	search: string;
	hash: string;
	state: unknown;
	key: string;
};

function createHistoryLikeLocation(
	overrides: Partial<HistoryLikeLocation>,
): HistoryLikeLocation {
	return {
		pathname: "/",
		search: "",
		hash: "",
		state: null,
		key: "key",
		...overrides,
	};
}

beforeEach(() => {
	window.history.replaceState({}, "", "/");
	HistoryManager.getInstance();
});

describe("history_listener_prelude", () => {
	it("treats POP updates on the same data target as same-document", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "POP" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
		});

		expect(result.didLocationKeyChange).toBe(true);
		expect(result.popWithinSameDoc).toBe(true);
		expect(result.shouldSaveScrollState).toBe(false);
	});

	it("treats query-order changes as different POP targets", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "POP" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?b=2&a=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?a=1&b=2",
			},
		});

		expect(result.popWithinSameDoc).toBe(false);
		expect(result.shouldSaveScrollState).toBe(true);
	});

	it("never flags non-POP actions as same-document POP", () => {
		const result = analyzeHistoryListenerPrelude({
			action: "PUSH" as any,
			location: {
				key: "next-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
			lastKnownLocation: {
				key: "prev-key",
				pathname: "/same-doc",
				search: "?mode=1",
			},
		});

		expect(result.popWithinSameDoc).toBe(false);
		expect(result.shouldSaveScrollState).toBe(true);
	});

	it("reloads the browser when cross-document POP navigation cannot be handled by client navigation", async () => {
		const navigate = vi.fn().mockResolvedValue({ didNavigate: false });
		setNavigationStateAccess({
			navigate,
			removeNavigation: vi.fn(),
			getNavigations: vi.fn(() => new Map()),
		});

		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		HistoryManager.updateLastKnownLocation(
			createHistoryLikeLocation({
				pathname: "/from",
				key: "from-key",
			}) as any,
		);

		await customHistoryListener({
			action: "POP" as any,
			location: createHistoryLikeLocation({
				pathname: "/to",
				search: "?view=next",
				hash: "#section",
				key: "to-key",
			}) as any,
		});

		expect(navigate).toHaveBeenCalledWith(
			expect.objectContaining({
				href: "http://localhost:3000/to?view=next#section",
				navigationType: "browserHistory",
			}),
		);
		expect(consoleErrorSpy).toHaveBeenCalled();
		expect(HistoryManager.getLastKnownLocation().key).toBe("from-key");
	});

	it("keeps history location in sync after successful cross-document POP navigation", async () => {
		const navigate = vi.fn().mockResolvedValue({ didNavigate: true });
		setNavigationStateAccess({
			navigate,
			removeNavigation: vi.fn(),
			getNavigations: vi.fn(() => new Map()),
		});

		const nextLocation = createHistoryLikeLocation({
			pathname: "/target",
			search: "?x=1",
			key: "target-key",
		});
		HistoryManager.updateLastKnownLocation(
			createHistoryLikeLocation({
				pathname: "/origin",
				key: "origin-key",
			}) as any,
		);

		await customHistoryListener({
			action: "POP" as any,
			location: nextLocation as any,
		});

		expect(navigate).toHaveBeenCalledTimes(1);
		expect(HistoryManager.getLastKnownLocation().key).toBe("target-key");
	});

	it("does not dispatch location events when history keys do not change", async () => {
		const navigate = vi.fn().mockResolvedValue({ didNavigate: true });
		setNavigationStateAccess({
			navigate,
			removeNavigation: vi.fn(),
			getNavigations: vi.fn(() => new Map()),
		});

		let locationEventCount = 0;
		const cleanupLocationListener = addLocationListener(() => {
			locationEventCount++;
		});

		try {
			const sameKeyLocation = createHistoryLikeLocation({
				pathname: "/same-key",
				key: "same-key",
			});
			HistoryManager.updateLastKnownLocation(sameKeyLocation as any);

			await customHistoryListener({
				action: "PUSH" as any,
				location: sameKeyLocation as any,
			});
		} finally {
			cleanupLocationListener();
		}

		expect(locationEventCount).toBe(0);
		expect(navigate).not.toHaveBeenCalled();
		expect(HistoryManager.getLastKnownLocation().key).toBe("same-key");
	});
});

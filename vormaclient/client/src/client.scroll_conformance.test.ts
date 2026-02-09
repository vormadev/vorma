import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	navigationStateManager,
} from "./client";
import {
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";
import {
	HistoryManager,
	customHistoryListener,
} from "./history/history.ts";
import {
	initClient,
} from "./init_client.ts";
import {
	scrollStateManager,
} from "./scroll_state_manager.ts";

type HistoryLoc = {
	pathname: string;
	search: string;
	hash: string;
	key: string;
	state?: unknown;
};

const SCROLL_MAP_KEY = "__vorma__scrollStateMap";
const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

function mkLoc(partial?: Partial<HistoryLoc>): HistoryLoc {
	return {
		pathname: "/",
		search: "",
		hash: "",
		key: "k0",
		state: {},
		...(partial || {}),
	};
}

function setLastKnown(loc: HistoryLoc) {
	HistoryManager.updateLastKnownLocation(loc as any);
}

describeNavigationTestSuite(() => {
	describe("History/scroll conformance", () => {
		it("FEC-SCROLL-001_FE-SCROLL-001_FE-SCROLL-002_storage_uses_documented_keys_and_caps_map_at_50_fifo", () => {
			for (let i = 0; i < 51; i++) {
				scrollStateManager.saveState(`fifo-${i}`, { x: i, y: i });
			}
			(window as any).scrollX = 12;
			(window as any).scrollY = 34;
			scrollStateManager.savePageRefreshState();

			const storedMap = JSON.parse(
				sessionStorage.getItem(SCROLL_MAP_KEY) || "[]",
			) as Array<[string, { x: number; y: number }]>;
			const storedRefresh = JSON.parse(
				sessionStorage.getItem(PAGE_REFRESH_KEY) || "{}",
			) as {
				x: number;
				y: number;
				href: string;
				unix: number;
			};

			expect(storedMap.length).toBe(50);
			expect(storedMap[0]?.[0]).toBe("fifo-1");
			expect(storedMap.at(-1)?.[0]).toBe("fifo-50");
			expect(storedRefresh).toMatchObject({
				x: 12,
				y: 34,
				href: window.location.href,
			});
			expect(typeof storedRefresh.unix).toBe("number");
		});

		it("FEC-SCROLL-002_FE-SCROLL-003_listener_saves_prior_scroll_before_navigation_away", async () => {
			setLastKnown(
				mkLoc({
					pathname: "/from",
					key: "prior-key",
				}),
			);
			(window as any).scrollX = 101;
			(window as any).scrollY = 202;

			await customHistoryListener({
				action: "PUSH",
				location: mkLoc({
					pathname: "/to",
					key: "next-key",
				}),
			} as any);

			const saved = scrollStateManager.getState("prior-key");
			expect(saved).toEqual({ x: 101, y: 202 });
		});

		it("FEC-SCROLL-003_FE-SCROLL-004_pop_same_document_hash_add_update_remove_scrolls_to_hash_or_restores_saved_or_top", async () => {
			const el = document.createElement("div");
			el.id = "scroll-hash-target";
			document.body.appendChild(el);
			const scrollIntoViewSpy = vi.spyOn(el, "scrollIntoView");

			setLastKnown(
				mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "",
					key: "doc-key-no-hash",
				}),
			);
			await customHistoryListener({
				action: "POP",
				location: mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "#scroll-hash-target",
					key: "doc-key-hash",
				}),
			} as any);
			expect(scrollIntoViewSpy).toHaveBeenCalled();

			scrollStateManager.saveState("doc-key-no-hash", { x: 77, y: 155 });
			setLastKnown(
				mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "#scroll-hash-target",
					key: "doc-key-hash",
				}),
			);
			await customHistoryListener({
				action: "POP",
				location: mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "",
					key: "doc-key-no-hash",
				}),
			} as any);
			expect(window.scrollTo).toHaveBeenCalledWith(77, 155);

			setLastKnown(
				mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "#scroll-hash-target",
					key: "doc-key-hash-2",
				}),
			);
			await customHistoryListener({
				action: "POP",
				location: mkLoc({
					pathname: "/docs",
					search: "?q=1",
					hash: "",
					key: "missing-key-for-top-fallback",
				}),
			} as any);
			expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
		});

		it("FEC-SCROLL-004_FE-SCROLL-005_pop_different_document_runs_browser_history_navigation_and_hard_reloads_on_failure", async () => {
			scrollStateManager.saveState("target-key", { x: 5, y: 9 });
			setLastKnown(
				mkLoc({
					pathname: "/from",
					key: "from-key",
				}),
			);

			const navigateSpy = vi.spyOn(navigationStateManager, "navigate");
			const reloadSpy = vi.spyOn(window.location, "reload");

			navigateSpy.mockResolvedValueOnce({ didNavigate: true });
			await customHistoryListener({
				action: "POP",
				location: mkLoc({
					pathname: "/to",
					key: "target-key",
				}),
			} as any);

			expect(navigateSpy).toHaveBeenCalledWith(
				expect.objectContaining({
					href: window.location.href,
					navigationType: "browserHistory",
					scrollStateToRestore: { x: 5, y: 9 },
				}),
			);
			expect(reloadSpy).not.toHaveBeenCalled();

			navigateSpy.mockResolvedValueOnce({ didNavigate: false });
			await customHistoryListener({
				action: "POP",
				location: mkLoc({
					pathname: "/to-fail",
					key: "target-key-fail",
				}),
			} as any);

			expect(reloadSpy).toHaveBeenCalled();
		});

		it("FEC-SCROLL-005_FE-SCROLL-006_FE-SCROLL-007_init_restores_fresh_same_url_refresh_state_only_and_sets_manual_scroll_restoration", async () => {
			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb: FrameRequestCallback) => {
					cb(0);
					return 1;
				});

			let restorationValue = "auto";
			Object.defineProperty(window.history, "scrollRestoration", {
				get: () => restorationValue,
				set: (v) => {
					restorationValue = v;
				},
				configurable: true,
			});

			sessionStorage.setItem(
				PAGE_REFRESH_KEY,
				JSON.stringify({
					x: 333,
					y: 444,
					unix: Date.now() - 1000,
					href: window.location.href,
				}),
			);
			setupGlobalVormaContext();
			await initClient({ renderFn: () => {}, vormaAppConfig });
			expect(window.scrollTo).toHaveBeenCalledWith(333, 444);
			expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
			expect(restorationValue).toBe("manual");

			vi.clearAllMocks();
			restorationValue = "auto";
			sessionStorage.setItem(
				PAGE_REFRESH_KEY,
				JSON.stringify({
					x: 100,
					y: 200,
					unix: Date.now() - 1000,
					href: "http://localhost:3000/other-url",
				}),
			);
			setupGlobalVormaContext();
			await initClient({ renderFn: () => {}, vormaAppConfig });
			expect(window.scrollTo).not.toHaveBeenCalledWith(100, 200);

			vi.clearAllMocks();
			restorationValue = "auto";
			sessionStorage.setItem(
				PAGE_REFRESH_KEY,
				JSON.stringify({
					x: 222,
					y: 333,
					unix: Date.now() - 6000,
					href: window.location.href,
				}),
			);
			setupGlobalVormaContext();
			await initClient({ renderFn: () => {}, vormaAppConfig });
			expect(window.scrollTo).not.toHaveBeenCalledWith(222, 333);

			rafSpy.mockRestore();
		});
	});
});

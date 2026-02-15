import { describe, expect, it, vi } from "vitest";
import { restoreRecentPageRefreshScrollState } from "../../platform/scroll.ts";

const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

describe("scroll_state_refresh_state", () => {
	it("removes malformed page-refresh snapshots", () => {
		sessionStorage.setItem(PAGE_REFRESH_KEY, "{not-json");
		const applyState = vi.fn();

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
	});

	it("removes parseable but invalid snapshot shapes", () => {
		sessionStorage.setItem(
			PAGE_REFRESH_KEY,
			JSON.stringify({
				x: "not-a-number",
				y: 100,
				unix: Date.now(),
				href: window.location.href,
			}),
		);
		const applyState = vi.fn();

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
	});

	it("removes parseable non-object snapshots", () => {
		sessionStorage.setItem(PAGE_REFRESH_KEY, JSON.stringify(123));
		const applyState = vi.fn();

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
	});

	it("restores recent snapshots for encoding-equivalent same-document hash URLs", () => {
		window.history.replaceState({}, "", "/refresh-hash#~");
		vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
			cb(0);
			return 0;
		});

		sessionStorage.setItem(
			PAGE_REFRESH_KEY,
			JSON.stringify({
				x: 123,
				y: 456,
				unix: Date.now(),
				href: "http://localhost:3000/refresh-hash#%7E",
			}),
		);
		const applyState = vi.fn();

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).toHaveBeenCalledWith({ x: 123, y: 456 });
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
	});

	it("removes snapshots that are stale or for a different URL", () => {
		window.history.replaceState({}, "", "/refresh-current#one");
		const applyState = vi.fn();

		sessionStorage.setItem(
			PAGE_REFRESH_KEY,
			JSON.stringify({
				x: 10,
				y: 20,
				unix: Date.now() - 10_000,
				href: "http://localhost:3000/refresh-current#one",
			}),
		);

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();

		sessionStorage.setItem(
			PAGE_REFRESH_KEY,
			JSON.stringify({
				x: 30,
				y: 40,
				unix: Date.now(),
				href: "http://localhost:3000/refresh-other#one",
			}),
		);

		restoreRecentPageRefreshScrollState(applyState);

		expect(applyState).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(PAGE_REFRESH_KEY)).toBeNull();
	});
});

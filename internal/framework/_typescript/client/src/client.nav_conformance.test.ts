import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	beginNavigation,
	getStatus,
	navigationStateManager,
	submit,
} from "./client";
import {
	addRouteChangeListener,
	addStatusListener,
} from "./events.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

type PendingFetch = {
	resolve: (response: Response) => void;
	reject: (error: unknown) => void;
};

function isIdleStatus(status: {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
}) {
	return !status.isNavigating && !status.isSubmitting && !status.isRevalidating;
}

function sameStatus(
	a: {
		isNavigating: boolean;
		isSubmitting: boolean;
		isRevalidating: boolean;
	},
	b: {
		isNavigating: boolean;
		isSubmitting: boolean;
		isRevalidating: boolean;
	},
) {
	return (
		a.isNavigating === b.isNavigating &&
		a.isSubmitting === b.isSubmitting &&
		a.isRevalidating === b.isRevalidating
	);
}

function baseRouteData(overrides?: Record<string, any>) {
	return {
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		errorExportKeys: [],
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		deps: [],
		cssBundles: [],
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		...(overrides || {}),
	};
}

function setupAbortableFetchHarness(): PendingFetch[] {
	const pending: PendingFetch[] = [];
	vi.mocked(fetch).mockImplementation((_, init?: RequestInit) => {
		return new Promise<Response>((resolve, reject) => {
			const signal = init?.signal;
			if (signal?.aborted) {
				reject(new DOMException("Aborted", "AbortError"));
				return;
			}
			const abortHandler = () =>
				reject(new DOMException("Aborted", "AbortError"));
			signal?.addEventListener("abort", abortHandler, { once: true });

			pending.push({
				resolve: (response) => resolve(response),
				reject,
			});
		});
	}) as any;
	return pending;
}

describeNavigationTestSuite(({ addListener }) => {
	describe("Navigation state-machine conformance", () => {
		it("FEC-NAV-001_FE-NAV-001_runtime_supports_declared_navigation_types", async () => {
			const mkResponse = () =>
				createMockResponse(baseRouteData(), {
					headers: { "X-Vorma-Build-Id": "build-nav-1" },
				});
			vi.mocked(fetch).mockImplementation(() => Promise.resolve(mkResponse()));

			const cases: Array<{
				navType:
					| "userNavigation"
					| "browserHistory"
					| "revalidation"
					| "redirect"
					| "prefetch"
					| "action";
				href: string;
			}> = [
				{
					navType: "userNavigation",
					href: "/nav-supported-user",
				},
				{
					navType: "browserHistory",
					href: "/nav-supported-pop",
				},
				{
					navType: "revalidation",
					href: window.location.href,
				},
				{
					navType: "redirect",
					href: "/nav-supported-redirect",
				},
				{
					navType: "prefetch",
					href: "/nav-supported-prefetch",
				},
				{
					navType: "action",
					href: "/nav-supported-action",
				},
			];

			for (const tc of cases) {
				const control = beginNavigation({
					href: tc.href,
					navigationType: tc.navType,
				});
				expect(control.promise).toBeTruthy();
				const outcome = await control.promise;
				expect(["success", "redirect", "aborted"]).toContain(outcome.type);
			}
			expect(vi.mocked(fetch).mock.calls.length).toBeGreaterThanOrEqual(6);
		});

		it("FEC-NAV-002_FE-NAV-002_FE-NAV-004_same_target_user_navigation_reuses_single_active_slot_and_control", async () => {
			const pending = setupAbortableFetchHarness();
			const target = new URL("/nav-reuse", window.location.href).href;

			const controlA = beginNavigation({
				href: target,
				navigationType: "userNavigation",
			});
			const controlB = beginNavigation({
				href: target,
				navigationType: "userNavigation",
			});

			expect(controlB).toBe(controlA);
			expect(controlB.promise).toBe(controlA.promise);
			expect(navigationStateManager.getNavigationsSize()).toBe(1);
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);

			pending[0]?.resolve(createMockResponse(baseRouteData()));
			await controlA.promise;
			navigationStateManager.removeNavigation(target);
		});

		it("FEC-NAV-003_FE-NAV-003_new_user_navigation_aborts_conflicting_active_prefetch_and_revalidation_work", async () => {
			const pending = setupAbortableFetchHarness();
			const targetA = new URL("/nav-a", window.location.href).href;
			const prefetchTarget = new URL(
				"/nav-prefetch",
				window.location.href,
			).href;
			const targetB = new URL("/nav-b", window.location.href).href;

			const activeControl = beginNavigation({
				href: targetA,
				navigationType: "userNavigation",
			});
			activeControl.promise.catch(() => {});

			const prefetchControl = beginNavigation({
				href: prefetchTarget,
				navigationType: "prefetch",
			});
			prefetchControl.promise.catch(() => {});

			const revalidateControl = beginNavigation({
				href: window.location.href,
				navigationType: "revalidation",
			});
			revalidateControl.promise.catch(() => {});

			const newControl = beginNavigation({
				href: targetB,
				navigationType: "userNavigation",
			});

			expect(activeControl.abortController?.signal.aborted).toBe(true);
			expect(prefetchControl.abortController?.signal.aborted).toBe(true);
			expect(revalidateControl.abortController?.signal.aborted).toBe(true);
			expect(navigationStateManager.getNavigationsSize()).toBe(1);
			expect(navigationStateManager.getNavigation(targetB)?.type).toBe(
				"userNavigation",
			);

			pending[3]?.resolve(createMockResponse(baseRouteData()));
			await newControl.promise;
			navigationStateManager.removeNavigation(targetB);
		});

		it("FEC-NAV-004_FE-NAV-005_FE-NAV-006_prefetch_deduplicates_and_upgrades_to_user_navigation", async () => {
			const pending = setupAbortableFetchHarness();
			const target = new URL("/nav-upgrade", window.location.href).href;

			const prefetchA = beginNavigation({
				href: target,
				navigationType: "prefetch",
			});
			const prefetchB = beginNavigation({
				href: target,
				navigationType: "prefetch",
			});

			expect(prefetchB.promise).toBe(prefetchA.promise);
			expect(vi.mocked(fetch).mock.calls).toHaveLength(1);

			const upgraded = beginNavigation({
				href: target,
				navigationType: "userNavigation",
			});
			expect(upgraded.promise).toBe(prefetchA.promise);
			expect(navigationStateManager.getNavigation(target)?.type).toBe(
				"userNavigation",
			);
			expect(navigationStateManager.getNavigation(target)?.intent).toBe(
				"navigate",
			);

			pending[0]?.resolve(createMockResponse(baseRouteData()));
			await upgraded.promise;
			navigationStateManager.removeNavigation(target);
		});

		it("FEC-NAV-005_FE-NAV-007_prefetch_for_current_document_is_noop_without_network_fetch", async () => {
			const control = beginNavigation({
				href: `${window.location.href}#fragment`,
				navigationType: "prefetch",
			});

			const outcome = await control.promise;
			expect(outcome.type).toBe("aborted");
			expect(vi.mocked(fetch).mock.calls).toHaveLength(0);
		});

		it("FEC-NAV-006_FE-NAV-008_FE-NAV-009_revalidation_coalesces_and_does_not_render_on_origin_change", async () => {
			const pending = setupAbortableFetchHarness();
			const routeChangeSpy = vi.fn();
			addListener(addRouteChangeListener, routeChangeSpy);

			const controlA = beginNavigation({
				href: window.location.href,
				navigationType: "revalidation",
			});
			const controlB = beginNavigation({
				href: window.location.href,
				navigationType: "revalidation",
			});
			expect(controlB.promise).toBe(controlA.promise);

			pending[0]?.resolve(createMockResponse(baseRouteData()));
			await controlA.promise;
			navigationStateManager.removeNavigation(window.location.href);

			setupGlobalVormaContext({
				hasRootData: true,
				loadersData: ["old-root-data"],
				matchedPatterns: ["/old"],
				importURLs: [],
				exportKeys: [],
				params: {},
				splatValues: [],
			});
			const pendingOrigin = setupAbortableFetchHarness();

			const navPromise = navigationStateManager.navigate({
				href: window.location.href,
				navigationType: "revalidation",
			});

			window.history.replaceState({}, "", "/different-origin");

			pendingOrigin[0]?.resolve(
				createMockResponse(
					baseRouteData({
						hasRootData: true,
						loadersData: ["new-root-data"],
						matchedPatterns: ["/new"],
					}),
				),
			);
			await navPromise;

			expect(routeChangeSpy).not.toHaveBeenCalled();
			expect(__vormaClientGlobal.get("loadersData")).toEqual([
				"old-root-data",
			]);
		});

		it("FEC-NAV-007_FE-NAV-010_FE-NAV-012_status_flags_reflect_true_active_work_and_clear_after_abort_completion", async () => {
			const pending = setupAbortableFetchHarness();

			expect(getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});

			const navA = navigationStateManager.navigate({
				href: "/status-nav-a",
				navigationType: "userNavigation",
			});
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isNavigating).toBe(true);
			expect(getStatus().isRevalidating).toBe(false);

			const revalidateA = navigationStateManager.navigate({
				href: window.location.href,
				navigationType: "revalidation",
			});
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isNavigating).toBe(true);
			expect(getStatus().isRevalidating).toBe(true);

			const navB = navigationStateManager.navigate({
				href: "/status-nav-b",
				navigationType: "userNavigation",
			});
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isNavigating).toBe(true);
			expect(getStatus().isRevalidating).toBe(false);

			pending[2]?.resolve(createMockResponse(baseRouteData()));
			await Promise.all([navA, revalidateA, navB]);
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});

			const skippedSubmission = submit(
				"/api/submit-skip-status",
				{
					method: "POST",
					body: JSON.stringify({ ok: true }),
					headers: {
						"Content-Type": "application/json",
					},
				},
				{
					revalidate: false,
					skipGlobalLoadingIndicator: true,
				},
			);
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isSubmitting).toBe(false);

			const regularSubmission = submit(
				"/api/submit-regular-status",
				{
					method: "POST",
					body: JSON.stringify({ ok: true }),
					headers: {
						"Content-Type": "application/json",
					},
				},
				{
					revalidate: false,
				},
			);
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isSubmitting).toBe(true);

			pending[3]?.resolve(createMockResponse({ ok: true }));
			pending[4]?.resolve(createMockResponse({ ok: true }));
			await Promise.all([skippedSubmission, regularSubmission]);
			await vi.advanceTimersByTimeAsync(16);

			expect(getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		});

		it("FEC-NAV-008_FE-NAV-011_FE-NAV-013_status_events_are_deduped_and_chained_submit_revalidate_has_no_intermediate_idle_gap", async () => {
			const pending = setupAbortableFetchHarness();
			const statusEvents: Array<{
				isNavigating: boolean;
				isSubmitting: boolean;
				isRevalidating: boolean;
			}> = [];
			addListener(addStatusListener, (event) => {
				statusEvents.push(event.detail);
			});

			const submitPromise = submit(
				"/api/submit-chain",
				{
					method: "POST",
					body: JSON.stringify({ chain: true }),
					headers: {
						"Content-Type": "application/json",
					},
				},
				{
					revalidate: true,
				},
			);

			await vi.advanceTimersByTimeAsync(16);
			pending[0]?.resolve(createMockResponse({ mutation: "ok" }));
			await vi.advanceTimersByTimeAsync(16);
			expect(getStatus().isSubmitting || getStatus().isRevalidating).toBe(
				true,
			);

			pending[1]?.resolve(createMockResponse(baseRouteData()));
			await submitPromise;
			await vi.advanceTimersByTimeAsync(24);

			expect(statusEvents.length).toBeGreaterThan(0);
			for (let i = 1; i < statusEvents.length; i++) {
				const prev = statusEvents[i - 1];
				const curr = statusEvents[i];
				if (!prev || !curr) continue;
				expect(sameStatus(prev, curr)).toBe(false);
			}

			const nonFinalIdleGap = statusEvents
				.slice(0, Math.max(0, statusEvents.length - 1))
				.some((s) => isIdleStatus(s));
			expect(nonFinalIdleGap).toBe(false);
			expect(isIdleStatus(statusEvents.at(-1)!)).toBe(true);
		});
	});
});

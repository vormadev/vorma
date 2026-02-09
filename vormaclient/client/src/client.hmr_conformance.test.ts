import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";
import {
	addRouteChangeListener,
} from "./events.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

async function loadHMRModule() {
	vi.resetModules();
	return import("./hmr/hmr.ts");
}

describeNavigationTestSuite(() => {
	describe("HMR and focus revalidation conformance", () => {
		it("FEC-HMR-001_FE-HMR-001_FE-HMR-003_dev_revalidate_hook_and_module_listener_registration_are_stable", async () => {
			const originalDev = import.meta.env.DEV;
			const hotOn = vi.fn();

			(import.meta.env as any).DEV = true;

			const hmrModule = await loadHMRModule();
			const clientModule = await import("./client");
			hmrModule.initHMR();

			expect((window as any).__waveRevalidate).toBe(
				clientModule.revalidate,
			);

			hmrModule.__runClientLoadersAfterHMRUpdate(
				{
					url: "http://localhost:3000/src/routes/user.ts?one",
					hot: { on: hotOn },
				} as ImportMeta,
				"/users/:id",
			);
			hmrModule.__runClientLoadersAfterHMRUpdate(
				{
					url: "http://localhost:3000/src/routes/user.ts?two",
					hot: { on: hotOn },
				} as ImportMeta,
				"/users/:id",
			);

			expect(hotOn).toHaveBeenCalledTimes(1);

			(import.meta.env as any).DEV = originalDev;
		});

		it("FEC-HMR-002_FE-HMR-002_matching_js_update_debounces_client_loader_rerun_and_dispatches_route_change", async () => {
			const originalDev = import.meta.env.DEV;
			let afterUpdateHandler: ((props: any) => void) | undefined;
			const hotOn = vi.fn((event: string, cb: (props: any) => void) => {
				if (event === "vite:afterUpdate") {
					afterUpdateHandler = cb;
				}
			});

			(import.meta.env as any).DEV = true;

			const waitFn = vi.fn().mockResolvedValue("hmr-client-data");
			setupGlobalVormaContext({
				matchedPatterns: ["/users/:id"],
				loadersData: [{ from: "server" }],
				importURLs: [],
				exportKeys: [],
				hasRootData: false,
				params: { id: "1" },
				splatValues: [],
				patternToWaitFnMap: { "/users/:id": waitFn },
			});

			const routeChangeListener = vi.fn();
			const cleanupRouteChange = addRouteChangeListener(routeChangeListener);

			const hmrModule = await loadHMRModule();
			hmrModule.initHMR();
			hmrModule.__runClientLoadersAfterHMRUpdate(
				{
					url: "http://localhost:3000/src/routes/user.ts",
					hot: { on: hotOn },
				} as ImportMeta,
				"/users/:id",
			);
			expect(afterUpdateHandler).toBeTypeOf("function");

			afterUpdateHandler?.({
				updates: [
					{
						type: "js-update",
						path: "/src/routes/user.ts?t=123",
					},
				],
			});

			await vi.advanceTimersByTimeAsync(20);
			await Promise.resolve();

			expect(waitFn).toHaveBeenCalledTimes(1);
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				"hmr-client-data",
			]);
			expect(routeChangeListener).toHaveBeenCalledTimes(1);

			cleanupRouteChange();
			(import.meta.env as any).DEV = originalDev;
		});

		it("FEC-HMR-003_FE-HMR-004_focus_revalidation_runs_only_when_idle_and_stale_time_elapsed", async () => {
			let currentStatus = {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			};
			let lastTriggered = Date.now();
			const revalidateSpy = vi.fn();

			vi.resetModules();
			vi.doMock("./client.ts", () => ({
				getStatus: () => currentStatus,
				getLastTriggeredNavOrRevalidateTimestampMS: () => lastTriggered,
				revalidate: revalidateSpy,
			}));
			const { revalidateOnWindowFocus } = await import(
				"./window_focus_revalidation/window_focus_revalidation.ts"
			);

			const cleanup = revalidateOnWindowFocus({ staleTimeMS: 1000 });

			// Busy: MUST NOT revalidate.
			currentStatus = {
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: false,
			};
			window.dispatchEvent(new Event("focus"));
			await vi.advanceTimersByTimeAsync(40);
			expect(revalidateSpy).not.toHaveBeenCalled();

			// Idle but fresh: MUST NOT revalidate.
			currentStatus = {
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			};
			lastTriggered = Date.now();
			window.dispatchEvent(new Event("focus"));
			await vi.advanceTimersByTimeAsync(40);
			expect(revalidateSpy).not.toHaveBeenCalled();

			// Idle and stale: MUST revalidate.
			lastTriggered = Date.now() - 1001;
			window.dispatchEvent(new Event("focus"));
			await vi.advanceTimersByTimeAsync(40);
			expect(revalidateSpy).toHaveBeenCalledTimes(1);

			cleanup();
			vi.doUnmock("./client.ts");
			vi.resetModules();
		});
	});
});

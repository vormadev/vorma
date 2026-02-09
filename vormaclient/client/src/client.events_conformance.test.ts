import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";
import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	dispatchBuildIDEvent,
	dispatchLocationEvent,
	dispatchRouteChangeEvent,
	dispatchStatusEvent,
} from "./events.ts";
import {
	customHistoryListener,
	HistoryManager,
} from "./history/history.ts";

describeNavigationTestSuite(() => {
	describe("Events conformance", () => {
		it("FEC-EVT-001_FE-EVT-001_runtime_uses_canonical_window_event_names", () => {
			const addSpy = vi.spyOn(window, "addEventListener");
			const dispatchSpy = vi.spyOn(window, "dispatchEvent");

			const statusListener = vi.fn();
			const routeChangeListener = vi.fn();
			const locationListener = vi.fn();
			const buildIDListener = vi.fn();

			const cleanupStatus = addStatusListener(statusListener);
			const cleanupRoute = addRouteChangeListener(routeChangeListener);
			const cleanupLocation = addLocationListener(locationListener);
			const cleanupBuild = addBuildIDListener(buildIDListener);

			expect(addSpy).toHaveBeenCalledWith(
				"vorma:status",
				expect.any(Function),
			);
			expect(addSpy).toHaveBeenCalledWith(
				"vorma:route-change",
				expect.any(Function),
			);
			expect(addSpy).toHaveBeenCalledWith(
				"vorma:location",
				expect.any(Function),
			);
			expect(addSpy).toHaveBeenCalledWith(
				"vorma:build-id",
				expect.any(Function),
			);

			dispatchStatusEvent({
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: false,
			});
			dispatchRouteChangeEvent({ __scrollState: { x: 11, y: 22 } });
			dispatchLocationEvent();
			dispatchBuildIDEvent({ oldID: "old", newID: "new" });

			expect(statusListener).toHaveBeenCalledTimes(1);
			expect(routeChangeListener).toHaveBeenCalledTimes(1);
			expect(locationListener).toHaveBeenCalledTimes(1);
			expect(buildIDListener).toHaveBeenCalledTimes(1);

			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({ type: "vorma:status" }),
			);
			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({ type: "vorma:route-change" }),
			);
			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({ type: "vorma:location" }),
			);
			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({ type: "vorma:build-id" }),
			);

			cleanupStatus();
			cleanupRoute();
			cleanupLocation();
			cleanupBuild();
		});

		it("FEC-EVT-002_FE-EVT-002_listener_adders_return_cleanup_that_unregisters_the_same_listener", () => {
			const cases: Array<{
				add: (fn: (event: any) => void) => () => void;
				dispatch: () => void;
			}> = [
				{
					add: addStatusListener,
					dispatch: () =>
						dispatchStatusEvent({
							isNavigating: false,
							isSubmitting: false,
							isRevalidating: false,
						}),
				},
				{
					add: addRouteChangeListener,
					dispatch: () => dispatchRouteChangeEvent({ __scrollState: undefined }),
				},
				{
					add: addLocationListener,
					dispatch: () => dispatchLocationEvent(),
				},
				{
					add: addBuildIDListener,
					dispatch: () =>
						dispatchBuildIDEvent({
							oldID: "before",
							newID: "after",
						}),
				},
			];

			for (const tc of cases) {
				const listener = vi.fn();
				const cleanup = tc.add(listener);
				tc.dispatch();
				expect(listener).toHaveBeenCalledTimes(1);

				cleanup();
				tc.dispatch();
				expect(listener).toHaveBeenCalledTimes(1);
			}
		});

		it("FEC-EVT-003_FE-EVT-003_history_location_key_change_dispatches_vorma_location_event", async () => {
			const prior = HistoryManager.getLastKnownLocation();
			HistoryManager.updateLastKnownLocation({
				...prior,
				key: "evt-old-key",
			} as typeof prior);

			const locationListener = vi.fn();
			const cleanup = addLocationListener(locationListener);

			await customHistoryListener({
				action: "PUSH",
				location: {
					pathname: "/evt-path",
					search: "",
					hash: "",
					state: null,
					key: "evt-new-key",
				},
			} as any);

			expect(locationListener).toHaveBeenCalledTimes(1);

			await customHistoryListener({
				action: "PUSH",
				location: {
					pathname: "/evt-path",
					search: "",
					hash: "",
					state: null,
					key: "evt-new-key",
				},
			} as any);

			expect(locationListener).toHaveBeenCalledTimes(1);
			cleanup();
		});
	});
});

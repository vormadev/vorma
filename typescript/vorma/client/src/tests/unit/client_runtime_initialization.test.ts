import { describe, expect, it, vi } from "vitest";

describe("client runtime initialization", () => {
	it("does not initialize navigation state access during module import", async () => {
		vi.resetModules();
		const contextModule = await import("../../app/context.ts");

		expect(() => contextModule.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);

		await import("../../client.ts");

		expect(() => contextModule.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);
	});

	it("initializes navigation state access on first runtime usage", async () => {
		vi.resetModules();
		const contextModule = await import("../../app/context.ts");
		const clientModule = await import("../../client.ts");

		expect(() => contextModule.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);

		expect(clientModule.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});

		expect(() => contextModule.getNavigationStateAccess()).not.toThrow();
		expect(contextModule.getNavigationStateAccess()).toMatchObject({
			navigate: expect.any(Function),
			removeNavigation: expect.any(Function),
			getNavigations: expect.any(Function),
		});
	});

	it("initializes navigation state access when history instance is requested", async () => {
		vi.resetModules();
		const contextModule = await import("../../app/context.ts");
		const clientModule = await import("../../client.ts");

		expect(() => contextModule.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);

		clientModule.getUnsafeHistoryInstance();

		expect(() => contextModule.getNavigationStateAccess()).not.toThrow();
	});

	it("keeps debug journal APIs stable and no-op when dev mode is disabled", async () => {
		vi.resetModules();
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;

		try {
			const clientModule = await import("../../client.ts");
			expect(clientModule.getNavigationDebugJournal()).toEqual([]);

			const control = clientModule.navigationStateManager.beginNavigation(
				{
					href: window.location.href,
					navigationType: "prefetch",
				},
			);
			await control.promise;

			expect(clientModule.getNavigationDebugJournal()).toEqual([]);
			expect(() =>
				clientModule.clearNavigationDebugJournal(),
			).not.toThrow();
			expect(clientModule.getNavigationDebugJournal()).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});
});

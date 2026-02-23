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

		clientModule.getHistoryInstance();

		expect(() => contextModule.getNavigationStateAccess()).not.toThrow();
	});
});

import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	__getVormaClientGlobal,
	getClientRuntimeRenderState,
	setClientLoaderWaitFn,
	VORMA_SYMBOL,
} from "../../app/context.ts";

let dom: JSDOM;
let mockGlobal: any;

function setup() {
	dom = new JSDOM("<!DOCTYPE html><body></body>", {
		url: "https://example.com",
	});
	global.window = dom.window as unknown as Window & typeof globalThis;
	global.document = dom.window.document;
	mockGlobal = {};
	(globalThis as any)[VORMA_SYMBOL] = mockGlobal;
}

function teardown() {
	delete (globalThis as any)[VORMA_SYMBOL];
	dom.window.close();
	global.window = undefined as any;
	global.document = undefined as any;
}

describe("__getVormaClientGlobal", () => {
	beforeEach(setup);

	afterEach(teardown);

	it("should get a value from the global state", () => {
		mockGlobal.params = { key: "value" };
		const { get } = __getVormaClientGlobal();
		expect(get("params")).toEqual({ key: "value" });
	});

	it("should set a value in the global state", () => {
		const { set, get } = __getVormaClientGlobal();
		set("buildID", "123");
		expect(get("buildID")).toBe("123");
	});

	it("should update existing global values correctly", () => {
		mockGlobal.activeComponents = [];
		const { set, get } = __getVormaClientGlobal();
		set("activeComponents", ["Component1"]);
		expect(get("activeComponents")).toEqual(["Component1"]);
	});

	it("returns the current client runtime render state from global values", () => {
		const activeComponents = ["A", "B"];
		const clientLoadersData = ["C"];
		const importURLs = ["/a.js", "/b.js"];
		const exportKeys = ["default", "Route"];
		mockGlobal.loadersData = [{ value: 1 }];
		mockGlobal.clientLoadersData = clientLoadersData;
		mockGlobal.outermostError = "runtime-error";
		mockGlobal.outermostErrorIdx = 1;
		mockGlobal.activeComponents = activeComponents;
		mockGlobal.activeErrorBoundary = "Boundary";
		mockGlobal.importURLs = importURLs;
		mockGlobal.exportKeys = exportKeys;

		const runtimeState = getClientRuntimeRenderState();

		expect(runtimeState.loadersData).toEqual([{ value: 1 }]);
		expect(runtimeState.clientLoadersData).toBe(clientLoadersData);
		expect(runtimeState.outermostError).toBe("runtime-error");
		expect(runtimeState.outermostErrorIdx).toBe(1);
		expect(runtimeState.activeComponents).toBe(activeComponents);
		expect(runtimeState.activeErrorBoundary).toBe("Boundary");
		expect(runtimeState.importURLs).toBe(importURLs);
		expect(runtimeState.exportKeys).toBe(exportKeys);
	});

	it("merges client loader wait functions into patternToWaitFnMap", () => {
		const firstWaitFn = vi.fn();
		const secondWaitFn = vi.fn();

		expect(mockGlobal.patternToWaitFnMap).toBeUndefined();
		setClientLoaderWaitFn("/first", firstWaitFn);
		const firstMap = mockGlobal.patternToWaitFnMap;
		expect(firstMap).toEqual({ "/first": firstWaitFn });

		setClientLoaderWaitFn("/second", secondWaitFn);
		expect(mockGlobal.patternToWaitFnMap).toEqual({
			"/first": firstWaitFn,
			"/second": secondWaitFn,
		});
		expect(mockGlobal.patternToWaitFnMap).not.toBe(firstMap);
	});

	it("throws when navigation state access is requested before initialization", async () => {
		vi.resetModules();
		const context = await import("../../app/context.ts");

		expect(() => context.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);
	});

	it("returns navigation state access after it is initialized", async () => {
		vi.resetModules();
		const context = await import("../../app/context.ts");
		const access = {
			navigate: vi.fn().mockResolvedValue({ didNavigate: true }),
			removeNavigation: vi.fn(),
			getNavigations: vi.fn().mockReturnValue([]),
		};

		context.setNavigationStateAccess(access);

		expect(context.getNavigationStateAccess()).toBe(access);
	});
});

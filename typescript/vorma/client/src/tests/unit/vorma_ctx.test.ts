import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __getVormaClientGlobal, VORMA_SYMBOL } from "../../app/context.ts";

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

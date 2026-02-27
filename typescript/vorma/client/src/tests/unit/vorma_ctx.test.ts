import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	__getVormaClientGlobal,
	getClientRuntimeRenderState,
	getRuntimeRouteSnapshot,
	setClientLoaderWaitFn,
	setRuntimeRouteSnapshot,
	VORMA_SYMBOL,
} from "../../runtime.ts";

let dom: JSDOM;
let mockGlobal: any;

function createDefaultRuntimeRouteSnapshot() {
	return {
		buildID: "1",
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientError: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		rootElementID: "app",
		activeComponents: [],
		activeErrorBoundary: undefined,
		clientLoadersData: [],
	};
}

function setup() {
	dom = new JSDOM("<!DOCTYPE html><body></body>", {
		url: "https://example.com",
	});
	global.window = dom.window as unknown as Window & typeof globalThis;
	global.document = dom.window.document;
	mockGlobal = {
		runtimeRouteSnapshot: createDefaultRuntimeRouteSnapshot(),
	};
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
		mockGlobal.runtimeRouteSnapshot.params = { key: "value" };
		const { get } = __getVormaClientGlobal();
		expect(get("runtimeRouteSnapshot").params).toEqual({ key: "value" });
	});

	it("should set a value in the global state", () => {
		const { set, get } = __getVormaClientGlobal();
		set("runtimeRouteSnapshot", {
			...get("runtimeRouteSnapshot"),
			buildID: "123",
		});
		expect(get("runtimeRouteSnapshot").buildID).toBe("123");
	});

	it("should update existing global values correctly", () => {
		mockGlobal.runtimeRouteSnapshot.activeComponents = [];
		const { set, get } = __getVormaClientGlobal();
		set("runtimeRouteSnapshot", {
			...get("runtimeRouteSnapshot"),
			activeComponents: ["Component1"],
		});
		expect(get("runtimeRouteSnapshot").activeComponents).toEqual([
			"Component1",
		]);
	});

	it("returns the current client runtime render state from global values", () => {
		const activeComponents = ["A", "B"];
		const clientLoadersData = ["C"];
		const importURLs = ["/a.js", "/b.js"];
		const exportKeys = ["default", "Route"];
		mockGlobal.runtimeRouteSnapshot = {
			...mockGlobal.runtimeRouteSnapshot,
			loadersData: [{ value: 1 }],
			clientLoadersData,
			outermostError: "runtime-error",
			outermostErrorIdx: 1,
			activeComponents,
			activeErrorBoundary: "Boundary",
			importURLs,
			exportKeys,
		};

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
		const context = await import("../../runtime.ts");

		expect(() => context.getNavigationStateAccess()).toThrow(
			"Navigation state access has not been initialized.",
		);
	});

	it("returns navigation state access after it is initialized", async () => {
		vi.resetModules();
		const context = await import("../../runtime.ts");
		const access = {
			navigate: vi.fn().mockResolvedValue({ didNavigate: true }),
			removeNavigation: vi.fn(),
			getNavigations: vi.fn().mockReturnValue([]),
		};

		context.setNavigationStateAccess(access);

		expect(context.getNavigationStateAccess()).toBe(access);
	});

	it("throws a clear error when Vorma runtime globals are missing", async () => {
		delete (globalThis as any)[VORMA_SYMBOL];
		const { get } = __getVormaClientGlobal();

		expect(() => get("runtimeRouteSnapshot").buildID).toThrow(
			'Vorma client runtime state is not initialized on globalThis[Symbol.for("__vorma_internal__")].',
		);
	});

	it("fails loud for invalid route-field get/set keys passed via unsafe casts", () => {
		const { get, set } = __getVormaClientGlobal();
		const initialSnapshot = get("runtimeRouteSnapshot");

		expect(() => (get as any)("buildID")).toThrow(
			'Vorma client global get contract violated: unsupported key "buildID".',
		);
		expect(() => (set as any)("buildID", "unsafe")).toThrow(
			'Vorma client global set contract violated: unsupported key "buildID".',
		);
		expect(get("runtimeRouteSnapshot")).toBe(initialSnapshot);
	});

	it("canonicalizes runtime snapshots, freezes them, and keeps snapshot fields authoritative", () => {
		mockGlobal.runtimeRouteSnapshot = {
			...mockGlobal.runtimeRouteSnapshot,
			buildID: "1",
			matchedPatterns: ["/probe"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["client-a"],
			importURLs: ["/probe.js"],
			exportKeys: ["default"],
			errorExportKeys: [],
			hasRootData: true,
			params: { id: "1" },
			splatValues: ["tail"],
			activeComponents: ["ProbeComponent"],
			activeErrorBoundary: undefined,
		};

		const initialSnapshot = getRuntimeRouteSnapshot();
		expect(initialSnapshot).toBe(getRuntimeRouteSnapshot());
		expect(Object.isFrozen(initialSnapshot)).toBe(true);
		expect(Object.isFrozen(initialSnapshot.matchedPatterns)).toBe(true);
		expect(Object.isFrozen(initialSnapshot.loadersData)).toBe(true);
		expect(Object.isFrozen(initialSnapshot.params)).toBe(true);

		const nextSnapshot = setRuntimeRouteSnapshot({
			...initialSnapshot,
			loadersData: [{ value: "loader-b" }],
			matchedPatterns: [...initialSnapshot.matchedPatterns],
			importURLs: [...initialSnapshot.importURLs],
			exportKeys: [...initialSnapshot.exportKeys],
			errorExportKeys: [...initialSnapshot.errorExportKeys],
			params: { ...initialSnapshot.params },
			splatValues: [...initialSnapshot.splatValues],
			clientLoadersData: [...initialSnapshot.clientLoadersData],
		});

		expect(nextSnapshot).toBe(getRuntimeRouteSnapshot());
		expect(nextSnapshot).not.toBe(initialSnapshot);
		expect(nextSnapshot.matchedPatterns).toBe(
			initialSnapshot.matchedPatterns,
		);
		expect(nextSnapshot.importURLs).toBe(initialSnapshot.importURLs);
		expect(nextSnapshot.exportKeys).toBe(initialSnapshot.exportKeys);
		expect(nextSnapshot.errorExportKeys).toBe(
			initialSnapshot.errorExportKeys,
		);
		expect(nextSnapshot.params).toBe(initialSnapshot.params);
		expect(nextSnapshot.splatValues).toBe(initialSnapshot.splatValues);
		expect(nextSnapshot.clientLoadersData).toBe(
			initialSnapshot.clientLoadersData,
		);
		expect(nextSnapshot.loadersData).not.toBe(initialSnapshot.loadersData);
		expect(mockGlobal.runtimeRouteSnapshot).toBe(nextSnapshot);
		const { get } = __getVormaClientGlobal();
		expect(get("runtimeRouteSnapshot").matchedPatterns).toBe(
			nextSnapshot.matchedPatterns,
		);
		expect(get("runtimeRouteSnapshot").loadersData).toBe(
			nextSnapshot.loadersData,
		);
		expect(get("runtimeRouteSnapshot").clientLoadersData).toBe(
			nextSnapshot.clientLoadersData,
		);
		expect(get("runtimeRouteSnapshot").outermostError).toBe(
			nextSnapshot.outermostError,
		);
		expect(get("runtimeRouteSnapshot").outermostErrorIdx).toBe(
			nextSnapshot.outermostErrorIdx,
		);
	});

	it("finalizes preinstalled non-canonical runtime snapshots on first read", () => {
		mockGlobal.runtimeRouteSnapshot = {
			buildID: "2",
			matchedPatterns: ["/seeded"],
			loadersData: [{ value: "seeded" }],
			clientLoadersData: ["seeded-client"],
			importURLs: ["/seeded.js"],
			exportKeys: ["default"],
			errorExportKeys: [],
			hasRootData: true,
			params: { id: "seeded" },
			splatValues: [],
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			outermostClientError: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			rootElementID: "app",
			activeComponents: ["SeededComponent"],
			activeErrorBoundary: undefined,
		};

		const finalizedSnapshot = getRuntimeRouteSnapshot();

		expect(Object.isFrozen(finalizedSnapshot)).toBe(true);
		expect(Object.isFrozen(finalizedSnapshot.matchedPatterns)).toBe(true);
		expect(Object.isFrozen(finalizedSnapshot.loadersData)).toBe(true);
		expect(mockGlobal.runtimeRouteSnapshot).toBe(finalizedSnapshot);
		const { get } = __getVormaClientGlobal();
		expect(get("runtimeRouteSnapshot").matchedPatterns).toBe(
			finalizedSnapshot.matchedPatterns,
		);
		expect(get("runtimeRouteSnapshot").loadersData).toBe(
			finalizedSnapshot.loadersData,
		);
		expect(get("runtimeRouteSnapshot").clientLoadersData).toBe(
			finalizedSnapshot.clientLoadersData,
		);
	});
});

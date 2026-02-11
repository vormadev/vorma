import { afterEach, beforeEach, describe, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import { navigationStateManager } from "../../client";
import { HistoryManager } from "../../platform/history.ts";
import type { VormaAppConfig } from "../../app/helpers.ts";

export const vormaAppConfig: VormaAppConfig = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

// Mock only what's necessary for testing
const mockSessionStorage = (() => {
	let store: { [key: string]: string } = {};
	return {
		getItem: (key: string) => store[key] || null,
		setItem: (key: string, value: string) => {
			if (key) {
				store[key] = value.toString();
			}
		},
		removeItem: (key: string) => {
			if (key) {
				delete store[key];
			}
		},
		clear: () => {
			store = {};
		},
	};
})();

// Helper to setup initial Vorma context
export const setupGlobalVormaContext = (initialData = {}) => {
	(globalThis as any)[Symbol.for("__vorma_internal__")] = {
		buildID: "1",
		matchedPatterns: [],
		importURLs: [],
		exportKeys: [],
		loadersData: [],
		params: {},
		splatValues: [],
		hasRootData: false,
		activeComponents: [],
		clientLoadersData: [],
		patternToWaitFnMap: {},
		viteDevURL: "",
		publicPathPrefix: "",
		patternRegistry: createPatternRegistry(),
		...initialData,
	};
};

// Helper to create mock fetch responses
export const createMockResponse = (data: any, options: ResponseInit = {}) => {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Vorma-Build-Id": "1",
			...options.headers,
		},
		...options,
	});
};

export interface NavigationTestSuiteHelpers {
	addCleanup: (fn: () => void) => () => void;
	addListener: <T>(
		adder: (fn: (e: CustomEvent<T>) => void) => () => void,
		fn: (e: CustomEvent<T>) => void,
	) => () => void;
}

export const describeNavigationTestSuite = (
	registerTests: (helpers: NavigationTestSuiteHelpers) => void,
) => {
	describe("Comprehensive Navigation Test Suite", () => {
		let locationBackup: Location;
		let historyBackup: History;
		const cleanupFns: Array<() => void> = [];

		beforeEach(() => {
			vi.useFakeTimers({ shouldAdvanceTime: true });

			// Mock CSS.escape if it doesn't exist (not available in jsdom)
			if (!global.CSS) {
				(global as any).CSS = {};
			}
			if (!global.CSS.escape) {
				global.CSS.escape = (str: string) =>
					str.replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^`{|}~]/g, "\\$&");
			}

			vi.doMock("/module1.js", () => ({ default: () => {} }));
			vi.doMock("/module2.js", () => ({ default: () => {} }));

			// Backup original objects
			locationBackup = window.location;
			historyBackup = window.history;

			// Set up a complete window.location mock
			Object.defineProperty(window, "location", {
				value: {
					href: "http://localhost:3000/",
					origin: "http://localhost:3000",
					protocol: "http:",
					host: "localhost:3000",
					hostname: "localhost",
					port: "3000",
					pathname: "/",
					search: "",
					hash: "",
					assign: vi.fn((url) => {
						window.location.href = url;
					}),
					replace: vi.fn((url) => {
						const newUrl = new URL(url, window.location.href);
						window.location.href = newUrl.href;
						window.location.pathname = newUrl.pathname;
						window.location.search = newUrl.search;
						window.location.hash = newUrl.hash;
					}),
					reload: vi.fn(),
					toString: () => window.location.href,
				},
				writable: true,
				configurable: true,
			});

			// Mock Element.prototype.scrollIntoView
			if (!Element.prototype.scrollIntoView) {
				Element.prototype.scrollIntoView = vi.fn();
			}

			// Mock history.scrollRestoration
			let scrollRestorationValue = "auto";
			Object.defineProperty(window.history, "scrollRestoration", {
				get: () => scrollRestorationValue,
				set: (v) => {
					scrollRestorationValue = v;
				},
				configurable: true,
			});

			// Mock history methods
			window.history.replaceState = vi.fn((state, title, url) => {
				if (url) {
					const newUrl = new URL(url, window.location.href);
					window.location.href = newUrl.href;
					window.location.pathname = newUrl.pathname;
					window.location.search = newUrl.search;
					window.location.hash = newUrl.hash;
				}
			});

			window.history.pushState = vi.fn((state, title, url) => {
				if (url) {
					const newUrl = new URL(url, window.location.href);
					window.location.href = newUrl.href;
					window.location.pathname = newUrl.pathname;
					window.location.search = newUrl.search;
					window.location.hash = newUrl.hash;
				}
			});

			// Mock sessionStorage
			Object.defineProperty(window, "sessionStorage", {
				value: mockSessionStorage,
				writable: true,
				configurable: true,
			});

			// Mock window scroll properties
			Object.defineProperty(window, "scrollTo", {
				value: vi.fn(),
				writable: true,
			});
			Object.defineProperty(window, "scrollX", {
				value: 0,
				writable: true,
			});
			Object.defineProperty(window, "scrollY", {
				value: 0,
				writable: true,
			});

			// Mock startViewTransition
			const mockStartViewTransition = vi.fn((callback) => {
				callback?.();
				return { finished: Promise.resolve() };
			});

			Object.defineProperty(document, "startViewTransition", {
				value: mockStartViewTransition,
				configurable: true,
			});

			// Setup Vorma context
			setupGlobalVormaContext();

			// Setup spies
			vi.spyOn(window, "fetch");
			vi.spyOn(window, "dispatchEvent");
			vi.spyOn(console, "error").mockImplementation(() => {});
			vi.spyOn(console, "info").mockImplementation(() => {});

			// Clear all state
			mockSessionStorage.clear();
			vi.clearAllMocks();
			navigationStateManager.clearAll();
			document.title = "Initial Page";
			(window as any).scrollX = 0;
			(window as any).scrollY = 0;

			// Clear any existing listeners to prevent memory leaks
			cleanupFns.forEach((fn) => fn());
			cleanupFns.length = 0;

			// Initialize history after location is properly set up
			HistoryManager.init();
		});

		afterEach(async () => {
			// Run all pending timers to ensure status events fire
			await vi.runAllTimersAsync();

			// Clean up all listeners
			cleanupFns.forEach((fn) => fn());
			cleanupFns.length = 0;

			// Clear DOM
			document.body.innerHTML = "";
			document.head.innerHTML = "";

			// Clear any pending promises to avoid unhandled rejections
			vi.clearAllMocks();

			// Restore mocks
			vi.restoreAllMocks();

			// Restore original objects
			Object.defineProperty(window, "location", {
				value: locationBackup,
				writable: true,
				configurable: true,
			});
			Object.defineProperty(window, "history", {
				value: historyBackup,
				writable: true,
				configurable: true,
			});

			// Force garbage collection if available
			if (global.gc) {
				global.gc();
			}
		});

		// Add helper to register cleanup functions
		const addCleanup = (fn: () => void) => {
			cleanupFns.push(fn);
			return fn;
		};

		// Update all listener additions to register cleanup
		const addListener = <T>(
			adder: (fn: (e: CustomEvent<T>) => void) => () => void,
			fn: (e: CustomEvent<T>) => void,
		) => {
			const cleanup = adder(fn);
			addCleanup(cleanup);
			return cleanup;
		};

		registerTests({ addCleanup, addListener });
	});
};

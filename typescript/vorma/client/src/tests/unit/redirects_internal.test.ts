import { afterEach, describe, expect, it, vi } from "vitest";
import type { NavigationEntry, RedirectData } from "../../../src/runtime.ts";

type ContextModule = typeof import("../../runtime.ts");
type RedirectsModule = typeof import("../../runtime.ts");

async function loadRedirectModules(): Promise<{
	contextModule: ContextModule;
	redirectsModule: RedirectsModule;
}> {
	vi.resetModules();
	const contextModule = await import("../../runtime.ts");
	const redirectsModule = await import("../../runtime.ts");
	return { contextModule, redirectsModule };
}

function createRedirectEntryForCleanup(type: NavigationEntry["type"]): {
	key: string;
	entry: NavigationEntry;
} {
	const targetUrl = `http://localhost:3000/${type}`;
	return {
		key: targetUrl,
		entry: {
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: Promise.resolve({ type: "aborted" }),
			},
			type,
			intent: "navigate",
			phase: "fetching",
			startTime: 0,
			targetUrl,
			originUrl: "http://localhost:3000/",
		},
	};
}

function createDidRedirectData(): RedirectData {
	return {
		status: "did",
		href: "/already-redirected",
		hrefDetails: {
			isHTTP: true,
		},
	} as unknown as RedirectData;
}

function createShouldRedirectDataWithOverrides(overrides: {
	shouldRedirectStrategy?: string;
	isHTTP?: boolean;
}): RedirectData {
	return {
		status: "should",
		href: "/target",
		latestBuildID: "build-2",
		shouldRedirectStrategy: overrides.shouldRedirectStrategy ?? "hard",
		hrefDetails: {
			isHTTP: overrides.isHTTP ?? true,
			isExternal: false,
			isInternal: true,
			absoluteURL: "http://localhost:3000/target",
			relativeURL: "/target",
		},
	} as unknown as RedirectData;
}

describe("redirects internal defensive branches", () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it("is a no-op for already-applied redirect data", async () => {
		const { redirectsModule } = await loadRedirectModules();

		await expect(
			redirectsModule.effectuateRedirectDataResult(
				createDidRedirectData(),
				0,
			),
		).resolves.toBeNull();
	});

	it("returns null for already-applied redirect data without requiring navigation state", async () => {
		const { redirectsModule } = await loadRedirectModules();

		await expect(
			redirectsModule.effectuateRedirectDataResult(
				createDidRedirectData(),
				0,
			),
		).resolves.toBeNull();
	});

	it("returns null when hard redirect data is non-http", async () => {
		const { contextModule, redirectsModule } = await loadRedirectModules();

		const removeNavigation = vi.fn();
		contextModule.setNavigationStateAccess({
			navigate: vi.fn().mockResolvedValue({ didNavigate: false }),
			removeNavigation,
			getNavigations: vi.fn().mockReturnValue(new Map()),
		});

		const result = await redirectsModule.effectuateRedirectDataResult(
			createShouldRedirectDataWithOverrides({
				shouldRedirectStrategy: "hard",
				isHTTP: false,
			}),
			0,
		);

		expect(result).toBeNull();
		expect(removeNavigation).not.toHaveBeenCalled();
	});

	it("returns null for unknown redirect strategy after cleanup", async () => {
		const { contextModule, redirectsModule } = await loadRedirectModules();
		const redirectEntry = createRedirectEntryForCleanup("redirect");
		const userNavigationEntry =
			createRedirectEntryForCleanup("userNavigation");
		const navigationMap = new Map<string, NavigationEntry>([
			[redirectEntry.key, redirectEntry.entry],
			[userNavigationEntry.key, userNavigationEntry.entry],
		]);

		const removeNavigation = vi.fn((key: string) => {
			navigationMap.delete(key);
		});
		contextModule.setNavigationStateAccess({
			navigate: vi.fn().mockResolvedValue({ didNavigate: false }),
			removeNavigation,
			getNavigations: vi.fn(() => navigationMap),
		});

		const result = await redirectsModule.effectuateRedirectDataResult(
			createShouldRedirectDataWithOverrides({
				shouldRedirectStrategy: "unexpected",
			}),
			0,
		);

		expect(result).toBeNull();
		expect(
			redirectEntry.entry.control.abortController?.signal.aborted,
		).toBe(true);
		expect(
			userNavigationEntry.entry.control.abortController?.signal.aborted,
		).toBe(false);
		expect(removeNavigation).toHaveBeenCalledWith(redirectEntry.key);
		expect(removeNavigation).not.toHaveBeenCalledWith(
			userNavigationEntry.key,
		);
	});

	it("returns null for soft redirect when navigation does not complete", async () => {
		const { contextModule, redirectsModule } = await loadRedirectModules();
		const navigate = vi.fn().mockResolvedValue({ didNavigate: false });
		contextModule.setNavigationStateAccess({
			navigate,
			removeNavigation: vi.fn(),
			getNavigations: vi.fn().mockReturnValue(new Map()),
		});

		const result = await redirectsModule.effectuateRedirectDataResult(
			createShouldRedirectDataWithOverrides({
				shouldRedirectStrategy: "soft",
			}),
			2,
			{
				href: "/from-submit",
				navigationType: "redirect",
				replace: true,
				scrollToTop: false,
				state: { from: "submit" },
			},
		);

		expect(result).toBeNull();
		expect(navigate).toHaveBeenCalledWith({
			href: "/target",
			navigationType: "redirect",
			redirectCount: 3,
			replace: true,
			scrollToTop: false,
			state: { from: "submit" },
		});
	});

	it("cleans redirect and revalidation lanes before soft redirect navigation", async () => {
		const { contextModule, redirectsModule } = await loadRedirectModules();
		const redirectEntry = createRedirectEntryForCleanup("redirect");
		const revalidationEntry = createRedirectEntryForCleanup("revalidation");
		const userNavigationEntry =
			createRedirectEntryForCleanup("userNavigation");
		const navigationMap = new Map<string, NavigationEntry>([
			[redirectEntry.key, redirectEntry.entry],
			[revalidationEntry.key, revalidationEntry.entry],
			[userNavigationEntry.key, userNavigationEntry.entry],
		]);
		const removeNavigation = vi.fn((key: string) => {
			navigationMap.delete(key);
		});
		const navigate = vi.fn().mockResolvedValue({ didNavigate: true });
		contextModule.setNavigationStateAccess({
			navigate,
			removeNavigation,
			getNavigations: vi.fn(() => navigationMap),
		});

		const result = await redirectsModule.effectuateRedirectDataResult(
			createShouldRedirectDataWithOverrides({
				shouldRedirectStrategy: "soft",
			}),
			4,
		);

		expect(
			redirectEntry.entry.control.abortController?.signal.aborted,
		).toBe(true);
		expect(
			revalidationEntry.entry.control.abortController?.signal.aborted,
		).toBe(true);
		expect(
			userNavigationEntry.entry.control.abortController?.signal.aborted,
		).toBe(false);
		expect(removeNavigation).toHaveBeenCalledWith(redirectEntry.key);
		expect(removeNavigation).toHaveBeenCalledWith(revalidationEntry.key);
		expect(removeNavigation).not.toHaveBeenCalledWith(
			userNavigationEntry.key,
		);
		const firstCleanupInvocationOrder =
			removeNavigation.mock.invocationCallOrder[0];
		const firstNavigateInvocationOrder =
			navigate.mock.invocationCallOrder[0];
		expect(firstCleanupInvocationOrder).toBeDefined();
		expect(firstNavigateInvocationOrder).toBeDefined();
		expect(firstCleanupInvocationOrder!).toBeLessThan(
			firstNavigateInvocationOrder!,
		);
		expect(navigate).toHaveBeenCalledWith({
			href: "/target",
			navigationType: "redirect",
			redirectCount: 5,
			replace: undefined,
			scrollToTop: undefined,
			state: undefined,
		});
		expect(result).toMatchObject({
			status: "did",
			href: "/target",
		});
	});

	it("returns did redirect data for soft redirect when navigation completes", async () => {
		const { contextModule, redirectsModule } = await loadRedirectModules();
		const navigate = vi.fn().mockResolvedValue({ didNavigate: true });
		contextModule.setNavigationStateAccess({
			navigate,
			removeNavigation: vi.fn(),
			getNavigations: vi.fn().mockReturnValue(new Map()),
		});

		const result = await redirectsModule.effectuateRedirectDataResult(
			createShouldRedirectDataWithOverrides({
				shouldRedirectStrategy: "soft",
			}),
			0,
		);

		expect(result).toMatchObject({
			status: "did",
			href: "/target",
		});
		expect(navigate).toHaveBeenCalledWith({
			href: "/target",
			navigationType: "redirect",
			redirectCount: 1,
			replace: undefined,
			scrollToTop: undefined,
			state: undefined,
		});
	});

	it("throws explicit error for non-http native redirect response URLs", async () => {
		const { redirectsModule } = await loadRedirectModules();
		const response = createResponseMarkedAsRedirected({
			url: "mailto:test@example.com",
		});
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		await expect(
			redirectsModule.handleRedirects({
				abortController: new AbortController(),
				url: new URL("http://localhost:3000/start"),
			}),
		).rejects.toThrow("must be an HTTP(S) URL");
	});

	it("throws explicit error for invalid redirect header targets", async () => {
		const { redirectsModule } = await loadRedirectModules();
		const response = new Response(
			JSON.stringify({
				ok: true,
			}),
			{
				status: 200,
				headers: {
					"X-Client-Redirect": "http://%zz",
				},
			},
		);
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		await expect(
			redirectsModule.handleRedirects({
				abortController: new AbortController(),
				url: new URL("http://localhost:3000/start"),
			}),
		).rejects.toThrow("X-Client-Redirect has invalid redirect target");
	});

	it("throws explicit error for invalid native redirect response URLs", async () => {
		const { redirectsModule } = await loadRedirectModules();
		const response = createResponseMarkedAsRedirected({
			url: "http://%zz",
		});
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		await expect(
			redirectsModule.handleRedirects({
				abortController: new AbortController(),
				url: new URL("http://localhost:3000/start"),
			}),
		).rejects.toThrow(
			"redirected fetch response URL has invalid redirect target",
		);
	});

	it("resolves relative X-Client-Redirect targets against request URL, not current page URL", async () => {
		window.history.replaceState({}, "", "/current-parent/");
		const { redirectsModule } = await loadRedirectModules();
		const response = new Response(
			JSON.stringify({
				ok: true,
			}),
			{
				status: 200,
				headers: {
					"X-Client-Redirect": "child-redirect",
				},
			},
		);
		Object.defineProperty(response, "url", {
			value: "http://localhost:3000/server/base/start?vorma_json=1",
			configurable: true,
		});
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		const result = await redirectsModule.handleRedirects({
			abortController: new AbortController(),
			url: new URL(
				"http://localhost:3000/server/base/start?vorma_json=1",
			),
		});

		expect(result.redirectData).toMatchObject({
			status: "should",
			shouldRedirectStrategy: "soft",
			href: "http://localhost:3000/server/base/child-redirect",
		});
		expect(result.response).toBe(response);
	});

	it("resolves relative X-Vorma-Reload targets against request URL, not current page URL", async () => {
		window.history.replaceState({}, "", "/current-parent/");
		const { redirectsModule } = await loadRedirectModules();
		const response = new Response(
			JSON.stringify({
				ok: true,
			}),
			{
				status: 200,
				headers: {
					"X-Vorma-Reload": "child-reload",
				},
			},
		);
		Object.defineProperty(response, "url", {
			value: "http://localhost:3000/server/base/start?vorma_json=1",
			configurable: true,
		});
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		const result = await redirectsModule.handleRedirects({
			abortController: new AbortController(),
			url: new URL(
				"http://localhost:3000/server/base/start?vorma_json=1",
			),
		});

		expect(result.redirectData).toMatchObject({
			status: "should",
			shouldRedirectStrategy: "hard",
			href: "child-reload",
			hrefDetails: {
				absoluteURL: "http://localhost:3000/server/base/child-reload",
			},
		});
		expect(result.response).toBe(response);
	});

	it("short-circuits same-target X-Client-Redirect headers to did redirect data", async () => {
		window.history.replaceState({}, "", "/header-same-target#~");
		const { redirectsModule } = await loadRedirectModules();
		const response = new Response(
			JSON.stringify({
				ok: true,
			}),
			{
				status: 200,
				headers: {
					"X-Client-Redirect": "/header-same-target#%7E",
				},
			},
		);
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		const result = await redirectsModule.handleRedirects({
			abortController: new AbortController(),
			url: new URL("http://localhost:3000/start"),
		});

		expect(result.redirectData).toMatchObject({
			status: "did",
		});
		expect(result.response).toBe(response);
	});

	it("does not short-circuit same-target X-Vorma-Reload headers", async () => {
		window.history.replaceState({}, "", "/header-same-target-reload#~");
		const { redirectsModule } = await loadRedirectModules();
		const response = new Response(
			JSON.stringify({
				ok: true,
			}),
			{
				status: 200,
				headers: {
					"X-Vorma-Reload": "/header-same-target-reload#%7E",
				},
			},
		);
		vi.spyOn(window, "fetch").mockResolvedValue(response);

		const result = await redirectsModule.handleRedirects({
			abortController: new AbortController(),
			url: new URL("http://localhost:3000/start"),
		});

		expect(result.redirectData).toMatchObject({
			status: "should",
			shouldRedirectStrategy: "hard",
			href: "/header-same-target-reload#%7E",
		});
		expect(result.response).toBe(response);
	});
});

function createResponseMarkedAsRedirected(props: { url: string }): Response {
	const response = new Response(null, { status: 200 });
	Object.defineProperty(response, "redirected", {
		value: true,
		configurable: true,
	});
	Object.defineProperty(response, "url", {
		value: props.url,
		configurable: true,
	});
	return response;
}

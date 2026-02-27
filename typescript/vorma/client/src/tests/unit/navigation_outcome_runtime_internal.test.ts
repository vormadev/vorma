import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "../../../src/runtime.ts";
import {
	__vormaClientGlobal,
	handleNavigationOutcomeWithInternalResult,
	setNavigationStateAccess,
	VORMA_SYMBOL,
} from "../../runtime.ts";

function createEntry(props: {
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
	targetUrl?: string;
	operationID?: number;
}): NavigationEntry {
	return {
		operationID: props.operationID ?? 1,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl ?? "http://localhost:3000/target",
		originUrl: "http://localhost:3000/origin",
	};
}

function createRedirectOutcome(): Extract<
	NavigationOutcome,
	{ type: "redirect" }
> {
	return {
		type: "redirect",
		redirectData: {
			status: "should",
			shouldRedirectStrategy: "soft",
			latestBuildID: "2",
			href: "/redirect-target",
			hrefDetails: {
				url: new URL("http://localhost:3000/redirect-target"),
				isHTTP: true,
				isInternal: true,
				isExternal: false,
				absoluteURL: "http://localhost:3000/redirect-target",
				relativeURL: "/redirect-target",
			},
		},
		props: {
			href: "http://localhost:3000/target",
			navigationType: "userNavigation",
		},
	};
}

function createSuccessOutcome(
	props: {
		navigationType?: NavigateProps["navigationType"];
	} = {},
): Extract<NavigationOutcome, { type: "success" }> {
	return {
		type: "success",
		response: new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": "1",
			},
		}),
		json: {
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			deps: [],
			cssBundles: [],
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			title: undefined,
			metaHeadEls: undefined,
			restHeadEls: undefined,
		},
		preloadPlan: {
			moduleDependencies: [],
			cssBundles: [],
		},
		waitFnPromise: Promise.resolve({ data: [] }),
		props: {
			href: "http://localhost:3000/target",
			navigationType: props.navigationType ?? "userNavigation",
		},
	};
}

function installVormaGlobalForNavigationOutcomeTests(): void {
	(globalThis as any)[VORMA_SYMBOL] = {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: {
			actionsRouterMountRoot: "/api/",
			actionsDynamicRune: ":",
			actionsSplatRune: "*",
			loadersDynamicRune: ":",
			loadersSplatRune: "*",
			loadersExplicitIndexSegmentIdentifier: "_index",
		},
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: undefined,
		runtimeRouteSnapshot: {
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			outermostClientError: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			buildID: "1",
			rootElementID: undefined,
			activeComponents: [],
			activeErrorBoundary: undefined,
			clientLoadersData: [],
		},
	};
}

beforeEach(() => {
	installVormaGlobalForNavigationOutcomeTests();
});

afterEach(() => {
	vi.restoreAllMocks();
	delete (globalThis as any)[VORMA_SYMBOL];
});

describe("navigation outcome runtime", () => {
	it("returns cancelled when no navigation entry exists for the target", async () => {
		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi.fn(async () => {});
		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => undefined,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
			},
			outcome: { type: "aborted" },
			expectedOperationID: 1,
		});

		expect(result).toEqual({
			type: "cancelled",
			reason: "entry_not_found",
		});
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("deletes and cancels for aborted outcomes owned by the current operation", async () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi.fn(async () => {});
		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => entry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
			},
			outcome: { type: "aborted" },
			expectedOperationID: entry.operationID,
		});

		expect(result).toEqual({
			type: "cancelled",
			reason: "outcome_aborted",
		});
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: entry.targetUrl,
			reason: "outcome_aborted",
		});
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("syncs build ID, deletes navigation, and effectuates redirects", async () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const redirectOutcome = createRedirectOutcome();
		const navigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: navigateSpy,
			removeNavigation: vi.fn(),
			getNavigations: () => new Map(),
		});
		const deleteNavigation = vi.fn(() => {
			return true;
		});
		const processSuccessfulNavigation = vi.fn(async () => {});
		const navigationProps: NavigateProps = {
			href: "/target",
			navigationType: "userNavigation",
		};

		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => entry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps,
			outcome: redirectOutcome,
			expectedOperationID: entry.operationID,
		});

		expect(result).toEqual({
			type: "committed",
			didNavigate: true,
		});
		expect(__vormaClientGlobal.get("runtimeRouteSnapshot").buildID).toBe(
			"2",
		);
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: entry.targetUrl,
			reason: "redirect_effectuate",
		});
		expect(navigateSpy).toHaveBeenCalledOnce();
		expect(navigateSpy).toHaveBeenCalledWith({
			href: "/redirect-target",
			navigationType: "redirect",
			redirectCount: 1,
			state: entry.state,
			replace: entry.replace,
			scrollToTop: entry.scrollToTop,
		});
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("does not run redirect side effects when stale ownership cancels the outcome", async () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const redirectOutcome = createRedirectOutcome();
		const deleteNavigation = vi.fn(() => true);
		const navigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: navigateSpy,
			removeNavigation: vi.fn(),
			getNavigations: () => new Map(),
		});

		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => entry,
			deleteNavigation,
			processSuccessfulNavigation: vi.fn(async () => {}),
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
			},
			outcome: redirectOutcome,
			expectedOperationID: entry.operationID + 1,
		});

		expect(result).toEqual({
			type: "cancelled",
			reason: "stale_control_ownership",
		});
		expect(__vormaClientGlobal.get("runtimeRouteSnapshot").buildID).toBe(
			"1",
		);
		expect(navigateSpy).not.toHaveBeenCalled();
		expect(deleteNavigation).not.toHaveBeenCalled();
	});

	it("processes successful outcomes and returns committed didNavigate true", async () => {
		const entry = createEntry({
			type: "userNavigation",
			intent: "navigate",
		});
		const successOutcome = createSuccessOutcome();
		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi.fn(async () => {});
		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => entry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: "/target",
				navigationType: "userNavigation",
			},
			outcome: successOutcome,
			expectedOperationID: entry.operationID,
		});

		expect(result).toEqual({
			type: "committed",
			didNavigate: true,
		});
		expect(processSuccessfulNavigation).toHaveBeenCalledOnce();
		expect(processSuccessfulNavigation).toHaveBeenCalledWith(
			successOutcome,
			entry,
		);
		expect(deleteNavigation).not.toHaveBeenCalled();
	});

	it("returns committed didNavigate false for idle prefetch success entries", async () => {
		const entry = createEntry({
			type: "prefetch",
			intent: "none",
		});
		const successOutcome = createSuccessOutcome({
			navigationType: "prefetch",
		});
		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi.fn(async () => {});
		const result = await handleNavigationOutcomeWithInternalResult({
			findNavigationEntry: () => entry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: "/target",
				navigationType: "prefetch",
			},
			outcome: successOutcome,
			expectedOperationID: entry.operationID,
		});

		expect(result).toEqual({
			type: "committed",
			didNavigate: false,
		});
		expect(processSuccessfulNavigation).toHaveBeenCalledOnce();
		expect(processSuccessfulNavigation).toHaveBeenCalledWith(
			successOutcome,
			entry,
		);
		expect(deleteNavigation).not.toHaveBeenCalled();
	});
});

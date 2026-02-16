import { afterEach, describe, expect, it, vi } from "vitest";
import { handleNavigationOutcomeWithInternalResult } from "../../core/navigation/runtime_navigation_outcome.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "../../core/navigation/types.ts";
import * as redirectsModule from "../../core/redirects.ts";

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
		preloadCommands: [],
		waitFnPromise: Promise.resolve({ data: [] }),
		props: {
			href: "http://localhost:3000/target",
			navigationType: props.navigationType ?? "userNavigation",
		},
	};
}

afterEach(() => {
	vi.restoreAllMocks();
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
		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi.fn(async () => {});
		const syncBuildIDSpy = vi
			.spyOn(redirectsModule, "syncBuildIDFromRedirectData")
			.mockImplementation(() => {});
		const effectuateRedirectSpy = vi
			.spyOn(redirectsModule, "effectuateRedirectDataResult")
			.mockResolvedValue({
				status: "did",
				href: "/redirect-target",
				hrefDetails: redirectOutcome.redirectData.hrefDetails,
			} as any);
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
		expect(syncBuildIDSpy).toHaveBeenCalledOnce();
		expect(syncBuildIDSpy).toHaveBeenCalledWith(
			redirectOutcome.redirectData,
		);
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: entry.targetUrl,
			reason: "redirect_effectuate",
		});
		expect(effectuateRedirectSpy).toHaveBeenCalledOnce();
		expect(effectuateRedirectSpy).toHaveBeenCalledWith(
			redirectOutcome.redirectData,
			0,
			navigationProps,
		);
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
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

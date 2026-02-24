import { describe, expect, it, vi } from "vitest";
import { executeNavigationSinglePass } from "../../core/navigation/runtime_navigation_pass_runtime.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationOutcome,
} from "../../core/navigation/types.ts";

function createNavigationEntry(props: {
	operationID: number;
	targetUrl?: string;
}): NavigationEntry {
	return {
		operationID: props.operationID,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: "userNavigation",
		intent: "navigate",
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl ?? "http://localhost:3000/target",
		originUrl: "http://localhost:3000/",
	};
}

function createSuccessOutcome(): Extract<
	NavigationOutcome,
	{ type: "success" }
> {
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
			href: "/target",
			navigationType: "userNavigation",
		},
	};
}

describe("navigation pass runtime", () => {
	it("runs a successful pass and applies outcome handling", async () => {
		const navigationProps: NavigateProps = {
			href: "/target",
			navigationType: "userNavigation",
		};
		const entry = createNavigationEntry({
			operationID: 7,
		});
		const control: NavigationControl = {
			abortController: new AbortController(),
			promise: Promise.resolve(createSuccessOutcome()),
			operationID: 7,
		};
		const processSuccessfulNavigation = vi.fn(async () => {});
		const onNavigationPromiseRejected = vi.fn();

		const result = await executeNavigationSinglePass({
			navigationProps,
			beginNavigation: () => control,
			findNavigationEntry: () => entry,
			deleteNavigation: vi.fn(() => true),
			processSuccessfulNavigation,
			onNavigationPromiseRejected,
		});

		expect(result).toEqual({ didNavigate: true });
		expect(processSuccessfulNavigation).toHaveBeenCalledOnce();
		expect(processSuccessfulNavigation).toHaveBeenCalledWith(
			expect.objectContaining({
				type: "success",
			}),
			entry,
		);
		expect(onNavigationPromiseRejected).not.toHaveBeenCalled();
	});

	it("cleans up owned entries and reports rejected pass failures", async () => {
		const navigationProps: NavigateProps = {
			href: "/target",
			navigationType: "userNavigation",
		};
		const ownedEntry = createNavigationEntry({
			operationID: 9,
		});
		const control: NavigationControl = {
			abortController: new AbortController(),
			promise: Promise.reject(new Error("network_error")),
			operationID: 9,
		};
		const deleteNavigation = vi.fn(() => true);
		const onNavigationPromiseRejected = vi.fn();

		const result = await executeNavigationSinglePass({
			navigationProps,
			beginNavigation: () => control,
			findNavigationEntry: () => ownedEntry,
			deleteNavigation,
			processSuccessfulNavigation: vi.fn(async () => {}),
			onNavigationPromiseRejected,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: "http://localhost:3000/target",
			reason: "navigate_promise_rejected",
		});
		expect(onNavigationPromiseRejected).toHaveBeenCalledOnce();
		expect(onNavigationPromiseRejected).toHaveBeenCalledWith({
			targetUrl: "http://localhost:3000/target",
			ownedEntry,
		});
	});

	it("does not delete entries that are no longer owned on rejection", async () => {
		const navigationProps: NavigateProps = {
			href: "/target",
			navigationType: "userNavigation",
		};
		const staleEntry = createNavigationEntry({
			operationID: 5,
		});
		const control: NavigationControl = {
			abortController: new AbortController(),
			promise: Promise.reject(new Error("network_error")),
			operationID: 12,
		};
		const deleteNavigation = vi.fn(() => true);
		const onNavigationPromiseRejected = vi.fn();

		const result = await executeNavigationSinglePass({
			navigationProps,
			beginNavigation: () => control,
			findNavigationEntry: () => staleEntry,
			deleteNavigation,
			processSuccessfulNavigation: vi.fn(async () => {}),
			onNavigationPromiseRejected,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(onNavigationPromiseRejected).toHaveBeenCalledOnce();
		expect(onNavigationPromiseRejected).toHaveBeenCalledWith({
			targetUrl: "http://localhost:3000/target",
			ownedEntry: undefined,
		});
	});
});

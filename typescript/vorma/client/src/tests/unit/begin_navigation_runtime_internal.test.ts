import { describe, expect, it, vi } from "vitest";
import type {
	BeginNavigationContext,
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "../../../src/runtime.ts";
import {
	beginNavigation,
	buildBeginNavigationRuntimeCommandPlan,
	decideBeginNavigationExecutionPlan,
} from "../../runtime.ts";

let nextOperationID = 1;

function createEntry(props: {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
}): NavigationEntry {
	return {
		operationID: nextOperationID++,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({
				type: "aborted",
			} as const satisfies NavigationOutcome),
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl,
		originUrl: "http://localhost:3000/",
		scrollToTop: true,
		replace: false,
		state: undefined,
	};
}

function createNavigationProps(props: Partial<NavigateProps>): NavigateProps {
	return {
		href: props.href ?? "http://localhost:3000/",
		navigationType: props.navigationType ?? "userNavigation",
		state: props.state,
		scrollStateToRestore: props.scrollStateToRestore,
		replace: props.replace,
		redirectCount: props.redirectCount,
		scrollToTop: props.scrollToTop,
	};
}

describe("begin navigation runtime command planning", () => {
	it("builds ordered commands for reuse with prefetch promotion and stale-lane aborts", () => {
		const staleActive = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});
		const matchedPrefetch = createEntry({
			targetUrl: "http://localhost:3000/reused#prefetch",
			type: "prefetch",
			intent: "none",
		});
		const stalePrefetch = createEntry({
			targetUrl: "http://localhost:3000/stale-prefetch",
			type: "prefetch",
			intent: "none",
		});
		const staleRevalidation = createEntry({
			targetUrl: "http://localhost:3000/stale-revalidation",
			type: "revalidation",
			intent: "revalidate",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/reused#next",
				navigationType: "browserHistory",
			}),
			currentHref: "http://localhost:3000/current",
			lanes: {
				active: staleActive,
				revalidation: staleRevalidation,
				prefetch: new Map([
					[matchedPrefetch.targetUrl, matchedPrefetch],
					[stalePrefetch.targetUrl, stalePrefetch],
				]),
			},
		});
		const commandPlan = buildBeginNavigationRuntimeCommandPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/reused#next",
				navigationType: "browserHistory",
			}),
			executionPlan,
		});

		expect(commandPlan.commands.map((command) => command.type)).toEqual([
			"abort_navigation_entry",
			"abort_navigation_entry",
			"abort_navigation_entry",
			"apply_reuse_instruction",
			"schedule_status_update_if_changed",
		]);
		expect(commandPlan.terminalResult).toEqual({
			type: "return_reused_control",
		});
	});

	it("does not emit explicit schedule command when creating active-lane navigation", () => {
		const staleActive = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/fresh",
				navigationType: "browserHistory",
			}),
			currentHref: "http://localhost:3000/current",
			lanes: {
				active: staleActive,
				revalidation: null,
				prefetch: new Map(),
			},
		});
		const commandPlan = buildBeginNavigationRuntimeCommandPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/fresh",
				navigationType: "browserHistory",
			}),
			executionPlan,
		});

		expect(commandPlan.commands.map((command) => command.type)).toEqual([
			"abort_navigation_entry",
		]);
		expect(commandPlan.terminalResult).toEqual({
			type: "create_navigation_control",
			createInstruction: {
				slot: "active",
			},
			navigationProps: {
				href: "http://localhost:3000/fresh",
				navigationType: "browserHistory",
				redirectCount: undefined,
				replace: undefined,
				scrollStateToRestore: undefined,
				scrollToTop: undefined,
				state: undefined,
			},
		});
	});

	it("does not emit status-update command for prefetch create", () => {
		const staleActive = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/new-prefetch",
				navigationType: "prefetch",
			}),
			currentHref: "http://localhost:3000/current",
			lanes: {
				active: staleActive,
				revalidation: null,
				prefetch: new Map(),
			},
		});
		const commandPlan = buildBeginNavigationRuntimeCommandPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/new-prefetch",
				navigationType: "prefetch",
			}),
			executionPlan,
		});

		expect(commandPlan.commands.map((command) => command.type)).toEqual([]);
	});

	it("returns immediate-abort terminal result for prefetch of current location", () => {
		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/current#next",
				navigationType: "prefetch",
			}),
			currentHref: "http://localhost:3000/current#live",
			lanes: {
				active: null,
				revalidation: null,
				prefetch: new Map(),
			},
		});
		const commandPlan = buildBeginNavigationRuntimeCommandPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/current#next",
				navigationType: "prefetch",
			}),
			executionPlan,
		});

		expect(commandPlan.commands).toEqual([]);
		expect(commandPlan.terminalResult).toEqual({
			type: "return_immediately_aborted_control",
		});
	});
});

describe("begin navigation runtime command execution", () => {
	it("executes reuse promotion/abort commands before returning reused control", () => {
		const staleActive = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});
		const matchedPrefetch = createEntry({
			targetUrl: "http://localhost:3000/reuse#a",
			type: "prefetch",
			intent: "none",
		});
		const stalePrefetch = createEntry({
			targetUrl: "http://localhost:3000/stale-prefetch",
			type: "prefetch",
			intent: "none",
		});
		const staleRevalidation = createEntry({
			targetUrl: "http://localhost:3000/stale-revalidation",
			type: "revalidation",
			intent: "revalidate",
		});

		let activeNavigation: NavigationEntry | null = staleActive;
		let revalidationNavigation: NavigationEntry | null = staleRevalidation;
		const prefetchNavigationsByTargetUrl = new Map<string, NavigationEntry>(
			[
				[matchedPrefetch.targetUrl, matchedPrefetch],
				[stalePrefetch.targetUrl, stalePrefetch],
			],
		);
		const scheduleStatusUpdate = vi.fn();
		const fetchRouteData = vi.fn(() =>
			Promise.resolve({ type: "aborted" as const }),
		);
		const deleteNavigation = vi.fn(
			(props: { targetUrl: string; reason: string }) => {
				let deleted = false;
				if (activeNavigation?.targetUrl === props.targetUrl) {
					activeNavigation = null;
					deleted = true;
				}
				if (revalidationNavigation?.targetUrl === props.targetUrl) {
					revalidationNavigation = null;
					deleted = true;
				}
				if (prefetchNavigationsByTargetUrl.delete(props.targetUrl)) {
					deleted = true;
				}
				return deleted;
			},
		);

		const context: BeginNavigationContext = {
			getActiveNavigation: () => activeNavigation,
			setActiveNavigation: (entry) => {
				activeNavigation = entry;
			},
			getRevalidationNavigation: () => revalidationNavigation,
			setRevalidationNavigation: (entry) => {
				revalidationNavigation = entry;
			},
			prefetchNavigationsByTargetUrl,
			scheduleStatusUpdate,
			fetchRouteData,
			deleteNavigation,
			allocateNavigationOperationID: vi.fn(() => nextOperationID++),
		};

		const control = beginNavigation(
			context,
			createNavigationProps({
				href: "http://localhost:3000/reuse#b",
				navigationType: "browserHistory",
			}),
		);

		expect(control).toBe(matchedPrefetch.control);
		expect(activeNavigation).toBe(matchedPrefetch);
		expect(revalidationNavigation).toBeNull();
		expect(
			prefetchNavigationsByTargetUrl.has(matchedPrefetch.targetUrl),
		).toBe(false);
		expect(
			prefetchNavigationsByTargetUrl.has(stalePrefetch.targetUrl),
		).toBe(false);
		expect(staleActive.control.abortController?.signal.aborted).toBe(true);
		expect(stalePrefetch.control.abortController?.signal.aborted).toBe(
			true,
		);
		expect(staleRevalidation.control.abortController?.signal.aborted).toBe(
			true,
		);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
		expect(fetchRouteData).not.toHaveBeenCalled();
	});

	it("creates active navigation control and schedules status update", () => {
		let activeNavigation: NavigationEntry | null = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});
		let revalidationNavigation: NavigationEntry | null = null;
		const scheduleStatusUpdate = vi.fn();
		const fetchRouteData = vi.fn(() =>
			Promise.resolve({ type: "aborted" as const }),
		);
		const deleteNavigation = vi.fn(() => true);

		const context: BeginNavigationContext = {
			getActiveNavigation: () => activeNavigation,
			setActiveNavigation: (entry) => {
				activeNavigation = entry;
			},
			getRevalidationNavigation: () => revalidationNavigation,
			setRevalidationNavigation: (entry) => {
				revalidationNavigation = entry;
			},
			prefetchNavigationsByTargetUrl: new Map(),
			scheduleStatusUpdate,
			fetchRouteData,
			deleteNavigation,
			allocateNavigationOperationID: vi.fn(() => nextOperationID++),
		};

		const control = beginNavigation(
			context,
			createNavigationProps({
				href: "http://localhost:3000/fresh-active",
				navigationType: "browserHistory",
			}),
		);

		expect(activeNavigation).toBeTruthy();
		expect(control).toBe(activeNavigation?.control);
		expect(activeNavigation?.targetUrl).toBe(
			"http://localhost:3000/fresh-active",
		);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
		expect(fetchRouteData).toHaveBeenCalledTimes(1);
		expect(fetchRouteData).toHaveBeenCalledWith(
			expect.any(AbortController),
			{
				href: "http://localhost:3000/fresh-active",
				navigationType: "browserHistory",
				redirectCount: undefined,
				replace: undefined,
				scrollStateToRestore: undefined,
				scrollToTop: undefined,
				state: undefined,
			},
		);
		expect(deleteNavigation).toHaveBeenCalledTimes(1);
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: "http://localhost:3000/stale-active",
			reason: "begin_navigation_abort_instruction_active",
		});
	});

	it("returns immediate-abort control without starting fetch work", async () => {
		window.history.replaceState(
			{},
			"",
			"http://localhost:3000/current#live",
		);

		const context: BeginNavigationContext = {
			getActiveNavigation: () => null,
			setActiveNavigation: () => {},
			getRevalidationNavigation: () => null,
			setRevalidationNavigation: () => {},
			prefetchNavigationsByTargetUrl: new Map(),
			scheduleStatusUpdate: vi.fn(),
			fetchRouteData: vi.fn(() =>
				Promise.resolve({ type: "aborted" as const }),
			),
			deleteNavigation: vi.fn(() => true),
			allocateNavigationOperationID: vi.fn(() => nextOperationID++),
		};

		const control = beginNavigation(
			context,
			createNavigationProps({
				href: "http://localhost:3000/current#next",
				navigationType: "prefetch",
			}),
		);

		const outcome = await control.promise;
		expect(control.abortController?.signal.aborted).toBe(true);
		expect(outcome).toEqual({ type: "aborted" });
		expect(context.fetchRouteData).not.toHaveBeenCalled();
	});

	it("creates revalidation with current href from revalidation instruction", () => {
		window.history.replaceState(
			{},
			"",
			"http://localhost:3000/account#live",
		);
		let revalidationNavigation: NavigationEntry | null = null;
		const fetchRouteData = vi.fn(() =>
			Promise.resolve({ type: "aborted" as const }),
		);
		const scheduleStatusUpdate = vi.fn();

		const context: BeginNavigationContext = {
			getActiveNavigation: () => null,
			setActiveNavigation: () => {},
			getRevalidationNavigation: () => revalidationNavigation,
			setRevalidationNavigation: (entry) => {
				revalidationNavigation = entry;
			},
			prefetchNavigationsByTargetUrl: new Map(),
			scheduleStatusUpdate,
			fetchRouteData,
			deleteNavigation: vi.fn(() => true),
			allocateNavigationOperationID: vi.fn(() => nextOperationID++),
		};

		const control = beginNavigation(
			context,
			createNavigationProps({
				href: "http://localhost:3000/ignored",
				navigationType: "revalidation",
			}),
		);

		const createdRevalidationNavigation =
			context.getRevalidationNavigation();
		expect(createdRevalidationNavigation).toBeTruthy();
		if (!createdRevalidationNavigation) {
			throw new Error(
				"Test invariant violated: expected created revalidation navigation.",
			);
		}
		expect(control).toBe(createdRevalidationNavigation.control);
		expect(createdRevalidationNavigation.targetUrl).toBe(
			"http://localhost:3000/account#live",
		);
		expect(fetchRouteData).toHaveBeenCalledWith(
			expect.any(AbortController),
			{
				href: "http://localhost:3000/account#live",
				navigationType: "revalidation",
				redirectCount: undefined,
				replace: undefined,
				scrollStateToRestore: undefined,
				scrollToTop: undefined,
				state: undefined,
			},
		);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("deletes owned active entry when created active fetch rejects", async () => {
		let activeNavigation: NavigationEntry | null = null;
		const deleteNavigation = vi.fn(() => true);
		const context: BeginNavigationContext = {
			getActiveNavigation: () => activeNavigation,
			setActiveNavigation: (entry) => {
				activeNavigation = entry;
			},
			getRevalidationNavigation: () => null,
			setRevalidationNavigation: () => {},
			prefetchNavigationsByTargetUrl: new Map(),
			scheduleStatusUpdate: vi.fn(),
			fetchRouteData: vi.fn(async () => {
				throw new Error("fetch failed");
			}),
			deleteNavigation,
			allocateNavigationOperationID: vi.fn(() => nextOperationID++),
		};

		const control = beginNavigation(
			context,
			createNavigationProps({
				href: "http://localhost:3000/rejected",
				navigationType: "browserHistory",
			}),
		);

		await expect(control.promise).rejects.toThrow("fetch failed");
		expect(deleteNavigation).toHaveBeenCalledTimes(1);
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl: "http://localhost:3000/rejected",
			reason: "active_navigation_fetch_rejected",
		});
	});
});

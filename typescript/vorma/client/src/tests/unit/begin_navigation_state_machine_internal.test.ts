import { describe, expect, it } from "vitest";
import { decideBeginNavigationExecutionPlan } from "../../core/navigation/begin_navigation_state_machine.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "../../core/navigation/types.ts";

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

describe("begin navigation state machine", () => {
	it("reuses matching active navigation and aborts stale lanes", () => {
		const activeEntry = createEntry({
			targetUrl: "http://localhost:3000/active#cached",
			type: "userNavigation",
			intent: "navigate",
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
				href: "http://localhost:3000/active#next",
				navigationType: "userNavigation",
				scrollToTop: false,
				replace: true,
				state: { source: "test" },
			}),
			currentHref: "http://localhost:3000/current",
			lanes: {
				active: activeEntry,
				revalidation: staleRevalidation,
				prefetch: new Map([[stalePrefetch.targetUrl, stalePrefetch]]),
			},
		});

		expect(executionPlan).toEqual({
			type: "reuse",
			abortInstructions: [
				{
					slot: "prefetch",
					key: stalePrefetch.targetUrl,
					entry: stalePrefetch,
				},
				{
					slot: "revalidation",
					entry: staleRevalidation,
				},
			],
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: activeEntry,
				promotion: {
					targetUrl: "http://localhost:3000/active#next",
					type: "userNavigation",
					intent: "navigate",
					scrollToTop: false,
					replace: true,
					state: { source: "test" },
				},
			},
		});
	});

	it("promotes matching prefetch entry into active lane", () => {
		const staleActive = createEntry({
			targetUrl: "http://localhost:3000/stale-active",
			type: "userNavigation",
			intent: "navigate",
		});
		const matchedPrefetch = createEntry({
			targetUrl: "http://localhost:3000/destination#prefetched",
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
				href: "http://localhost:3000/destination#next",
				navigationType: "browserHistory",
				scrollToTop: true,
				replace: false,
				state: { from: "history" },
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

		expect(executionPlan).toEqual({
			type: "reuse",
			abortInstructions: [
				{
					slot: "active",
					entry: staleActive,
				},
				{
					slot: "prefetch",
					key: stalePrefetch.targetUrl,
					entry: stalePrefetch,
				},
				{
					slot: "revalidation",
					entry: staleRevalidation,
				},
			],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: matchedPrefetch.targetUrl,
				entry: matchedPrefetch,
				promotion: {
					targetUrl: "http://localhost:3000/destination#next",
					type: "browserHistory",
					intent: "navigate",
					scrollToTop: true,
					replace: false,
					state: { from: "history" },
				},
			},
		});
	});

	it("returns immediate-abort plan for prefetch targeting current page", () => {
		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/dashboard#next",
				navigationType: "prefetch",
			}),
			currentHref: "http://localhost:3000/dashboard#current",
			lanes: {
				active: null,
				revalidation: null,
				prefetch: new Map(),
			},
		});

		expect(executionPlan).toEqual({
			type: "immediateAbort",
			abortInstructions: [],
		});
	});

	it("reuses matching prefetch entry for prefetch intent", () => {
		const matchedPrefetch = createEntry({
			targetUrl: "http://localhost:3000/dashboard#cached",
			type: "prefetch",
			intent: "none",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/dashboard#next",
				navigationType: "prefetch",
			}),
			currentHref: "http://localhost:3000/current",
			lanes: {
				active: null,
				revalidation: null,
				prefetch: new Map([
					[matchedPrefetch.targetUrl, matchedPrefetch],
				]),
			},
		});

		expect(executionPlan).toEqual({
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: matchedPrefetch.targetUrl,
				entry: matchedPrefetch,
				promotion: null,
			},
		});
	});

	it("reuses matching revalidation lane and ignores explicit href", () => {
		const revalidationEntry = createEntry({
			targetUrl: "http://localhost:3000/account#cached",
			type: "revalidation",
			intent: "revalidate",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/ignored",
				navigationType: "revalidation",
			}),
			currentHref: "http://localhost:3000/account#live",
			lanes: {
				active: null,
				revalidation: revalidationEntry,
				prefetch: new Map(),
			},
		});

		expect(executionPlan).toEqual({
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidationEntry,
				promotion: null,
			},
		});
	});

	it("aborts stale revalidation lane before creating a new revalidation", () => {
		const staleRevalidation = createEntry({
			targetUrl: "http://localhost:3000/other",
			type: "revalidation",
			intent: "revalidate",
		});

		const executionPlan = decideBeginNavigationExecutionPlan({
			navigationProps: createNavigationProps({
				href: "http://localhost:3000/ignored",
				navigationType: "revalidation",
			}),
			currentHref: "http://localhost:3000/settings",
			lanes: {
				active: null,
				revalidation: staleRevalidation,
				prefetch: new Map(),
			},
		});

		expect(executionPlan).toEqual({
			type: "create",
			abortInstructions: [
				{
					slot: "revalidation",
					entry: staleRevalidation,
				},
			],
			createInstruction: {
				slot: "revalidation",
				revalidationHref: "http://localhost:3000/settings",
			},
		});
	});
});

import { describe, expect, it, vi } from "vitest";
import {
	deleteNavigationFromSlots,
	findNavigationEntryInSlots,
	transitionNavigationPhaseInSlots,
	type NavigationSlots,
} from "../../core/navigation/runtime.ts";
import type { NavigationEntry } from "../../core/navigation/types.ts";
import {
	isSkipEligibilityViolated,
	type SkipCheckContext,
} from "../../core/navigation/fetch_route_data.ts";

function createEntry(props: {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
}): NavigationEntry {
	return {
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl,
		originUrl: window.location.href,
	};
}

describe("navigation bookkeeping key aliasing", () => {
	it("finds active navigation by same data target when hash differs", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationSlots = {
			activeNavigation: active,
			prefetchCache: new Map(),
			pendingRevalidation: null,
		};

		const found = findNavigationEntryInSlots(
			slots,
			"http://localhost:3000/alias#second",
		);
		expect(found).toBe(active);
	});

	it("does not alias active navigation when search params differ", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias?tab=a#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationSlots = {
			activeNavigation: active,
			prefetchCache: new Map(),
			pendingRevalidation: null,
		};

		const found = findNavigationEntryInSlots(
			slots,
			"http://localhost:3000/alias?tab=b#first",
		);
		expect(found).toBeUndefined();
	});

	it("deletes prefetch entry by same data target when hash differs", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map([[prefetch.targetUrl, prefetch]]),
			pendingRevalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromSlots(
			slots,
			"http://localhost:3000/prefetch#second",
			onStatusRelevantChange,
		);

		expect(deleted).toBe(true);
		expect(slots.prefetchCache.size).toBe(0);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});

	it("does not delete prefetch entry when search params differ", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch?tab=a#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map([[prefetch.targetUrl, prefetch]]),
			pendingRevalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromSlots(
			slots,
			"http://localhost:3000/prefetch?tab=b#first",
			onStatusRelevantChange,
		);

		expect(deleted).toBe(false);
		expect(slots.prefetchCache.size).toBe(1);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});

	it("transitions pending revalidation phase by same data target alias", () => {
		const pendingRevalidation = createEntry({
			targetUrl: "http://localhost:3000/revalidate#first",
			type: "revalidation",
			intent: "revalidate",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map(),
			pendingRevalidation,
		};
		const onStatusRelevantChange = vi.fn();

		transitionNavigationPhaseInSlots(
			slots,
			"http://localhost:3000/revalidate#second",
			"waiting",
			onStatusRelevantChange,
		);

		expect(slots.pendingRevalidation?.phase).toBe("waiting");
		expect(onStatusRelevantChange).toHaveBeenCalledTimes(1);
	});

	it("does not transition pending revalidation phase when search params differ", () => {
		const pendingRevalidation = createEntry({
			targetUrl: "http://localhost:3000/revalidate?view=a#first",
			type: "revalidation",
			intent: "revalidate",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map(),
			pendingRevalidation,
		};
		const onStatusRelevantChange = vi.fn();

		transitionNavigationPhaseInSlots(
			slots,
			"http://localhost:3000/revalidate?view=b#first",
			"waiting",
			onStatusRelevantChange,
		);

		expect(slots.pendingRevalidation?.phase).toBe("fetching");
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});
});

function buildContext(targetHref: string): SkipCheckContext {
	return {
		routeManifest: { "/items": 1 },
		patternRegistry: {},
		patternToWaitFnMap: {},
		clientModuleMap: {
			"/items": {
				importURL: "/items.js",
				exportKey: "default",
				errorExportKey: "",
			},
		},
		currentMatchedPatterns: ["/items"],
		currentParams: {},
		currentSplatValues: [],
		currentLoadersData: [{}],
		url: new URL(targetHref),
		matchResult: {
			matches: [
				{
					registeredPattern: {
						originalPattern: "/items",
						normalizedSegments: [],
						lastSegType: "static",
					},
				},
			],
			params: {},
			splatValues: [],
		},
	};
}

describe("skip server fetch eligibility", () => {
	it("treats query order changes as changed for skip gating", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?b=2&a=1");

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("allows skip gating when query string is exactly unchanged", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?a=1&b=2");

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});
});

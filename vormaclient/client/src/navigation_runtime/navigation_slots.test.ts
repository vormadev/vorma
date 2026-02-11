import { describe, expect, it, vi } from "vitest";
import {
	deleteNavigationFromSlots,
	transitionNavigationPhaseInSlots,
} from "./navigation_slot_mutations.ts";
import {
	findNavigationEntryInSlots,
	type NavigationSlots,
} from "./navigation_slots.ts";
import type { NavigationEntry } from "./types.ts";

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

describe("navigation slot key aliasing", () => {
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
});

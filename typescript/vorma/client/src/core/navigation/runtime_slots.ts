import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import type { StatusEventDetail } from "../../platform/events.ts";
import {
	findMapEntryByNavigationTarget,
	hasSameNavigationTarget,
} from "../../platform/url.ts";
import type {
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

export type NavigationLanes = {
	active: NavigationEntry | null;
	revalidation: NavigationEntry | null;
	prefetch: Map<string, NavigationEntry>;
};

export type RuntimeLanes = NavigationLanes & {
	submissions: Map<string | symbol, SubmissionEntry>;
};

export function createRuntimeLanes(): RuntimeLanes {
	return {
		active: null,
		revalidation: null,
		prefetch: new Map<string, NavigationEntry>(),
		submissions: new Map<string | symbol, SubmissionEntry>(),
	};
}

export function computeNavigationStatus(props: {
	lanes: RuntimeLanes;
}): StatusEventDetail {
	const { lanes } = props;

	const isNavigating =
		lanes.active !== null &&
		lanes.active.intent === "navigate" &&
		lanes.active.phase !== "complete";

	const isRevalidating =
		lanes.revalidation !== null && lanes.revalidation.phase !== "complete";

	let isSubmitting = false;
	for (const submission of lanes.submissions.values()) {
		if (!submission.skipGlobalLoadingIndicator) {
			isSubmitting = true;
			break;
		}
	}

	return { isNavigating, isSubmitting, isRevalidating };
}

export function createStatusSignaler(props: {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
}): () => void {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = props;
	let lastDispatchedStatus: StatusEventDetail | null = null;

	const scheduleStatusUpdate = debounce(() => {
		const newStatus = getStatus();
		if (jsonDeepEquals(lastDispatchedStatus, newStatus)) {
			return;
		}
		lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}, debounceMS);

	return scheduleStatusUpdate;
}

type NavigationLaneMatch =
	| {
			lane: "active";
			entry: NavigationEntry;
	  }
	| {
			lane: "prefetch";
			key: string;
			entry: NavigationEntry;
	  }
	| {
			lane: "revalidation";
			entry: NavigationEntry;
	  };

export function findNavigationEntryInNavigationLanes(props: {
	lanes: NavigationLanes;
	targetUrl: string;
}): NavigationEntry | undefined {
	return matchNavigationLaneByTargetURL(props)?.entry;
}

export function getNavigationsSizeFromNavigationLanes(props: {
	lanes: NavigationLanes;
}): number {
	const { lanes } = props;
	let size = 0;
	if (lanes.active) size++;
	size += lanes.prefetch.size;
	if (lanes.revalidation) size++;
	return size;
}

export function buildNavigationsMapFromNavigationLanes(props: {
	lanes: NavigationLanes;
}): Map<string, NavigationEntry> {
	const { lanes } = props;
	const map = new Map<string, NavigationEntry>();
	if (lanes.active) {
		map.set(lanes.active.targetUrl, lanes.active);
	}
	for (const [key, entry] of lanes.prefetch) {
		map.set(key, entry);
	}
	if (lanes.revalidation) {
		map.set(lanes.revalidation.targetUrl, lanes.revalidation);
	}
	return map;
}

function matchNavigationLaneByTargetURL(props: {
	lanes: NavigationLanes;
	targetUrl: string;
}): NavigationLaneMatch | undefined {
	const { lanes, targetUrl } = props;
	if (
		lanes.active &&
		hasSameNavigationTarget({
			firstHref: lanes.active.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			lane: "active",
			entry: lanes.active,
		};
	}

	const prefetchLaneKey = findMapEntryByNavigationTarget({
		map: lanes.prefetch,
		targetHref: targetUrl,
	})?.[0];
	if (prefetchLaneKey) {
		return {
			lane: "prefetch",
			key: prefetchLaneKey,
			entry: lanes.prefetch.get(prefetchLaneKey)!,
		};
	}

	if (
		lanes.revalidation &&
		hasSameNavigationTarget({
			firstHref: lanes.revalidation.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			lane: "revalidation",
			entry: lanes.revalidation,
		};
	}

	return undefined;
}

export function deleteNavigationFromNavigationLanes(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	onStatusRelevantChange: () => void;
}): boolean {
	const { lanes, targetUrl, onStatusRelevantChange } = props;
	const matchedLane = matchNavigationLaneByTargetURL({
		lanes,
		targetUrl,
	});
	if (!matchedLane) {
		return false;
	}

	switch (matchedLane.lane) {
		case "active":
			lanes.active = null;
			onStatusRelevantChange();
			return true;
		case "prefetch":
			lanes.prefetch.delete(matchedLane.key);
			return true;
		case "revalidation":
			lanes.revalidation = null;
			onStatusRelevantChange();
			return true;
	}
}

export function transitionNavigationPhaseInNavigationLanes(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	phase: NavigationPhase;
	onStatusRelevantChange: () => void;
}): void {
	const { lanes, targetUrl, phase, onStatusRelevantChange } = props;
	const matchedLane = matchNavigationLaneByTargetURL({ lanes, targetUrl });
	if (!matchedLane) {
		return;
	}

	matchedLane.entry.phase = phase;
	if (matchedLane.lane !== "prefetch") {
		onStatusRelevantChange();
	}
}

export function clearRuntimeLanes(props: {
	lanes: RuntimeLanes;
	onStatusRelevantChange: () => void;
}): void {
	const { lanes, onStatusRelevantChange } = props;
	if (lanes.active) {
		lanes.active.control.abortController?.abort();
		lanes.active = null;
	}

	for (const prefetchEntry of lanes.prefetch.values()) {
		prefetchEntry.control.abortController?.abort();
	}
	lanes.prefetch.clear();

	if (lanes.revalidation) {
		lanes.revalidation.control.abortController?.abort();
		lanes.revalidation = null;
	}

	for (const submissionEntry of lanes.submissions.values()) {
		submissionEntry.control.abortController?.abort();
	}
	lanes.submissions.clear();

	onStatusRelevantChange();
}

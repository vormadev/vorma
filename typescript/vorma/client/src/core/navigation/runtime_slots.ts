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

	const isSubmitting = Array.from(lanes.submissions.values()).some(
		(x) => !x.skipGlobalLoadingIndicator,
	);

	return { isNavigating, isSubmitting, isRevalidating };
}

export function createStatusSignaler(props: {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
}): { scheduleStatusUpdate: () => void } {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = props;
	let lastDispatchedStatus: StatusEventDetail | null = null;

	function dispatchStatusEventInternal(): void {
		const newStatus = getStatus();
		if (jsonDeepEquals(lastDispatchedStatus, newStatus)) {
			return;
		}
		lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}

	const scheduleStatusUpdate = debounce(() => {
		dispatchStatusEventInternal();
	}, debounceMS);

	return {
		scheduleStatusUpdate,
	};
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

function findMatchingPrefetchLaneKey(props: {
	lanes: NavigationLanes;
	key: string;
}): string | undefined {
	return findMapEntryByNavigationTarget({
		map: props.lanes.prefetch,
		targetHref: props.key,
	})?.[0];
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

	const prefetchLaneKey = findMatchingPrefetchLaneKey({
		lanes,
		key: targetUrl,
	});
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
	const action = decideDeleteNavigationLaneAction({
		lanes,
		targetUrl,
	});
	return executeDeleteNavigationLaneAction({
		lanes,
		action,
		onStatusRelevantChange,
	});
}

type DeleteNavigationLaneAction =
	| {
			type: "stop";
	  }
	| {
			type: "clearActive";
	  }
	| {
			type: "deletePrefetch";
			key: string;
	  }
	| {
			type: "clearRevalidation";
	  };

function decideDeleteNavigationLaneAction(props: {
	lanes: NavigationLanes;
	targetUrl: string;
}): DeleteNavigationLaneAction {
	const matchedLane = matchNavigationLaneByTargetURL(props);
	if (!matchedLane) {
		return { type: "stop" };
	}

	switch (matchedLane.lane) {
		case "active":
			return { type: "clearActive" };
		case "prefetch":
			return {
				type: "deletePrefetch",
				key: matchedLane.key,
			};
		case "revalidation":
			return { type: "clearRevalidation" };
	}
}

function executeDeleteNavigationLaneAction(props: {
	lanes: NavigationLanes;
	action: DeleteNavigationLaneAction;
	onStatusRelevantChange: () => void;
}): boolean {
	const { lanes, action, onStatusRelevantChange } = props;
	switch (action.type) {
		case "stop":
			return false;
		case "clearActive":
			lanes.active = null;
			onStatusRelevantChange();
			return true;
		case "deletePrefetch":
			lanes.prefetch.delete(action.key);
			return true;
		case "clearRevalidation":
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
	const action = decideTransitionNavigationPhaseAction(props);
	executeTransitionNavigationPhaseAction({
		action,
		onStatusRelevantChange: props.onStatusRelevantChange,
	});
}

type TransitionNavigationPhaseAction =
	| {
			type: "stop";
	  }
	| {
			type: "setPhase";
			entry: NavigationEntry;
			phase: NavigationPhase;
			shouldSignalStatusChange: boolean;
	  };

function decideTransitionNavigationPhaseAction(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	phase: NavigationPhase;
}): TransitionNavigationPhaseAction {
	const matchedLane = matchNavigationLaneByTargetURL({
		lanes: props.lanes,
		targetUrl: props.targetUrl,
	});
	if (!matchedLane) {
		return { type: "stop" };
	}

	return {
		type: "setPhase",
		entry: matchedLane.entry,
		phase: props.phase,
		shouldSignalStatusChange: matchedLane.lane !== "prefetch",
	};
}

function executeTransitionNavigationPhaseAction(props: {
	action: TransitionNavigationPhaseAction;
	onStatusRelevantChange: () => void;
}): void {
	const { action, onStatusRelevantChange } = props;
	switch (action.type) {
		case "stop":
			return;
		case "setPhase":
			action.entry.phase = action.phase;
			if (action.shouldSignalStatusChange) {
				onStatusRelevantChange();
			}
			return;
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

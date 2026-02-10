import { dispatchStatusEvent, type StatusEventDetail } from "../events.ts";
import { createNavigationBookkeeping } from "./navigation_bookkeeping.ts";
import { computeNavigationStatus } from "./navigation_status.ts";
import { createStatusSignaler } from "./status_signaler.ts";
import type { NavigationBookkeeping } from "./navigation_bookkeeping.ts";
import type { SubmissionEntry } from "./types.ts";

export type RuntimeBookkeepingStatusSetup = {
	navigationBookkeeping: NavigationBookkeeping;
	getStatus: () => StatusEventDetail;
	scheduleStatusUpdate: () => void;
};

export function createRuntimeBookkeepingStatusSetup(
	submissions: Map<string | symbol, SubmissionEntry>,
): RuntimeBookkeepingStatusSetup {
	let scheduleStatusUpdate: () => void = () => {};

	const navigationBookkeeping = createNavigationBookkeeping({
		onStatusRelevantChange: () => {
			scheduleStatusUpdate();
		},
	});

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			activeNavigation: navigationBookkeeping.getActiveNavigation(),
			pendingRevalidation: navigationBookkeeping.getPendingRevalidation(),
			submissions,
		});
	}

	const statusSignaler = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});
	scheduleStatusUpdate = statusSignaler.scheduleStatusUpdate;

	return {
		navigationBookkeeping,
		getStatus,
		scheduleStatusUpdate,
	};
}

import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import type { StatusEventDetail } from "../events.ts";

export type StatusSignaler = {
	scheduleStatusUpdate: () => void;
};

export type CreateStatusSignalerOptions = {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
};

export function createStatusSignaler(
	options: CreateStatusSignalerOptions,
): StatusSignaler {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = options;
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

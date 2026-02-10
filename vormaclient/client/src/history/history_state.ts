import { createBrowserHistory } from "history";
import type { historyInstance } from "./npm_history_types.ts";

const historyState: {
	instance: historyInstance | undefined;
	lastKnownLocation: historyInstance["location"] | undefined;
} = {
	instance: undefined,
	lastKnownLocation: undefined,
};

export function getHistoryInstance(): historyInstance {
	if (!historyState.instance) {
		historyState.instance =
			createBrowserHistory() as unknown as historyInstance;
		historyState.lastKnownLocation = historyState.instance.location;
	}
	return historyState.instance;
}

export function getLastKnownHistoryLocation(): historyInstance["location"] {
	if (!historyState.lastKnownLocation) {
		historyState.lastKnownLocation = getHistoryInstance().location;
	}
	return historyState.lastKnownLocation;
}

export function setLastKnownHistoryLocation(
	location: historyInstance["location"],
): void {
	historyState.lastKnownLocation = location;
}

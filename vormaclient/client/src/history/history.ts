import type { Update as NPMHistoryUpdate } from "history";
import { dispatchLocationEvent } from "../events.ts";
import { saveScrollState } from "../scroll_state_manager.ts";
import { analyzeHistoryListenerPrelude } from "./history_listener_prelude.ts";
import { handlePopNavigationForHistoryUpdate } from "./history_pop_navigation.ts";
import {
	getHistoryInstance,
	getLastKnownHistoryLocation,
	setLastKnownHistoryLocation,
} from "./history_state.ts";
import type { historyInstance, historyListener } from "./npm_history_types.ts";

function getInstance(): historyInstance {
	return getHistoryInstance();
}

function getLastKnownLocation(): historyInstance["location"] {
	return getLastKnownHistoryLocation();
}

function updateLastKnownLocation(location: historyInstance["location"]): void {
	setLastKnownHistoryLocation(location);
}

function setManualScrollRestoration(): void {
	if (history.scrollRestoration && history.scrollRestoration !== "manual") {
		history.scrollRestoration = "manual";
	}
}

function init(): void {
	const instance = getInstance();
	instance.listen(customHistoryListener as unknown as historyListener);
	setManualScrollRestoration();
}

export const HistoryManager = {
	getInstance,
	getLastKnownLocation,
	updateLastKnownLocation,
	init,
};

export function initCustomHistory(): void {
	init();
}

export async function customHistoryListener({
	action,
	location,
}: NPMHistoryUpdate): Promise<void> {
	const lastKnownLocation = getLastKnownLocation();
	const { didLocationKeyChange, popWithinSameDoc, shouldSaveScrollState } =
		analyzeHistoryListenerPrelude({
			action,
			location,
			lastKnownLocation,
		});

	if (didLocationKeyChange) {
		dispatchLocationEvent();
	}

	if (shouldSaveScrollState) {
		saveScrollState();
	}

	let navigationSucceeded = true;

	if (action === "POP") {
		navigationSucceeded = await handlePopNavigationForHistoryUpdate(
			location,
			lastKnownLocation,
			popWithinSameDoc,
		);
	}

	if (navigationSucceeded) {
		updateLastKnownLocation(location);
	}
}

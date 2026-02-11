import { dispatchLocationEvent } from "../events.ts";
import { saveScrollState } from "../scroll_state_manager.ts";
import { analyzeHistoryListenerPrelude } from "./history_listener_prelude.ts";
import { handlePopNavigationForHistoryUpdate } from "./history_pop_navigation.ts";
import {
	getHistoryInstance,
	getLastKnownHistoryLocation,
	setLastKnownHistoryLocation,
} from "./history_state.ts";
import type { historyListener } from "./npm_history_types.ts";

function setManualScrollRestoration(): void {
	if (history.scrollRestoration && history.scrollRestoration !== "manual") {
		history.scrollRestoration = "manual";
	}
}

function initHistory(): void {
	const instance = getHistoryInstance();
	instance.listen((update) => {
		void customHistoryListener(update);
	});
	setManualScrollRestoration();
}

export const HistoryManager = {
	getInstance: getHistoryInstance,
	getLastKnownLocation: getLastKnownHistoryLocation,
	updateLastKnownLocation: setLastKnownHistoryLocation,
	init: initHistory,
};

export async function customHistoryListener({
	action,
	location,
}: Parameters<historyListener>[0]): Promise<void> {
	const lastKnownLocation = getLastKnownHistoryLocation();
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
		setLastKnownHistoryLocation(location);
	}
}

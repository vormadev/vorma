import { createBrowserHistory, type Update as NPMHistoryUpdate } from "history";
import { navigationStateManager } from "../client.ts";
import { dispatchLocationEvent } from "../events.ts";
import {
	__applyScrollState,
	saveScrollState,
	scrollStateManager,
} from "../scroll_state_manager.ts";
import { logError } from "../utils/logging.ts";
import type { historyInstance, historyListener } from "./npm_history_types.ts";

export class HistoryManager {
	private static instance: historyInstance;
	private static lastKnownLocation: typeof HistoryManager.instance.location;

	static getInstance(): historyInstance {
		if (!this.instance) {
			this.instance =
				createBrowserHistory() as unknown as historyInstance;
			this.lastKnownLocation = this.instance.location;
		}
		return this.instance;
	}

	static getLastKnownLocation() {
		return this.lastKnownLocation;
	}

	static updateLastKnownLocation(
		location: typeof HistoryManager.instance.location,
	) {
		this.lastKnownLocation = location;
	}

	static init(): void {
		const instance = this.getInstance();
		instance.listen(customHistoryListener as unknown as historyListener);
		this.setManualScrollRestoration();
	}

	private static setManualScrollRestoration(): void {
		if (
			history.scrollRestoration &&
			history.scrollRestoration !== "manual"
		) {
			history.scrollRestoration = "manual";
		}
	}
}

export function initCustomHistory(): void {
	HistoryManager.init();
}

export async function customHistoryListener({
	action,
	location,
}: NPMHistoryUpdate): Promise<void> {
	const lastKnownLocation = HistoryManager.getLastKnownLocation();

	if (location.key !== lastKnownLocation.key) {
		dispatchLocationEvent();
	}

	const popWithinSameDoc =
		action === "POP" &&
		location.pathname === lastKnownLocation.pathname &&
		location.search === lastKnownLocation.search;

	const removingHash =
		popWithinSameDoc && lastKnownLocation.hash && !location.hash;
	const addingHash =
		popWithinSameDoc && !lastKnownLocation.hash && location.hash;
	const updatingHash = popWithinSameDoc && location.hash;

	if (!popWithinSameDoc) {
		saveScrollState();
	}

	let navigationSucceeded = true;

	if (action === "POP") {
		const newHash = location.hash.slice(1);

		if (addingHash || updatingHash) {
			__applyScrollState({ hash: newHash });
		}

		if (removingHash) {
			const stored = scrollStateManager.getState(location.key);
			__applyScrollState(stored ?? { x: 0, y: 0 });
		}

		if (!popWithinSameDoc) {
			const result = await navigationStateManager.navigate({
				href: window.location.href,
				navigationType: "browserHistory",
				scrollStateToRestore: scrollStateManager.getState(location.key),
			});

			if (!result.didNavigate) {
				navigationSucceeded = false;
				logError(
					"Browser POP navigation failed, attempting hard reload of the destination.",
				);

				// This just reloads the current (failed) URL.
				// It preserves the history stack and ensures no UI/URL mismatch,
				// which could otherwise happen if a browser forward/back navigation fails
				window.location.reload();
			}
		}
	}

	if (navigationSucceeded) {
		HistoryManager.updateLastKnownLocation(location);
	}
}

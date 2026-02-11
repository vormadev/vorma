import { HistoryManager } from "./history/history.ts";
import { normalizedHashFragmentFromHash } from "./hash_fragment.ts";
import {
	restoreRecentPageRefreshScrollState,
	savePageRefreshScrollStateSnapshot,
} from "./scroll_state_refresh_state.ts";
import {
	getStoredScrollState,
	saveStoredScrollState,
} from "./scroll_state_storage.ts";
import type { ScrollState } from "./scroll_state_types.ts";

export type { ScrollState } from "./scroll_state_types.ts";

function createScrollStateManager() {
	function saveState(key: string, state: ScrollState): void {
		saveStoredScrollState(key, state);
	}

	function getState(key: string): ScrollState | undefined {
		return getStoredScrollState(key);
	}

	function savePageRefreshState(): void {
		savePageRefreshScrollStateSnapshot();
	}

	function restorePageRefreshState(): void {
		restoreRecentPageRefreshScrollState((x, y) => {
			__applyScrollState({ x, y });
		});
	}

	return {
		saveState,
		getState,
		savePageRefreshState,
		restorePageRefreshState,
	};
}

export const scrollStateManager = createScrollStateManager();

export function __applyScrollState(state?: ScrollState): void {
	if (!state) {
		const id = normalizedHashFragmentFromHash(window.location.hash);
		if (id) {
			document.getElementById(id)?.scrollIntoView();
		}
		return;
	}

	if ("hash" in state) {
		const hash = normalizedHashFragmentFromHash(state.hash);
		if (hash) {
			document.getElementById(hash)?.scrollIntoView();
		}
	} else {
		window.scrollTo(state.x, state.y);
	}
}

export function saveScrollState(): void {
	const lastKnownLocation = HistoryManager.getLastKnownLocation();
	scrollStateManager.saveState(lastKnownLocation.key, {
		x: window.scrollX,
		y: window.scrollY,
	});
}

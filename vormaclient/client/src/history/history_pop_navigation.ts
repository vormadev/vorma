import { getNavigationStateAccess } from "../navigation_state_access.ts";
import {
	__applyScrollState,
	scrollStateManager,
} from "../scroll_state_manager.ts";
import { logError } from "../utils/logging.ts";
import type { historyInstance } from "./npm_history_types.ts";

function applyHashDrivenPopScrollState(
	location: historyInstance["location"],
	lastKnownLocation: historyInstance["location"],
	popWithinSameDoc: boolean,
): void {
	const removingHash =
		popWithinSameDoc && !!lastKnownLocation.hash && !location.hash;
	const addingHash =
		popWithinSameDoc && !lastKnownLocation.hash && !!location.hash;
	const updatingHash = popWithinSameDoc && !!location.hash;

	if (addingHash || updatingHash) {
		__applyScrollState({ hash: location.hash.slice(1) });
	}

	if (removingHash) {
		const stored = scrollStateManager.getState(location.key);
		__applyScrollState(stored ?? { x: 0, y: 0 });
	}
}

async function navigateCrossDocumentPop(
	location: historyInstance["location"],
): Promise<boolean> {
	const result = await getNavigationStateAccess().navigate({
		href: window.location.href,
		navigationType: "browserHistory",
		scrollStateToRestore: scrollStateManager.getState(location.key),
	});

	if (result.didNavigate) {
		return true;
	}

	logError(
		"Browser POP navigation failed, attempting hard reload of the destination.",
	);

	// This just reloads the current (failed) URL.
	// It preserves the history stack and ensures no UI/URL mismatch,
	// which could otherwise happen if a browser forward/back navigation fails
	window.location.reload();
	return false;
}

export async function handlePopNavigationForHistoryUpdate(
	location: historyInstance["location"],
	lastKnownLocation: historyInstance["location"],
	popWithinSameDoc: boolean,
): Promise<boolean> {
	applyHashDrivenPopScrollState(
		location,
		lastKnownLocation,
		popWithinSameDoc,
	);

	if (popWithinSameDoc) {
		return true;
	}

	return navigateCrossDocumentPop(location);
}

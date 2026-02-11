import { getNavigationStateAccess } from "../navigation_state_access.ts";
import {
	__applyScrollState,
	scrollStateManager,
} from "../scroll_state_manager.ts";
import {
	hashFragmentFromHash,
	normalizedHashFragmentFromHash,
} from "../hash_fragment.ts";
import { logError } from "../utils/logging.ts";
import type { historyInstance } from "./npm_history_types.ts";

function applyHashDrivenPopScrollState(
	location: historyInstance["location"],
	lastKnownLocation: historyInstance["location"],
	popWithinSameDoc: boolean,
): void {
	const locationHashTarget = normalizedHashFragmentFromHash(location.hash);
	const lastKnownHashTarget = normalizedHashFragmentFromHash(
		lastKnownLocation.hash,
	);
	const hasLocationHashTarget = locationHashTarget !== "";
	const hasLastKnownHashTarget = lastKnownHashTarget !== "";
	const hashTargetChanged = locationHashTarget !== lastKnownHashTarget;
	const removingHash =
		popWithinSameDoc && hasLastKnownHashTarget && !hasLocationHashTarget;
	const addingHash =
		popWithinSameDoc && !hasLastKnownHashTarget && hasLocationHashTarget;
	const updatingHash =
		popWithinSameDoc && hasLocationHashTarget && hashTargetChanged;

	if (addingHash || updatingHash) {
		__applyScrollState({
			hash: hashFragmentFromHash(location.hash),
		});
	}

	if (removingHash) {
		const stored = scrollStateManager.getState(location.key);
		__applyScrollState(stored ?? { x: 0, y: 0 });
	}
}

function buildListenerLocationHref(
	location: historyInstance["location"],
): string {
	return new URL(
		`${location.pathname}${location.search}${location.hash}`,
		window.location.origin,
	).href;
}

async function navigateCrossDocumentPop(
	location: historyInstance["location"],
): Promise<boolean> {
	const result = await getNavigationStateAccess().navigate({
		href: buildListenerLocationHref(location),
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

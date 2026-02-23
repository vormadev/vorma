import {
	createBrowserHistory,
	type Action,
	type BrowserHistory,
	type Location,
	type Update,
} from "history";
import { resolveAbsoluteHref } from "vorma/kit/url";
import { getNavigationStateAccess } from "../app/context.ts";
import { dispatchLocationEvent } from "./events.ts";
import { logError } from "./safety.ts";
import {
	__applyScrollState,
	saveScrollState,
	scrollStateManager,
} from "./scroll.ts";
import {
	hasSameDataTarget,
	hashFragmentFromHash,
	normalizedHashFragmentFromHash,
} from "./url.ts";

const historyState: {
	instance: BrowserHistory | undefined;
	lastKnownLocation: Location | undefined;
} = {
	instance: undefined,
	lastKnownLocation: undefined,
};

let cleanupHistoryListener: (() => void) | null = null;
let latestHistoryListenerSequenceIssued = 0;
let historyListenerProcessingTail: Promise<void> = Promise.resolve();

function issueHistoryListenerSequence(): number {
	latestHistoryListenerSequenceIssued += 1;
	return latestHistoryListenerSequenceIssued;
}

function isLatestHistoryListenerSequence(sequence: number): boolean {
	return sequence === latestHistoryListenerSequenceIssued;
}

function getHistoryInstance(): BrowserHistory {
	if (!historyState.instance) {
		historyState.instance = createBrowserHistory();
		historyState.lastKnownLocation = historyState.instance.location;
	}
	return historyState.instance;
}

function getLastKnownHistoryLocation(): Location {
	if (!historyState.lastKnownLocation) {
		historyState.lastKnownLocation = getHistoryInstance().location;
	}
	return historyState.lastKnownLocation;
}

function setLastKnownHistoryLocation(location: Location): void {
	historyState.lastKnownLocation = location;
}

type HistoryLocationPrelude = Pick<Location, "key" | "pathname" | "search">;

function toAbsoluteHref(location: HistoryLocationPrelude): string {
	return resolveAbsoluteHref({
		href: `${location.pathname}${location.search}`,
		baseHref: window.location.origin,
	});
}

export function analyzeHistoryListenerPrelude(props: {
	action: Action;
	location: HistoryLocationPrelude;
	lastKnownLocation: HistoryLocationPrelude;
}): {
	didLocationKeyChange: boolean;
	popWithinSameDoc: boolean;
	shouldSaveScrollState: boolean;
} {
	const { action, location, lastKnownLocation } = props;
	const didLocationKeyChange = location.key !== lastKnownLocation.key;
	const popWithinSameDoc =
		action === "POP" &&
		hasSameDataTarget({
			firstHref: toAbsoluteHref(location),
			secondHref: toAbsoluteHref(lastKnownLocation),
		});

	return {
		didLocationKeyChange,
		popWithinSameDoc,
		shouldSaveScrollState: !popWithinSameDoc,
	};
}

function applyHashDrivenPopScrollState(props: {
	location: Location;
	lastKnownLocation: Location;
	popWithinSameDoc: boolean;
}): void {
	const { location, lastKnownLocation, popWithinSameDoc } = props;
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

function buildListenerLocationHref(location: Location): string {
	return resolveAbsoluteHref({
		href: `${location.pathname}${location.search}${location.hash}`,
		baseHref: window.location.origin,
	});
}

function isJSDOMEnvironment(): boolean {
	return /jsdom/i.test(navigator.userAgent);
}

function attemptHardReloadAfterFailedPopNavigation(): void {
	if (isJSDOMEnvironment()) {
		return;
	}

	try {
		window.location.reload();
	} catch (error) {
		logError(
			"Browser POP hard reload failed after client navigation fallback.",
			error,
		);
	}
}

async function navigateCrossDocumentPop(location: Location): Promise<boolean> {
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

	attemptHardReloadAfterFailedPopNavigation();
	return false;
}

async function handlePopNavigationForHistoryUpdate(props: {
	location: Location;
	lastKnownLocation: Location;
	popWithinSameDoc: boolean;
}): Promise<boolean> {
	const { location, lastKnownLocation, popWithinSameDoc } = props;
	applyHashDrivenPopScrollState({
		location,
		lastKnownLocation,
		popWithinSameDoc,
	});

	if (popWithinSameDoc) {
		return true;
	}

	return navigateCrossDocumentPop(location);
}

function setManualScrollRestoration(): void {
	if (history.scrollRestoration && history.scrollRestoration !== "manual") {
		history.scrollRestoration = "manual";
	}
}

function initHistory(): void {
	const instance = getHistoryInstance();
	cleanupHistoryListener?.();
	cleanupHistoryListener = instance.listen((update) => {
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

async function processHistoryUpdate({
	action,
	location,
}: Update): Promise<void> {
	const listenerSequence = issueHistoryListenerSequence();
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
		navigationSucceeded = await handlePopNavigationForHistoryUpdate({
			location,
			lastKnownLocation,
			popWithinSameDoc,
		});
	}

	if (
		navigationSucceeded &&
		isLatestHistoryListenerSequence(listenerSequence)
	) {
		setLastKnownHistoryLocation(location);
	}
}

export function customHistoryListener(update: Update): Promise<void> {
	const queuedHistoryUpdate = historyListenerProcessingTail.then(
		() => processHistoryUpdate(update),
		() => processHistoryUpdate(update),
	);
	historyListenerProcessingTail = queuedHistoryUpdate.then(
		() => undefined,
		() => undefined,
	);
	return queuedHistoryUpdate;
}

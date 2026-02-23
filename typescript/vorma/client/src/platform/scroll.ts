import { HistoryManager } from "./history.ts";
import {
	isSameDocumentLocation,
	normalizedHashFragmentFromHash,
} from "./url.ts";

export type ScrollState = { x: number; y: number } | { hash: string };

const STORAGE_KEY = "__vorma__scrollStateMap";
const MAX_ENTRIES = 50;
const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

type PageRefreshSnapshot = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

function safeSessionStorageGetItem(key: string): string | null {
	try {
		return sessionStorage.getItem(key);
	} catch {
		return null;
	}
}

function safeSessionStorageSetItem(key: string, value: string): void {
	try {
		sessionStorage.setItem(key, value);
	} catch {
		// Ignore sessionStorage write failures to keep navigation functional.
	}
}

function safeSessionStorageRemoveItem(key: string): void {
	try {
		sessionStorage.removeItem(key);
	} catch {
		// Ignore sessionStorage remove failures to keep navigation functional.
	}
}

function getStoredScrollStateMap(): Map<string, ScrollState> {
	const stored = safeSessionStorageGetItem(STORAGE_KEY);
	if (!stored) return new Map();

	try {
		return new Map(JSON.parse(stored));
	} catch {
		safeSessionStorageRemoveItem(STORAGE_KEY);
		return new Map();
	}
}

function setStoredScrollStateMap(map: Map<string, ScrollState>): void {
	safeSessionStorageSetItem(
		STORAGE_KEY,
		JSON.stringify(Array.from(map.entries())),
	);
}

export function saveStoredScrollState(key: string, state: ScrollState): void {
	const map = getStoredScrollStateMap();
	map.set(key, state);

	if (map.size > MAX_ENTRIES) {
		const firstKey = map.keys().next().value;
		if (firstKey !== undefined) map.delete(firstKey);
	}

	setStoredScrollStateMap(map);
}

export function getStoredScrollState(key: string): ScrollState | undefined {
	return getStoredScrollStateMap().get(key);
}

function isValidSnapshot(value: unknown): value is PageRefreshSnapshot {
	if (!value || typeof value !== "object") return false;
	const snapshot = value as Partial<PageRefreshSnapshot>;
	return (
		typeof snapshot.href === "string" &&
		typeof snapshot.unix === "number" &&
		Number.isFinite(snapshot.unix) &&
		typeof snapshot.x === "number" &&
		Number.isFinite(snapshot.x) &&
		typeof snapshot.y === "number" &&
		Number.isFinite(snapshot.y)
	);
}

function savePageRefreshScrollStateSnapshot(): void {
	const state = {
		x: window.scrollX,
		y: window.scrollY,
		unix: Date.now(),
		href: window.location.href,
	};
	safeSessionStorageSetItem(PAGE_REFRESH_KEY, JSON.stringify(state));
}

export function restoreRecentPageRefreshScrollState(
	applyState: (props: { x: number; y: number }) => void,
): void {
	const stored = safeSessionStorageGetItem(PAGE_REFRESH_KEY);
	if (!stored) return;

	try {
		const state = JSON.parse(stored);
		if (!isValidSnapshot(state)) {
			safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
			return;
		}

		const isRecentSnapshot = Date.now() - state.unix < 5000;
		const isCurrentLocation = isSameDocumentLocation({
			targetHref: state.href,
			currentHref: window.location.href,
		});
		if (!isCurrentLocation || !isRecentSnapshot) {
			safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
			return;
		}

		safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
		window.requestAnimationFrame(() => {
			applyState({ x: state.x, y: state.y });
		});
	} catch {
		safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
	}
}

export const scrollStateManager = {
	saveState: saveStoredScrollState,
	getState: getStoredScrollState,
	savePageRefreshState: savePageRefreshScrollStateSnapshot,
	restorePageRefreshState: () => {
		restoreRecentPageRefreshScrollState(({ x, y }) => {
			applyScrollState({ x, y });
		});
	},
};

export function applyScrollState(state?: ScrollState): void {
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

export const __applyScrollState = applyScrollState;

export function saveScrollState(): void {
	const lastKnownLocation = HistoryManager.getLastKnownLocation();
	scrollStateManager.saveState(lastKnownLocation.key, {
		x: window.scrollX,
		y: window.scrollY,
	});
}

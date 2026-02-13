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

function getStoredScrollStateMap(): Map<string, ScrollState> {
	const stored = sessionStorage.getItem(STORAGE_KEY);
	if (!stored) return new Map();

	try {
		return new Map(JSON.parse(stored));
	} catch {
		sessionStorage.removeItem(STORAGE_KEY);
		return new Map();
	}
}

function setStoredScrollStateMap(map: Map<string, ScrollState>): void {
	sessionStorage.setItem(
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
	sessionStorage.setItem(PAGE_REFRESH_KEY, JSON.stringify(state));
}

export function restoreRecentPageRefreshScrollState(
	applyState: (x: number, y: number) => void,
): void {
	const stored = sessionStorage.getItem(PAGE_REFRESH_KEY);
	if (!stored) return;

	try {
		const state = JSON.parse(stored);
		if (!isValidSnapshot(state)) {
			sessionStorage.removeItem(PAGE_REFRESH_KEY);
			return;
		}

		const isRecentSnapshot = Date.now() - state.unix < 5000;
		const isCurrentLocation = isSameDocumentLocation(
			state.href,
			window.location.href,
		);
		if (!isCurrentLocation || !isRecentSnapshot) {
			sessionStorage.removeItem(PAGE_REFRESH_KEY);
			return;
		}

		sessionStorage.removeItem(PAGE_REFRESH_KEY);
		window.requestAnimationFrame(() => {
			applyState(state.x, state.y);
		});
	} catch {
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
	}
}

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
			applyScrollState({ x, y });
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

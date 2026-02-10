import type { ScrollState } from "./scroll_state_types.ts";

const STORAGE_KEY = "__vorma__scrollStateMap";
const MAX_ENTRIES = 50;

function getStoredScrollStateMap(): Map<string, ScrollState> {
	const stored = sessionStorage.getItem(STORAGE_KEY);
	if (!stored) return new Map();

	try {
		return new Map(JSON.parse(stored));
	} catch {
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
		if (firstKey) map.delete(firstKey);
	}

	setStoredScrollStateMap(map);
}

export function getStoredScrollState(key: string): ScrollState | undefined {
	return getStoredScrollStateMap().get(key);
}

import { HistoryManager } from "./history/history.ts";

export type ScrollState = { x: number; y: number } | { hash: string };

class ScrollStateManager {
	private readonly STORAGE_KEY = "__vorma__scrollStateMap";
	private readonly PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";
	private readonly MAX_ENTRIES = 50;

	saveState(key: string, state: ScrollState): void {
		const map = this.getMap();
		map.set(key, state);

		// Enforce size limit
		if (map.size > this.MAX_ENTRIES) {
			const firstKey = map.keys().next().value;
			if (firstKey) map.delete(firstKey);
		}

		this.saveMap(map);
	}

	getState(key: string): ScrollState | undefined {
		return this.getMap().get(key);
	}

	savePageRefreshState(): void {
		const state = {
			x: window.scrollX,
			y: window.scrollY,
			unix: Date.now(),
			href: window.location.href,
		};
		sessionStorage.setItem(this.PAGE_REFRESH_KEY, JSON.stringify(state));
	}

	restorePageRefreshState(): void {
		const stored = sessionStorage.getItem(this.PAGE_REFRESH_KEY);
		if (!stored) return;

		try {
			const state = JSON.parse(stored);
			if (
				state.href === window.location.href &&
				Date.now() - state.unix < 5000
			) {
				sessionStorage.removeItem(this.PAGE_REFRESH_KEY);
				window.requestAnimationFrame(() => {
					__applyScrollState({ x: state.x, y: state.y });
				});
			}
		} catch {}
	}

	private getMap(): Map<string, ScrollState> {
		const stored = sessionStorage.getItem(this.STORAGE_KEY);
		if (!stored) return new Map();

		try {
			return new Map(JSON.parse(stored));
		} catch {
			return new Map();
		}
	}

	private saveMap(map: Map<string, ScrollState>): void {
		sessionStorage.setItem(
			this.STORAGE_KEY,
			JSON.stringify(Array.from(map.entries())),
		);
	}
}

export const scrollStateManager = new ScrollStateManager();

export function __applyScrollState(state?: ScrollState): void {
	if (!state) {
		const id = window.location.hash.slice(1);
		if (id) {
			document.getElementById(id)?.scrollIntoView();
		}
		return;
	}

	if ("hash" in state) {
		if (state.hash) {
			document.getElementById(state.hash)?.scrollIntoView();
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

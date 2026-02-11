import { isSameDocumentLocation } from "./hash_fragment.ts";

const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

type PageRefreshSnapshot = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

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

export function savePageRefreshScrollStateSnapshot(): void {
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

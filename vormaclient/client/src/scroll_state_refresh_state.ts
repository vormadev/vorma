const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

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
		if (
			state.href === window.location.href &&
			Date.now() - state.unix < 5000
		) {
			sessionStorage.removeItem(PAGE_REFRESH_KEY);
			window.requestAnimationFrame(() => {
				applyState(state.x, state.y);
			});
		}
	} catch {}
}

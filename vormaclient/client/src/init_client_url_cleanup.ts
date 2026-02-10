import { VORMA_HARD_RELOAD_QUERY_PARAM } from "./hard_reload.ts";
import { HistoryManager } from "./history/history.ts";

export function cleanupHardReloadQueryParam(): void {
	const url = new URL(window.location.href);
	if (!url.searchParams.has(VORMA_HARD_RELOAD_QUERY_PARAM)) {
		return;
	}

	url.searchParams.delete(VORMA_HARD_RELOAD_QUERY_PARAM);
	HistoryManager.getInstance().replace(url.href);
}

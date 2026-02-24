import { HistoryManager } from "./history.ts";

export type RuntimeLocationState = {
	pathname: string;
	search: string;
	hash: string;
	state: unknown;
};

/**
 * Returns a normalized browser location snapshot consumed by client adapters.
 */
export function getRuntimeLocationState(): RuntimeLocationState {
	return {
		pathname: window.location.pathname,
		search: window.location.search,
		hash: window.location.hash,
		state: HistoryManager.getInstance().location.state,
	};
}

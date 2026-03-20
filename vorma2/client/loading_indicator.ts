/// <reference types="vite/client" />

import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { addStatusListener } from "./events.ts";
import { get_loading_status, get_manager } from "./navigation.ts";
import { revalidate } from "./public_api.ts";
import type { GlobalLoadingIndicatorConfig } from "./types.ts";

const DEFAULT_DELAY = 12;

export function setupGlobalLoadingIndicator(
	config: GlobalLoadingIndicatorConfig,
): () => void {
	const inc_all = !config.include || config.include === "all";
	const inc_nav =
		inc_all ||
		(Array.isArray(config.include) &&
			config.include.includes("navigations"));
	const inc_sub =
		inc_all ||
		(Array.isArray(config.include) &&
			config.include.includes("submissions"));
	const inc_rev =
		inc_all ||
		(Array.isArray(config.include) &&
			config.include.includes("revalidations"));
	const start_delay = config.startDelayMS ?? DEFAULT_DELAY;
	const stop_delay = config.stopDelayMS ?? DEFAULT_DELAY;
	let start_timer: number | null = null;
	let stop_timer: number | null = null;

	function should_run(): boolean {
		const s = get_loading_status(get_manager().get_store());
		return (
			(inc_nav && s.isNavigating) ||
			(inc_sub && s.isSubmitting) ||
			(inc_rev && s.isRevalidating)
		);
	}

	function sync(): void {
		if (should_run()) {
			if (stop_timer !== null) {
				clearTimeout(stop_timer);
				stop_timer = null;
			}
			if (config.isRunning() || start_timer !== null) return;
			start_timer = window.setTimeout(() => {
				start_timer = null;
				if (!should_run() || config.isRunning()) return;
				config.start();
			}, start_delay);
		} else {
			if (start_timer !== null) {
				clearTimeout(start_timer);
				start_timer = null;
			}
			if (!config.isRunning() || stop_timer !== null) return;
			stop_timer = window.setTimeout(() => {
				stop_timer = null;
				if (should_run() || !config.isRunning()) return;
				config.stop();
			}, stop_delay);
		}
	}

	const remove = addStatusListener(() => sync());
	sync();
	return () => {
		remove();
		if (start_timer !== null) clearTimeout(start_timer);
		if (stop_timer !== null) clearTimeout(stop_timer);
		if (config.isRunning()) config.stop();
	};
}

export function revalidateOnWindowFocus(options?: {
	staleTimeMS?: number;
}): () => void {
	const stale_ms = options?.staleTimeMS ?? 5_000;
	return addOnWindowFocusListener(() => {
		const store = get_manager().get_store();
		if (Date.now() - store.last_nav_or_revalidate_ts >= stale_ms)
			void revalidate();
	});
}

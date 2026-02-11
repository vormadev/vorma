import { debounce } from "vorma/kit/debounce";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import {
	getLastTriggeredNavOrRevalidateTimestampMS,
	getStatus,
	revalidate,
} from "../client.ts";
import { addStatusListener, type StatusEvent } from "../platform/events.ts";
import { dispatchRouteChangeEvent } from "../platform/events.ts";
import { logInfo } from "../platform/safety.ts";
import { __vormaClientGlobal } from "../app/context.ts";
import { setupClientLoaders } from "./render_runtime.ts";

let devTimeSetupClientLoadersDebounced: () => Promise<void> = () =>
	Promise.resolve();

let hmrRevalidateSet: Set<string>;

export let __runClientLoadersAfterHMRUpdate: (
	importMeta: ImportMeta,
	pattern: string,
) => void = () => {};

type HotModuleRuntime = {
	on: (
		event: string,
		callback: (props: { updates: Array<{ type: string; path: string }> }) => void,
	) => void;
};

type HMRWindow = Window & {
	__waveRevalidate?: typeof revalidate;
};

function resolveHotModuleRuntime(
	importMeta: ImportMeta,
): HotModuleRuntime | undefined {
	const runtimeHot =
		(importMeta as ImportMeta & { hot?: unknown }).hot ??
		(import.meta as ImportMeta & { hot?: unknown }).hot;
	if (runtimeHot && typeof (runtimeHot as HotModuleRuntime).on === "function") {
		return runtimeHot as HotModuleRuntime;
	}
	return undefined;
}

export function initHMR() {
	if (import.meta.env.DEV) {
		(window as HMRWindow).__waveRevalidate = revalidate;

		devTimeSetupClientLoadersDebounced = debounce(async () => {
			await setupClientLoaders();
			dispatchRouteChangeEvent({});
		}, 10);

		__runClientLoadersAfterHMRUpdate = (importMeta, pattern) => {
			if (hmrRevalidateSet === undefined) {
				hmrRevalidateSet = new Set();
			}

			const hot = resolveHotModuleRuntime(importMeta);
			if (import.meta.env.DEV && hot) {
				const thisURL = new URL(importMeta.url, location.href);
				thisURL.search = "";
				const thisPathname = thisURL.pathname;

				const alreadyRegistered = hmrRevalidateSet.has(thisPathname);
				if (alreadyRegistered) {
					return;
				}

				hmrRevalidateSet.add(thisPathname);

				hot.on("vite:afterUpdate", (props) => {
					for (const update of props.updates) {
						if (update.type === "js-update") {
							const updateURL = new URL(update.path, location.href);
							updateURL.search = "";
							if (updateURL.pathname === thisURL.pathname) {
								if (
									__vormaClientGlobal
										.get("matchedPatterns")
										.includes(pattern)
								) {
									logInfo(
										"Refreshing client loaders due to change in pattern:",
										pattern,
									);
									devTimeSetupClientLoadersDebounced();
								}
							}
						}
					}
				});
			}
		};
	}
}

const DEFAULT_DELAY = 12;

type GlobalLoadingIndicatorIncludesOption =
	| "navigations"
	| "submissions"
	| "revalidations";

type GlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<GlobalLoadingIndicatorIncludesOption>;
	startDelayMS?: number;
	stopDelayMS?: number;
};

type ParsedGlobalLoadingIndicatorConfig = {
	includesAll: boolean;
	includesNavigations: boolean;
	includesSubmissions: boolean;
	includesRevalidations: boolean;
	startDelayMS: number;
	stopDelayMS: number;
};

function resolveIncludes(
	config: GlobalLoadingIndicatorConfig,
	includesOption: GlobalLoadingIndicatorIncludesOption,
) {
	const isArray = Array.isArray(config.include);
	return isArray && config.include?.includes(includesOption);
}

export function setupGlobalLoadingIndicator(
	config: GlobalLoadingIndicatorConfig,
) {
	let gliDebounceStartTimer: number | null = null;
	let gliDebounceStopTimer: number | null = null;
	const includesAll = !config.include || config.include === "all";
	const pc: ParsedGlobalLoadingIndicatorConfig = {
		includesAll,
		includesNavigations:
			resolveIncludes(config, "navigations") || includesAll,
		includesSubmissions:
			resolveIncludes(config, "submissions") || includesAll,
		includesRevalidations:
			resolveIncludes(config, "revalidations") || includesAll,
		startDelayMS: config.startDelayMS ?? DEFAULT_DELAY,
		stopDelayMS: config.stopDelayMS ?? DEFAULT_DELAY,
	};
	function clearStartTimer() {
		if (gliDebounceStartTimer !== null) {
			window.clearTimeout(gliDebounceStartTimer);
			gliDebounceStartTimer = null;
		}
	}
	function clearStopTimer() {
		if (gliDebounceStopTimer !== null) {
			window.clearTimeout(gliDebounceStopTimer);
			gliDebounceStopTimer = null;
		}
	}
	function clearTimers() {
		clearStartTimer();
		clearStopTimer();
	}
	function handleStatusChange(e?: StatusEvent) {
		const shouldBeWorking = getIsWorking(pc, e);
		if (shouldBeWorking) {
			clearStopTimer();
			if (gliDebounceStartTimer === null) {
				gliDebounceStartTimer = window.setTimeout(() => {
					gliDebounceStartTimer = null;
					if (!config.isRunning() && getIsWorking(pc)) {
						config.start();
					}
				}, pc.startDelayMS);
			}
		} else {
			clearStartTimer();
			if (gliDebounceStopTimer === null) {
				gliDebounceStopTimer = window.setTimeout(() => {
					gliDebounceStopTimer = null;
					if (config.isRunning() && !getIsWorking(pc)) {
						config.stop();
					}
				}, pc.stopDelayMS);
			}
		}
	}
	handleStatusChange();
	const removeStatusListenerCallback = addStatusListener(handleStatusChange);
	return () => {
		removeStatusListenerCallback();
		clearTimers();
		if (config.isRunning()) {
			config.stop();
		}
	};
}

function getIsWorking(
	pc: ParsedGlobalLoadingIndicatorConfig,
	e?: StatusEvent,
): boolean {
	const status = e?.detail ?? getStatus();
	if (pc.includesAll) {
		return (
			status.isNavigating || status.isSubmitting || status.isRevalidating
		);
	}
	if (pc.includesNavigations && status.isNavigating) {
		return true;
	}
	if (pc.includesSubmissions && status.isSubmitting) {
		return true;
	}
	if (pc.includesRevalidations && status.isRevalidating) {
		return true;
	}
	return false;
}

/**
 * If called, will setup listeners to revalidate the current route when
 * the window regains focus and at least `staleTimeMS` has passed since
 * the last revalidation. The `staleTimeMS` option defaults to 5,000
 * (5 seconds). Returns a cleanup function.
 */
export function revalidateOnWindowFocus(options?: { staleTimeMS?: number }) {
	const staleTimeMS = options?.staleTimeMS ?? 5_000;
	return addOnWindowFocusListener(() => {
		const status = getStatus();
		if (
			!status.isNavigating &&
			!status.isSubmitting &&
			!status.isRevalidating
		) {
			if (
				Date.now() - getLastTriggeredNavOrRevalidateTimestampMS() <
				staleTimeMS
			) {
				return;
			}
			revalidate();
		}
	});
}

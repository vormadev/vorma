import { debounce } from "vorma/kit/debounce";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import {
	__vormaClientGlobal,
	type PatternWaitFn,
	setClientLoaderWaitFn,
} from "../app/context.ts";
import {
	getLastTriggeredNavOrRevalidateTimestampMS,
	getStatus,
	revalidate,
} from "../client.ts";
import {
	addStatusListener,
	dispatchRouteChangeEvent,
	type StatusEvent,
	type StatusEventDetail,
} from "../platform/events.ts";
import { logInfo } from "../platform/safety.ts";
import {
	registerClientLoaderPatternOrThrow,
	setupClientLoaders,
} from "./render_runtime.ts";
export function shouldTriggerFocusRevalidation(props: {
	status: StatusEventDetail;
	nowTimestampMS: number;
	lastTriggeredNavOrRevalidateTimestampMS: number;
	staleTimeMS: number;
}): boolean {
	if (
		props.status.isNavigating ||
		props.status.isSubmitting ||
		props.status.isRevalidating
	) {
		return false;
	}

	if (
		props.nowTimestampMS - props.lastTriggeredNavOrRevalidateTimestampMS <
		props.staleTimeMS
	) {
		return false;
	}

	return true;
}

let devTimeSetupClientLoadersDebounced: () => Promise<void> = () =>
	Promise.resolve();

let hmrRegisteredPathnamesByRuntime: WeakMap<
	HotModuleRuntime,
	Set<string>
> = new WeakMap();
let hmrTrackedPatternsByRuntimeAndPathname: WeakMap<
	HotModuleRuntime,
	Map<string, Set<string>>
> = new WeakMap();

export let runClientLoadersAfterHMRUpdate: (
	importMeta: ImportMeta,
	pattern: string,
) => void = () => {};

export { runClientLoadersAfterHMRUpdate as __runClientLoadersAfterHMRUpdate };

type RegisterClientLoaderForAdapterProps = {
	pattern: string;
	waitFn: PatternWaitFn;
	reRunOnModuleChange?: ImportMeta;
	onRegistrationError?: (error: unknown) => void;
};

export function __registerClientLoaderForAdapter(
	props: RegisterClientLoaderForAdapterProps,
): void {
	const { pattern, waitFn, reRunOnModuleChange, onRegistrationError } = props;

	try {
		registerClientLoaderPatternOrThrow(pattern);
	} catch (error) {
		if (onRegistrationError) {
			onRegistrationError(error);
			return;
		}
		const reason = error instanceof Error ? error.message : String(error);
		throw new Error(
			`Failed to register client loader pattern "${pattern}": ${reason}`,
		);
	}

	setClientLoaderWaitFn(pattern, waitFn);

	if (import.meta.env.DEV && reRunOnModuleChange) {
		runClientLoadersAfterHMRUpdate(reRunOnModuleChange, pattern);
	}
}

type HotModuleRuntime = {
	on: (
		event: string,
		callback: (props: {
			updates: Array<{ type: string; path: string }>;
		}) => void,
	) => void;
};

type HMRWindow = Window & {
	// Wave dev runtime refresh script looks this up by name via
	// window[browserRevalidateFunctionName]. Keep this default key aligned with
	// wave/wave.go defaultBrowserRevalidateFunctionName.
	__waveRevalidate?: typeof revalidate;
};

function isHotModuleRuntime(value: unknown): value is HotModuleRuntime {
	return !!value && typeof (value as HotModuleRuntime).on === "function";
}

function resolveHotModuleRuntime(
	importMeta: ImportMeta,
): HotModuleRuntime | undefined {
	const importMetaHot = (importMeta as ImportMeta & { hot?: unknown }).hot;
	if (isHotModuleRuntime(importMetaHot)) {
		return importMetaHot;
	}

	const fallbackHot = (import.meta as ImportMeta & { hot?: unknown }).hot;
	return isHotModuleRuntime(fallbackHot) ? fallbackHot : undefined;
}

function normalizeModulePathnameForHMR(rawURL: string): string {
	const normalizedURL = new URL(rawURL, location.href);
	normalizedURL.search = "";
	return normalizedURL.pathname;
}

function shouldRefreshClientLoadersForTrackedHMRUpdate(props: {
	updates: Array<{ type: string; path: string }>;
	trackedPathname: string;
	trackedPatterns: Set<string>;
}): boolean {
	const { updates, trackedPathname, trackedPatterns } = props;
	const hasMatchingModuleUpdate = updates.some((update) => {
		if (update.type !== "js-update") {
			return false;
		}

		return normalizeModulePathnameForHMR(update.path) === trackedPathname;
	});
	if (!hasMatchingModuleUpdate) {
		return false;
	}

	const matchedPatterns = __vormaClientGlobal.get("matchedPatterns");
	const matchedPatternList = Array.isArray(matchedPatterns)
		? matchedPatterns
		: [];
	return Array.from(trackedPatterns).some((trackedPattern) =>
		matchedPatternList.includes(trackedPattern),
	);
}

function getRegisteredPathnamesForRuntime(
	hotRuntime: HotModuleRuntime,
): Set<string> {
	let registeredPathnames = hmrRegisteredPathnamesByRuntime.get(hotRuntime);
	if (!registeredPathnames) {
		registeredPathnames = new Set<string>();
		hmrRegisteredPathnamesByRuntime.set(hotRuntime, registeredPathnames);
	}

	return registeredPathnames;
}

function getTrackedPatternsForRuntimePathname(
	hotRuntime: HotModuleRuntime,
	pathname: string,
): Set<string> {
	let trackedPatternsByPathname =
		hmrTrackedPatternsByRuntimeAndPathname.get(hotRuntime);
	if (!trackedPatternsByPathname) {
		trackedPatternsByPathname = new Map<string, Set<string>>();
		hmrTrackedPatternsByRuntimeAndPathname.set(
			hotRuntime,
			trackedPatternsByPathname,
		);
	}

	let trackedPatterns = trackedPatternsByPathname.get(pathname);
	if (!trackedPatterns) {
		trackedPatterns = new Set<string>();
		trackedPatternsByPathname.set(pathname, trackedPatterns);
	}

	return trackedPatterns;
}

export function initHMR() {
	if (import.meta.env.DEV) {
		// Wave dev refresh calls this by name to trigger client revalidation.
		(window as HMRWindow).__waveRevalidate = revalidate;

		devTimeSetupClientLoadersDebounced = debounce(async () => {
			await setupClientLoaders();
			dispatchRouteChangeEvent({});
		}, 10);

		runClientLoadersAfterHMRUpdate = (importMeta, pattern) => {
			const hot = resolveHotModuleRuntime(importMeta);
			if (!hot) {
				return;
			}

			const registeredPathnames = getRegisteredPathnamesForRuntime(hot);
			const trackedPathname = normalizeModulePathnameForHMR(
				importMeta.url,
			);
			const trackedPatterns = getTrackedPatternsForRuntimePathname(
				hot,
				trackedPathname,
			);
			trackedPatterns.add(pattern);

			if (registeredPathnames.has(trackedPathname)) {
				return;
			}

			registeredPathnames.add(trackedPathname);

			hot.on("vite:afterUpdate", ({ updates }) => {
				if (
					!shouldRefreshClientLoadersForTrackedHMRUpdate({
						updates,
						trackedPathname,
						trackedPatterns,
					})
				) {
					return;
				}

				logInfo(
					"Refreshing client loaders due to change in pattern:",
					Array.from(trackedPatterns).join(", "),
				);
				devTimeSetupClientLoadersDebounced();
			});
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

function parseGlobalLoadingIndicatorConfig(
	config: GlobalLoadingIndicatorConfig,
): ParsedGlobalLoadingIndicatorConfig {
	const includesAll = !config.include || config.include === "all";
	const includeList =
		!includesAll && Array.isArray(config.include) ? config.include : [];

	return {
		includesAll,
		includesNavigations: includesAll || includeList.includes("navigations"),
		includesSubmissions: includesAll || includeList.includes("submissions"),
		includesRevalidations:
			includesAll || includeList.includes("revalidations"),
		startDelayMS: config.startDelayMS ?? DEFAULT_DELAY,
		stopDelayMS: config.stopDelayMS ?? DEFAULT_DELAY,
	};
}

export function setupGlobalLoadingIndicator(
	config: GlobalLoadingIndicatorConfig,
) {
	let gliDebounceStartTimer: number | null = null;
	let gliDebounceStopTimer: number | null = null;
	const pc = parseGlobalLoadingIndicatorConfig(config);
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
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: getStatus(),
			nowTimestampMS: Date.now(),
			lastTriggeredNavOrRevalidateTimestampMS:
				getLastTriggeredNavOrRevalidateTimestampMS(),
			staleTimeMS,
		});
		if (shouldRevalidate) {
			revalidate();
		}
	});
}

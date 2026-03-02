type ViteHMRUpdate = {
	type: string;
	path: string;
};

type NullableNumber = number | null | undefined;

type RuntimeRouteSnapshotForHMRRefresh = {
	matchedPatterns: string[];
	loadersData: unknown[];
	hasRootData: boolean;
	params: Record<string, string>;
	splatValues: string[];
	buildID: string;
	clientLoadersData: unknown[];
	outermostClientError: unknown;
	outermostClientErrorIdx: NullableNumber;
	outermostServerError: unknown;
	outermostServerErrorIdx: NullableNumber;
	outermostError: unknown;
	outermostErrorIdx: NullableNumber;
};

type ClientLoaderWaitFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		buildID: string;
	}>;
	signal: AbortSignal;
}) => Promise<unknown>;

type RuntimeGlobalForHMRRefresh = {
	runtimeRouteSnapshot?: RuntimeRouteSnapshotForHMRRefresh;
	patternToWaitFnMap?: Record<string, ClientLoaderWaitFn>;
};

type ImportMetaHot = {
	on: (
		event: "vite:afterUpdate",
		listener: (payload: { updates: Array<ViteHMRUpdate> }) => void,
	) => void;
};

const VORMA_SYMBOL = Symbol.for("__vorma_internal__");
const ROUTE_CHANGE_EVENT_NAME = "vorma:route-change";

const trackedPatternsByModulePath = new Map<string, Set<string>>();
const registeredViteHotRuntimes = new WeakSet<ImportMetaHot>();

function normalizeHMRModulePath(path: string): string {
	return new URL(path, window.location.href).pathname;
}

function resolveRuntimeGlobalForHMRRefresh():
	| RuntimeGlobalForHMRRefresh
	| undefined {
	const maybeGlobal = (globalThis as Record<PropertyKey, unknown>)[
		VORMA_SYMBOL
	] as RuntimeGlobalForHMRRefresh | undefined;
	if (!maybeGlobal || typeof maybeGlobal !== "object") {
		return undefined;
	}
	return maybeGlobal;
}

function resolveTrackedPatternsForModulePath(path: string): Set<string> {
	const normalizedModulePath = normalizeHMRModulePath(path);
	const existingTrackedPatterns =
		trackedPatternsByModulePath.get(normalizedModulePath);
	if (existingTrackedPatterns) {
		return existingTrackedPatterns;
	}
	const nextTrackedPatterns = new Set<string>();
	trackedPatternsByModulePath.set(normalizedModulePath, nextTrackedPatterns);
	return nextTrackedPatterns;
}

function resolveTrackedPatternsForUpdatePath(
	updatePath: string,
): Set<string> | undefined {
	return trackedPatternsByModulePath.get(normalizeHMRModulePath(updatePath));
}

function resolveOutermostClientErrorForTargetedRefresh(props: {
	snapshotBeforeRefresh: RuntimeRouteSnapshotForHMRRefresh;
	matchedPatternsToRefresh: Set<string>;
	refreshedOutermostClientError: unknown;
	refreshedOutermostClientErrorIdx: NullableNumber;
}): {
	outermostClientError: unknown;
	outermostClientErrorIdx: NullableNumber;
} {
	const snapshotBeforeRefresh = props.snapshotBeforeRefresh;
	const previousOutermostClientErrorIdx =
		snapshotBeforeRefresh.outermostClientErrorIdx;
	const shouldDropPreviousOutermostClientError =
		previousOutermostClientErrorIdx != null &&
		props.matchedPatternsToRefresh.has(
			snapshotBeforeRefresh.matchedPatterns[
				previousOutermostClientErrorIdx
			] as string,
		);
	const previousCandidate =
		!shouldDropPreviousOutermostClientError &&
		previousOutermostClientErrorIdx != null
			? {
					error: snapshotBeforeRefresh.outermostClientError,
					idx: previousOutermostClientErrorIdx,
				}
			: undefined;
	const refreshedCandidate =
		props.refreshedOutermostClientErrorIdx != null
			? {
					error: props.refreshedOutermostClientError,
					idx: props.refreshedOutermostClientErrorIdx,
				}
			: undefined;
	if (!previousCandidate) {
		return {
			outermostClientError: refreshedCandidate?.error,
			outermostClientErrorIdx: refreshedCandidate?.idx,
		};
	}
	if (!refreshedCandidate) {
		return {
			outermostClientError: previousCandidate.error,
			outermostClientErrorIdx: previousCandidate.idx,
		};
	}
	if (refreshedCandidate.idx < previousCandidate.idx) {
		return {
			outermostClientError: refreshedCandidate.error,
			outermostClientErrorIdx: refreshedCandidate.idx,
		};
	}
	return {
		outermostClientError: previousCandidate.error,
		outermostClientErrorIdx: previousCandidate.idx,
	};
}

async function completeTargetedClientLoaderRefresh(props: {
	snapshotBeforeRefresh: RuntimeRouteSnapshotForHMRRefresh;
	matchedPatternsToRefresh: Set<string>;
	patternToWaitFnMap: Record<string, ClientLoaderWaitFn>;
}): Promise<{
	clientLoadersData: unknown[];
	outermostClientError: unknown;
	outermostClientErrorIdx: NullableNumber;
}> {
	const clientLoadersData = Array.from(
		props.snapshotBeforeRefresh.clientLoadersData,
	);
	let outermostClientError: unknown = undefined;
	let outermostClientErrorIdx: NullableNumber = undefined;

	await Promise.all(
		props.snapshotBeforeRefresh.matchedPatterns.map(
			async (pattern, index) => {
				if (!props.matchedPatternsToRefresh.has(pattern)) {
					return;
				}
				const waitFn = props.patternToWaitFnMap[pattern];
				if (!waitFn) {
					return;
				}
				const serverDataPromise = Promise.resolve({
					matchedPatterns:
						props.snapshotBeforeRefresh.matchedPatterns,
					rootData: props.snapshotBeforeRefresh.hasRootData
						? props.snapshotBeforeRefresh.loadersData[0]
						: null,
					loaderData: props.snapshotBeforeRefresh.loadersData[index],
					buildID: props.snapshotBeforeRefresh.buildID,
				});
				try {
					clientLoadersData[index] = await waitFn({
						params: props.snapshotBeforeRefresh.params,
						splatValues: props.snapshotBeforeRefresh.splatValues,
						serverDataPromise,
						signal: new AbortController().signal,
					});
				} catch (error) {
					if (outermostClientErrorIdx == null) {
						outermostClientError = error;
						outermostClientErrorIdx = index;
					}
				}
			},
		),
	);

	return {
		clientLoadersData,
		outermostClientError,
		outermostClientErrorIdx,
	};
}

export async function refreshCurrentClientLoadersAfterHMRUpdate(props: {
	matchedPatternsToRefresh: string[];
}): Promise<boolean> {
	if (props.matchedPatternsToRefresh.length === 0) {
		return false;
	}
	const runtimeGlobal = resolveRuntimeGlobalForHMRRefresh();
	if (
		!runtimeGlobal?.runtimeRouteSnapshot ||
		!runtimeGlobal.patternToWaitFnMap
	) {
		return false;
	}
	const snapshotBeforeRefresh = runtimeGlobal.runtimeRouteSnapshot;
	const matchedPatternsToRefresh = new Set(props.matchedPatternsToRefresh);
	const clientLoaderResult = await completeTargetedClientLoaderRefresh({
		snapshotBeforeRefresh,
		matchedPatternsToRefresh,
		patternToWaitFnMap: runtimeGlobal.patternToWaitFnMap,
	});
	const outermostClientErrorState =
		resolveOutermostClientErrorForTargetedRefresh({
			snapshotBeforeRefresh,
			matchedPatternsToRefresh,
			refreshedOutermostClientError:
				clientLoaderResult.outermostClientError,
			refreshedOutermostClientErrorIdx:
				clientLoaderResult.outermostClientErrorIdx,
		});
	const nextSnapshot = {
		...snapshotBeforeRefresh,
		clientLoadersData: clientLoaderResult.clientLoadersData,
		outermostClientError: outermostClientErrorState.outermostClientError,
		outermostClientErrorIdx:
			outermostClientErrorState.outermostClientErrorIdx,
		outermostError:
			outermostClientErrorState.outermostClientError ??
			snapshotBeforeRefresh.outermostServerError,
		outermostErrorIdx:
			outermostClientErrorState.outermostClientErrorIdx ??
			snapshotBeforeRefresh.outermostServerErrorIdx,
	} as RuntimeRouteSnapshotForHMRRefresh;
	runtimeGlobal.runtimeRouteSnapshot = nextSnapshot;
	window.dispatchEvent(
		new CustomEvent(ROUTE_CHANGE_EVENT_NAME, {
			detail: {},
		}),
	);
	return true;
}

export function registerClientLoaderModuleForHMRRerun(props: {
	importMeta: Pick<ImportMeta, "url">;
	pattern: string;
}): void {
	if (!import.meta.env.DEV) {
		return;
	}
	if (!props.pattern) {
		return;
	}
	if (
		typeof props.importMeta.url !== "string" ||
		props.importMeta.url === ""
	) {
		return;
	}
	resolveTrackedPatternsForModulePath(props.importMeta.url).add(
		props.pattern,
	);
}

export function registerClientLoaderModuleForHMRRerunFromAdapterProps(props: {
	pattern: string;
	reRunOnModuleChange?: ImportMeta;
}): void {
	if (!import.meta.env.DEV) {
		return;
	}
	if (!props.reRunOnModuleChange) {
		return;
	}
	registerClientLoaderModuleForHMRRerun({
		importMeta: props.reRunOnModuleChange,
		pattern: props.pattern,
	});
}

export function resolveMatchedPatternsToRefreshForHMRUpdate(props: {
	updates: Array<ViteHMRUpdate>;
	currentMatchedPatterns: string[];
}): string[] {
	const currentMatchedPatternsSet = new Set(props.currentMatchedPatterns);
	if (currentMatchedPatternsSet.size === 0) {
		return [];
	}
	const matchedPatternsToRefresh = new Set<string>();
	for (const update of props.updates) {
		if (update.type !== "js-update") {
			continue;
		}
		const trackedPatternsForUpdatePath =
			resolveTrackedPatternsForUpdatePath(update.path);
		if (!trackedPatternsForUpdatePath) {
			continue;
		}
		for (const pattern of trackedPatternsForUpdatePath) {
			if (currentMatchedPatternsSet.has(pattern)) {
				matchedPatternsToRefresh.add(pattern);
			}
		}
	}
	return Array.from(matchedPatternsToRefresh);
}

export function applyViteAfterUpdatePayload(props: {
	updates: Array<ViteHMRUpdate>;
	currentImportURLs: string[];
	triggerRevalidate: () => void;
}): void {
	const runtimeSnapshot =
		resolveRuntimeGlobalForHMRRefresh()?.runtimeRouteSnapshot;
	const matchedPatternsToRefresh =
		resolveMatchedPatternsToRefreshForHMRUpdate({
			updates: props.updates,
			currentMatchedPatterns: runtimeSnapshot?.matchedPatterns ?? [],
		});
	if (matchedPatternsToRefresh.length === 0) {
		return;
	}
	void refreshCurrentClientLoadersAfterHMRUpdate({
		matchedPatternsToRefresh,
	}).catch(() => {
		props.triggerRevalidate();
	});
}

export function registerViteAfterUpdateListenerIfNeeded(props: {
	hotRuntime: ImportMetaHot | undefined;
	getCurrentImportURLs: () => string[];
	triggerRevalidate: () => void;
}): void {
	if (!import.meta.env.DEV) {
		return;
	}
	if (!props.hotRuntime) {
		return;
	}
	if (registeredViteHotRuntimes.has(props.hotRuntime)) {
		return;
	}
	props.hotRuntime.on("vite:afterUpdate", ({ updates }) => {
		applyViteAfterUpdatePayload({
			updates,
			currentImportURLs: props.getCurrentImportURLs(),
			triggerRevalidate: props.triggerRevalidate,
		});
	});
	registeredViteHotRuntimes.add(props.hotRuntime);
}

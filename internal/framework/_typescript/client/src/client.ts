/// <reference types="vite/client" />

import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches, type Match } from "vorma/kit/matcher/find-nested";
import { getIsGETRequest } from "vorma/kit/url";
import { AssetManager } from "./asset_manager.ts";
import {
	completeClientLoaders,
	findPartialMatchesOnClient,
	setClientLoadersState,
	type ClientLoadersResult,
} from "./client_loaders.ts";
import {
	dispatchBuildIDEvent,
	dispatchStatusEvent,
	type StatusEventDetail,
} from "./events.ts";
import { HistoryManager } from "./history/history.ts";
import type { historyInstance } from "./history/npm_history_types.ts";
import {
	effectuateRedirectDataResult,
	getBuildIDFromResponse,
	handleRedirects,
	type RedirectData,
} from "./redirects/redirects.ts";
import { __reRenderApp } from "./rendering.ts";
import {
	__applyScrollState,
	type ScrollState,
} from "./scroll_state_manager.ts";
import { isAbortError } from "./utils/errors.ts";
import { logError } from "./utils/logging.ts";
import {
	__vormaClientGlobal,
	type ClientLoaderAwaitedServerData,
	type GetRouteDataOutput,
} from "./vorma_ctx/vorma_ctx.ts";

/////////////////////////////////////////////////////////////////////
// TYPES
/////////////////////////////////////////////////////////////////////

export type VormaNavigationType =
	| "browserHistory"
	| "userNavigation"
	| "revalidation"
	| "redirect"
	| "prefetch"
	| "action";

export type NavigateProps = {
	href: string;
	state?: unknown;
	navigationType: VormaNavigationType;
	scrollStateToRestore?: ScrollState;
	replace?: boolean;
	redirectCount?: number;
	scrollToTop?: boolean;
};

// Discriminated union for navigation outcomes -- provides exhaustiveness checking
export type NavigationOutcome =
	| { type: "aborted" }
	| { type: "redirect"; redirectData: RedirectData; props: NavigateProps }
	| {
			type: "success";
			response: Response;
			json: GetRouteDataOutput;
			cssBundlePromises: Array<Promise<any>>;
			waitFnPromise: Promise<ClientLoadersResult> | undefined;
			props: NavigateProps;
	  };

export type NavigationControl = {
	abortController: AbortController | undefined;
	promise: Promise<NavigationOutcome>;
};

/////////////////////////////////////////////////////////////////////
// NAVIGATION STATE MANAGER
/////////////////////////////////////////////////////////////////////

// Navigation phases represent the lifecycle stages
type NavigationPhase =
	| "fetching" // Fetching route data
	| "waiting" // Waiting for assets/loaders
	| "rendering" // Applying changes to DOM
	| "complete"; // Navigation finished

// Navigation intent represents what should happen when complete
type NavigationIntent =
	| "none" // Prefetch -- don't navigate unless upgraded
	| "navigate" // Normal navigation -- update URL and render
	| "revalidate"; // Revalidation -- only update if still on same page

interface NavigationEntry {
	control: NavigationControl;
	type: VormaNavigationType;
	intent: NavigationIntent;
	phase: NavigationPhase;
	startTime: number;
	targetUrl: string; // URL this navigation is targeting
	originUrl: string; // URL when navigation started (for revalidation)
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
}

interface SubmissionEntry {
	control: {
		abortController: AbortController | undefined;
		promise: Promise<any>;
	};
	startTime: number;
	skipGlobalLoadingIndicator?: boolean;
}

/////////////////////////////////////////////////////////////////////
// canSkipServerFetch HELPERS - extracted for readability
/////////////////////////////////////////////////////////////////////

type SkipCheckContext = {
	routeManifest: Record<string, number>;
	patternRegistry: any;
	patternToWaitFnMap: Record<string, any>;
	clientModuleMap: Record<
		string,
		{ importURL: string; exportKey: string; errorExportKey: string }
	>;
	currentMatchedPatterns: string[];
	currentParams: Record<string, string>;
	currentSplatValues: string[];
	currentLoadersData: any[];
	url: URL;
	matchResult: any;
};

type SkipCheckResult =
	| { canSkip: false }
	| {
			canSkip: true;
			matchResult: any;
			importURLs: string[];
			exportKeys: string[];
			loadersData: any[];
	  };

function hasServerLoaderRemoval(ctx: SkipCheckContext): boolean {
	for (const pattern of ctx.currentMatchedPatterns) {
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (hasServerLoader) {
			const stillMatched = ctx.matchResult.matches.some(
				(m: Match) => m.registeredPattern.originalPattern === pattern,
			);
			if (!stillMatched) {
				return true;
			}
		}
	}
	return false;
}

function hasNewClientLoader(ctx: SkipCheckContext): boolean {
	for (const m of ctx.matchResult.matches) {
		const pattern = m.registeredPattern.originalPattern;
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];
		const wasAlreadyMatched = ctx.currentMatchedPatterns.includes(pattern);
		if (hasClientLoader && !wasAlreadyMatched) {
			return true;
		}
	}
	return false;
}

function findOutermostLoaderIndex(ctx: SkipCheckContext): number {
	for (let i = ctx.matchResult.matches.length - 1; i >= 0; i--) {
		const match: Match | undefined = ctx.matchResult.matches[i];
		if (!match) continue;

		const pattern = match.registeredPattern.originalPattern;
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];

		if (hasServerLoader || hasClientLoader) {
			return i;
		}
	}
	return -1;
}

function didSearchParamsChange(ctx: SkipCheckContext): boolean {
	const currentUrlObj = new URL(window.location.href);
	const currentParamsSorted = Array.from(
		currentUrlObj.searchParams.entries(),
	).sort();
	const targetParamsSorted = Array.from(
		ctx.url.searchParams.entries(),
	).sort();
	return !jsonDeepEquals(currentParamsSorted, targetParamsSorted);
}

function didOutermostParamsChange(
	ctx: SkipCheckContext,
	outermostLoaderIndex: number,
): boolean {
	const outermostMatch = ctx.matchResult.matches[outermostLoaderIndex];
	if (!outermostMatch) return false;

	for (const seg of outermostMatch.registeredPattern.normalizedSegments) {
		if (seg.segType === "dynamic") {
			const paramName = seg.normalizedVal.substring(1);
			if (
				ctx.matchResult.params[paramName] !==
				ctx.currentParams[paramName]
			) {
				return true;
			}
		}
	}

	const hasSplat = outermostMatch.registeredPattern.lastSegType === "splat";
	if (hasSplat) {
		if (
			!jsonDeepEquals(ctx.matchResult.splatValues, ctx.currentSplatValues)
		) {
			return true;
		}
	}

	return false;
}

function buildSkipResult(ctx: SkipCheckContext): SkipCheckResult {
	const importURLs: string[] = [];
	const exportKeys: string[] = [];
	const loadersData: any[] = [];

	for (let i = 0; i < ctx.matchResult.matches.length; i++) {
		const match: Match | undefined = ctx.matchResult.matches[i];
		if (!match) continue;

		const pattern = match.registeredPattern.originalPattern;
		const moduleInfo = ctx.clientModuleMap[pattern];
		if (!moduleInfo) {
			return { canSkip: false };
		}

		importURLs.push(moduleInfo.importURL);
		exportKeys.push(moduleInfo.exportKey);

		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (!hasServerLoader) {
			loadersData.push(undefined);
		} else {
			const currentPatternIndex =
				ctx.currentMatchedPatterns.indexOf(pattern);
			if (currentPatternIndex === -1) {
				return { canSkip: false };
			}
			loadersData.push(ctx.currentLoadersData[currentPatternIndex]);
		}
	}

	return {
		canSkip: true,
		matchResult: ctx.matchResult,
		importURLs,
		exportKeys,
		loadersData,
	};
}

/////////////////////////////////////////////////////////////////////
// NAVIGATION STATE MANAGER CLASS
/////////////////////////////////////////////////////////////////////

class NavigationStateManager {
	// Single slot for active user/browser/redirect navigation
	private _activeNavigation: NavigationEntry | null = null;
	// Separate cache for prefetches (can have multiple to different URLs)
	private _prefetchCache = new Map<string, NavigationEntry>();
	// Single slot for pending revalidation (at most one, coalesced)
	private _pendingRevalidation: NavigationEntry | null = null;
	// Submissions tracked separately
	private _submissions = new Map<string | symbol, SubmissionEntry>();

	private lastDispatchedStatus: StatusEventDetail | null = null;
	private dispatchStatusEventDebounced: () => void;
	private readonly REVALIDATION_COALESCE_MS = 8;

	constructor() {
		this.dispatchStatusEventDebounced = debounce(() => {
			this.dispatchStatusEvent();
		}, 8);
	}

	async navigate(props: NavigateProps): Promise<{ didNavigate: boolean }> {
		const control = this.beginNavigation(props);

		try {
			const outcome = await control.promise;

			// Handle based on outcome type (discriminated union)
			switch (outcome.type) {
				case "aborted":
					return { didNavigate: false };

				case "redirect": {
					const targetUrl = new URL(props.href, window.location.href)
						.href;
					const entry = this.findNavigationEntry(targetUrl);
					if (!entry) {
						return { didNavigate: false };
					}

					// Skip redirect effectuation for pure prefetches
					if (entry.type === "prefetch" && entry.intent === "none") {
						this.deleteNavigation(targetUrl);
						return { didNavigate: false };
					}

					this.deleteNavigation(targetUrl);
					await effectuateRedirectDataResult(
						outcome.redirectData,
						props.redirectCount || 0,
						props,
					);
					return { didNavigate: false };
				}

				case "success": {
					const targetUrl = new URL(props.href, window.location.href)
						.href;
					const entry = this.findNavigationEntry(targetUrl);
					if (!entry) {
						return { didNavigate: false };
					}

					if (
						entry.intent === "navigate" ||
						entry.intent === "revalidate"
					) {
						lastTriggeredNavOrRevalidateTimestampMS = Date.now();
					}

					await this.processSuccessfulNavigation(outcome, entry);

					if (entry.intent === "none" && entry.type === "prefetch") {
						return { didNavigate: false };
					}

					return { didNavigate: true };
				}

				default: {
					// Exhaustiveness check - TypeScript will error if a case is missing
					const _exhaustive: never = outcome;
					throw new Error(
						`Unexpected navigation outcome type: ${(_exhaustive as any).type}`,
					);
				}
			}
		} catch (error) {
			const targetUrl = new URL(props.href, window.location.href).href;
			this.deleteNavigation(targetUrl);
			if (!isAbortError(error)) {
				logError("Navigate error:", error);
			}
			return { didNavigate: false };
		}
	}

	beginNavigation(props: NavigateProps): NavigationControl {
		const targetUrl = new URL(props.href, window.location.href).href;

		switch (props.navigationType) {
			case "userNavigation":
				return this.beginUserNavigation(props, targetUrl);
			case "prefetch":
				return this.beginPrefetch(props, targetUrl);
			case "revalidation":
				return this.beginRevalidation(props);
			case "browserHistory":
			case "redirect":
			default:
				return this.createActiveNavigation(props, "navigate");
		}
	}

	private beginUserNavigation(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		// Abort active navigation if it's to a different URL
		if (
			this._activeNavigation &&
			this._activeNavigation.targetUrl !== targetUrl
		) {
			this._activeNavigation.control.abortController?.abort();
			this._activeNavigation = null;
		}

		// Abort all prefetches except the one we might upgrade
		for (const [url, prefetch] of this._prefetchCache.entries()) {
			if (url !== targetUrl) {
				prefetch.control.abortController?.abort();
				this._prefetchCache.delete(url);
			}
		}

		// Abort pending revalidation only if it's to a different URL
		if (
			this._pendingRevalidation &&
			this._pendingRevalidation.targetUrl !== targetUrl
		) {
			this._pendingRevalidation.control.abortController?.abort();
			this._pendingRevalidation = null;
		}

		// Check if there's already an active navigation to this URL
		if (this._activeNavigation?.targetUrl === targetUrl) {
			return this._activeNavigation.control;
		}

		// Check if there's a prefetch to upgrade
		const existingPrefetch = this._prefetchCache.get(targetUrl);
		if (existingPrefetch) {
			// Upgrade prefetch: move from cache to active slot, change intent
			this._prefetchCache.delete(targetUrl);
			existingPrefetch.type = "userNavigation";
			existingPrefetch.intent = "navigate";
			existingPrefetch.scrollToTop = props.scrollToTop;
			existingPrefetch.replace = props.replace;
			existingPrefetch.state = props.state;
			this._activeNavigation = existingPrefetch;
			this.scheduleStatusUpdate();
			return existingPrefetch.control;
		}

		// Check if there's a pending revalidation to the same URL - upgrade it
		if (this._pendingRevalidation?.targetUrl === targetUrl) {
			// Upgrade revalidation: change intent so user gets proper link semantics
			this._pendingRevalidation.type = "userNavigation";
			this._pendingRevalidation.intent = "navigate";
			this._pendingRevalidation.scrollToTop = props.scrollToTop;
			this._pendingRevalidation.replace = props.replace;
			this._pendingRevalidation.state = props.state;
			return this._pendingRevalidation.control;
		}

		return this.createActiveNavigation(props, "navigate");
	}

	private beginPrefetch(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		// If there's already an active navigation to this URL, return its control
		if (this._activeNavigation?.targetUrl === targetUrl) {
			return this._activeNavigation.control;
		}

		// If there's already a prefetch to this URL, return its control
		const existingPrefetch = this._prefetchCache.get(targetUrl);
		if (existingPrefetch) {
			return existingPrefetch.control;
		}

		// If there's a pending revalidation to this URL, return its control
		if (this._pendingRevalidation?.targetUrl === targetUrl) {
			return this._pendingRevalidation.control;
		}

		// Don't prefetch current page
		const currentUrl = new URL(window.location.href);
		const targetUrlObj = new URL(targetUrl);
		currentUrl.hash = "";
		targetUrlObj.hash = "";
		if (currentUrl.href === targetUrlObj.href) {
			return {
				abortController: new AbortController(),
				promise: Promise.resolve({ type: "aborted" as const }),
			};
		}

		return this.createPrefetch(props, targetUrl);
	}

	private beginRevalidation(props: NavigateProps): NavigationControl {
		const currentUrl = window.location.href;

		// Coalesce recent revalidations
		if (
			this._pendingRevalidation &&
			Date.now() - this._pendingRevalidation.startTime <
				this.REVALIDATION_COALESCE_MS
		) {
			return this._pendingRevalidation.control;
		}

		// Abort existing revalidation
		if (this._pendingRevalidation) {
			this._pendingRevalidation.control.abortController?.abort();
			this._pendingRevalidation = null;
		}

		return this.createRevalidation({ ...props, href: currentUrl });
	}

	private createActiveNavigation(
		props: NavigateProps,
		intent: NavigationIntent,
	): NavigationControl {
		const controller = new AbortController();
		const targetUrl = new URL(props.href, window.location.href).href;

		const entry: NavigationEntry = {
			control: {
				abortController: controller,
				promise: this.fetchRouteData(controller, props).catch(
					(error) => {
						this.deleteNavigation(targetUrl);
						throw error;
					},
				),
			},
			type: props.navigationType,
			intent,
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
			scrollToTop: props.scrollToTop,
			replace: props.replace,
			state: props.state,
		};

		this._activeNavigation = entry;
		this.scheduleStatusUpdate();
		return entry.control;
	}

	private createPrefetch(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		const controller = new AbortController();

		const entry: NavigationEntry = {
			control: {
				abortController: controller,
				promise: this.fetchRouteData(controller, props).catch(
					(error) => {
						this._prefetchCache.delete(targetUrl);
						throw error;
					},
				),
			},
			type: "prefetch",
			intent: "none",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
			scrollToTop: props.scrollToTop,
			replace: props.replace,
			state: props.state,
		};

		this._prefetchCache.set(targetUrl, entry);
		// No status update needed - prefetches don't affect status
		return entry.control;
	}

	private createRevalidation(props: NavigateProps): NavigationControl {
		const controller = new AbortController();
		const targetUrl = new URL(props.href, window.location.href).href;

		const entry: NavigationEntry = {
			control: {
				abortController: controller,
				promise: this.fetchRouteData(controller, props).catch(
					(error) => {
						if (
							this._pendingRevalidation?.targetUrl === targetUrl
						) {
							this._pendingRevalidation = null;
							this.scheduleStatusUpdate();
						}
						throw error;
					},
				),
			},
			type: "revalidation",
			intent: "revalidate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
			scrollToTop: props.scrollToTop,
			replace: props.replace,
			state: props.state,
		};

		this._pendingRevalidation = entry;
		this.scheduleStatusUpdate();
		return entry.control;
	}

	private transitionPhase(targetUrl: string, phase: NavigationPhase): void {
		if (this._activeNavigation?.targetUrl === targetUrl) {
			this._activeNavigation.phase = phase;
			this.scheduleStatusUpdate();
			return;
		}

		const prefetch = this._prefetchCache.get(targetUrl);
		if (prefetch) {
			prefetch.phase = phase;
			// No status update for prefetches
			return;
		}

		if (this._pendingRevalidation?.targetUrl === targetUrl) {
			this._pendingRevalidation.phase = phase;
			this.scheduleStatusUpdate();
		}
	}

	private canSkipServerFetch(targetUrl: string): SkipCheckResult {
		// Early return: no route manifest
		const routeManifest = __vormaClientGlobal.get("routeManifest");
		if (!routeManifest) {
			return { canSkip: false };
		}

		// Early return: no pattern registry
		const patternRegistry = __vormaClientGlobal.get("patternRegistry");
		if (!patternRegistry) {
			return { canSkip: false };
		}

		// Early return: no match
		const url = new URL(targetUrl);
		const matchResult = findNestedMatches(patternRegistry, url.pathname);
		if (!matchResult) {
			return { canSkip: false };
		}

		// Build context for helper functions
		const ctx: SkipCheckContext = {
			routeManifest,
			patternRegistry,
			patternToWaitFnMap:
				__vormaClientGlobal.get("patternToWaitFnMap") || {},
			clientModuleMap: __vormaClientGlobal.get("clientModuleMap") || {},
			currentMatchedPatterns:
				__vormaClientGlobal.get("matchedPatterns") || [],
			currentParams: __vormaClientGlobal.get("params") || {},
			currentSplatValues: __vormaClientGlobal.get("splatValues") || [],
			currentLoadersData: __vormaClientGlobal.get("loadersData") || [],
			url,
			matchResult,
		};

		// Early return: server loader being removed
		if (hasServerLoaderRemoval(ctx)) {
			return { canSkip: false };
		}

		// Early return: new client loader introduced
		if (hasNewClientLoader(ctx)) {
			return { canSkip: false };
		}

		// Find outermost loader index for param/search change checks
		const outermostLoaderIndex = findOutermostLoaderIndex(ctx);

		// Early return: search params changed with loaders present
		if (outermostLoaderIndex !== -1 && didSearchParamsChange(ctx)) {
			return { canSkip: false };
		}

		// Early return: outermost loader params changed
		if (
			outermostLoaderIndex !== -1 &&
			didOutermostParamsChange(ctx, outermostLoaderIndex)
		) {
			return { canSkip: false };
		}

		// Build and return skip result
		return buildSkipResult(ctx);
	}

	private async fetchRouteData(
		controller: AbortController,
		props: NavigateProps,
	): Promise<NavigationOutcome> {
		try {
			const url = new URL(props.href, window.location.href);

			// Check if we can skip the server fetch (not for revalidations)
			if (
				props.navigationType !== "revalidation" &&
				props.navigationType !== "action"
			) {
				const skipCheck = this.canSkipServerFetch(url.href);

				if (skipCheck.canSkip) {
					return this.buildClientOnlyOutcome(
						skipCheck,
						props,
						controller,
					);
				}
			}

			url.searchParams.set(
				"vorma_json",
				__vormaClientGlobal.get("buildID") || "1",
			);

			if (props.navigationType === "revalidation") {
				const deploymentID = __vormaClientGlobal.get("deploymentID");
				if (deploymentID) {
					url.searchParams.set("dpl", deploymentID);
				}
			}

			// Start server fetch and immediately process the response to JSON
			const serverPromise = handleRedirects({
				abortController: controller,
				url,
				isPrefetch: props.navigationType === "prefetch",
				redirectCount: props.redirectCount,
			}).then(async (result) => {
				if (
					result.response &&
					result.response.ok &&
					!result.redirectData?.status
				) {
					const json = await result.response.json();
					return { ...result, json };
				}
				return { ...result, json: undefined };
			});

			// Try to match routes on the client and start parallel loaders
			const pathname = url.pathname;
			const matchResult = await findPartialMatchesOnClient(pathname);
			const patternToWaitFnMap =
				__vormaClientGlobal.get("patternToWaitFnMap");
			const runningLoaders = new Map<string, Promise<any>>();

			// Start client loaders for already-registered patterns
			if (matchResult) {
				const { params, splatValues, matches } = matchResult;

				for (let i = 0; i < matches.length; i++) {
					const match = matches[i];
					if (!match) continue;

					const pattern = match.registeredPattern.originalPattern;
					const loaderFn = patternToWaitFnMap[pattern];

					if (loaderFn) {
						const serverDataPromise = serverPromise
							.then(
								({
									response,
									json,
								}): ClientLoaderAwaitedServerData<any, any> => {
									if (!response || !response.ok || !json) {
										return {
											matchedPatterns: [],
											loaderData: undefined,
											rootData: null,
											buildID: "1",
										};
									}
									const serverIdx =
										json.matchedPatterns?.indexOf(pattern);
									const loaderData =
										serverIdx !== -1 &&
										serverIdx !== undefined
											? json.loadersData[serverIdx]
											: undefined;
									const rootData = json.hasRootData
										? json.loadersData[0]
										: null;
									const buildID =
										getBuildIDFromResponse(response) || "1";
									return {
										matchedPatterns:
											json.matchedPatterns || [],
										loaderData,
										rootData,
										buildID,
									};
								},
							)
							.catch(() => ({
								matchedPatterns: [],
								loaderData: undefined,
								rootData: null,
								buildID: "1",
							}));

						const loaderPromise = loaderFn({
							params,
							splatValues,
							serverDataPromise,
							signal: controller.signal,
						});

						runningLoaders.set(pattern, loaderPromise);
					}
				}
			}

			// Wait for server response
			const { redirectData, response, json } = await serverPromise;

			const redirected = redirectData?.status === "did";
			const responseNotOK = !response?.ok && response?.status !== 304;

			if (redirected || !response) {
				controller.abort();
				return { type: "aborted" };
			}

			if (responseNotOK) {
				controller.abort();
				throw new Error(`Fetch failed with status ${response.status}`);
			}

			if (redirectData?.status === "should") {
				controller.abort();
				return { type: "redirect", redirectData, props };
			}

			if (!json) {
				controller.abort();
				throw new Error("No JSON response");
			}

			// deps are only present in prod because they stem from the rollup metafile
			const depsToPreload = import.meta.env.DEV
				? [...new Set(json.importURLs)]
				: json.deps;
			for (const dep of depsToPreload ?? []) {
				if (dep) AssetManager.preloadModule(dep);
			}

			const buildID = getBuildIDFromResponse(response);

			// Complete client loader execution
			const waitFnPromise = completeClientLoaders(
				json,
				buildID,
				runningLoaders,
				controller.signal,
			);

			const cssBundlePromises: Array<Promise<any>> = [];
			for (const bundle of json.cssBundles ?? []) {
				cssBundlePromises.push(AssetManager.preloadCSS(bundle));
			}

			return {
				type: "success",
				response,
				json,
				props,
				cssBundlePromises,
				waitFnPromise,
			};
		} catch (error) {
			if (!isAbortError(error)) {
				logError("Navigation failed", error);
			}
			throw error;
		}
	}

	private buildClientOnlyOutcome(
		skipCheck: Extract<SkipCheckResult, { canSkip: true }>,
		props: NavigateProps,
		controller: AbortController,
	): NavigationOutcome {
		const { matchResult, importURLs, exportKeys, loadersData } = skipCheck;

		const json: GetRouteDataOutput = {
			matchedPatterns: matchResult.matches.map(
				(m: Match) => m.registeredPattern.originalPattern,
			),
			loadersData: loadersData,
			importURLs: importURLs,
			exportKeys: exportKeys,
			hasRootData: __vormaClientGlobal.get("hasRootData"),
			params: matchResult.params,
			splatValues: matchResult.splatValues,
			deps: [],
			cssBundles: [],
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			errorExportKeys: [],
			title: undefined,
			metaHeadEls: undefined,
			restHeadEls: undefined,
			activeComponents: undefined as unknown as [],
		};

		const response = new Response(JSON.stringify(json), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": __vormaClientGlobal.get("buildID") || "1",
			},
		});

		const currentClientLoadersData =
			__vormaClientGlobal.get("clientLoadersData") || [];
		const patternToWaitFnMap =
			__vormaClientGlobal.get("patternToWaitFnMap") || {};
		const runningLoaders = new Map<string, Promise<any>>();

		for (let i = 0; i < json.matchedPatterns.length; i++) {
			const pattern = json.matchedPatterns[i];
			if (!pattern) continue;

			if (patternToWaitFnMap[pattern]) {
				const currentMatchedPatterns =
					__vormaClientGlobal.get("matchedPatterns") || [];
				const currentPatternIndex =
					currentMatchedPatterns.indexOf(pattern);

				if (
					currentPatternIndex !== -1 &&
					currentClientLoadersData[currentPatternIndex] !== undefined
				) {
					runningLoaders.set(
						pattern,
						Promise.resolve(
							currentClientLoadersData[currentPatternIndex],
						),
					);
				}
			}
		}

		const waitFnPromise = completeClientLoaders(
			json,
			__vormaClientGlobal.get("buildID") || "1",
			runningLoaders,
			controller.signal,
		);

		return {
			type: "success",
			response,
			props,
			json,
			cssBundlePromises: [],
			waitFnPromise,
		};
	}

	async processSuccessfulNavigation(
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> {
		try {
			const { response, json, props, cssBundlePromises, waitFnPromise } =
				outcome;

			// Only update module map and apply CSS if build IDs match
			const currentBuildID = __vormaClientGlobal.get("buildID");
			const responseBuildID = getBuildIDFromResponse(response);

			if (responseBuildID === currentBuildID) {
				// Update module map only when builds match
				const clientModuleMap =
					__vormaClientGlobal.get("clientModuleMap") || {};
				const matchedPatterns = json.matchedPatterns || [];
				const importURLs = json.importURLs || [];
				const exportKeys = json.exportKeys || [];
				const errorExportKeys = json.errorExportKeys || [];

				for (let i = 0; i < matchedPatterns.length; i++) {
					const pattern = matchedPatterns[i];
					const importURL = importURLs[i];
					const exportKey = exportKeys[i];
					const errorExportKey = errorExportKeys[i];

					if (pattern && importURL) {
						clientModuleMap[pattern] = {
							importURL,
							exportKey: exportKey || "default",
							errorExportKey: errorExportKey || "",
						};
					}
				}

				__vormaClientGlobal.set("clientModuleMap", clientModuleMap);

				// Apply CSS bundles immediately, even for prefetches
				if (json.cssBundles && json.cssBundles.length > 0) {
					AssetManager.applyCSS(json.cssBundles);
				}
			}

			// Validate revalidation is still applicable
			if (entry.type === "revalidation") {
				const currentUrl = window.location.href;
				if (currentUrl !== entry.originUrl) {
					this.deleteNavigation(entry.targetUrl);
					return;
				}
			}

			// Transition to waiting phase
			this.transitionPhase(entry.targetUrl, "waiting");

			// Skip if navigation was aborted
			if (!this.findNavigationEntry(entry.targetUrl)) {
				return;
			}

			// Update build ID if needed
			const oldID = __vormaClientGlobal.get("buildID");
			const newID = getBuildIDFromResponse(response);
			if (newID && newID !== oldID) {
				dispatchBuildIDEvent({ newID, oldID });
			}

			// Wait for client loaders and set state
			const clientLoadersResult = await waitFnPromise;
			setClientLoadersState(clientLoadersResult);

			// Wait for CSS
			if (cssBundlePromises.length > 0) {
				try {
					await Promise.all(cssBundlePromises);
				} catch (error) {
					logError("Error preloading CSS bundles:", error);
				}
			}

			// Skip rendering for prefetch without intent
			if (entry.intent === "none") {
				this.transitionPhase(entry.targetUrl, "complete");
				return;
			}

			// Skip rendering for revalidation if not on target page
			if (
				entry.type === "revalidation" &&
				window.location.href !== entry.originUrl
			) {
				return;
			}

			// Transition to rendering phase
			this.transitionPhase(entry.targetUrl, "rendering");

			// Render the app
			try {
				await __reRenderApp({
					json,
					navigationType: entry.type,
					runHistoryOptions:
						entry.intent === "navigate"
							? {
									href: entry.targetUrl,
									scrollStateToRestore:
										props.scrollStateToRestore,
									replace: entry.replace || props.replace,
									scrollToTop: entry.scrollToTop,
									state: entry.state,
								}
							: undefined,
					onFinish: () => {
						this.transitionPhase(entry.targetUrl, "complete");
					},
				});
			} catch (error) {
				this.transitionPhase(entry.targetUrl, "complete");
				if (!isAbortError(error)) {
					logError("Error completing navigation", error);
				}
				throw error;
			}
		} finally {
			if (!(entry.type === "prefetch" && entry.intent === "none")) {
				this.deleteNavigation(entry.targetUrl);
			}
		}
	}

	async submit<T = any>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	): Promise<{ success: true; data: T } | { success: false; error: string }> {
		const abortController = new AbortController();
		const submissionKey = options?.dedupeKey
			? `submission:${options.dedupeKey}`
			: Symbol("submission");

		// Abort duplicate submission
		if (typeof submissionKey === "string") {
			const existing = this._submissions.get(submissionKey);
			if (existing) {
				existing.control.abortController?.abort("deduped");
			}
		}

		const entry: SubmissionEntry = {
			control: {
				abortController,
				promise: Promise.resolve() as any,
			},
			startTime: Date.now(),
			skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
		};

		this._submissions.set(submissionKey, entry);
		this.scheduleStatusUpdate();

		try {
			const urlToUse = new URL(url, window.location.href);
			const headers = new Headers(requestInit?.headers);
			const deploymentID = __vormaClientGlobal.get("deploymentID");
			if (deploymentID) {
				headers.set("x-deployment-id", deploymentID);
			}
			const finalRequestInit: RequestInit = {
				...requestInit,
				headers,
				signal: abortController.signal,
			};

			const { redirectData, response } = await handleRedirects({
				abortController,
				url: urlToUse,
				isPrefetch: false,
				redirectCount: 0,
				requestInit: finalRequestInit,
			});

			const oldID = __vormaClientGlobal.get("buildID");
			const newID = getBuildIDFromResponse(response);
			if (newID && newID !== oldID) {
				dispatchBuildIDEvent({ newID, oldID });
			}

			if (!response || !response.ok) {
				return {
					success: false,
					error: String(response?.status || "unknown"),
				};
			}

			if (redirectData?.status === "should") {
				await effectuateRedirectDataResult(redirectData, 0);
				return { success: true, data: undefined as T };
			}

			const data = await response.json();

			// Auto-revalidate for mutations
			const isGET = getIsGETRequest(requestInit);
			const redirected = redirectData?.status === "did";
			if (!isGET && !redirected && options?.revalidate !== false) {
				await revalidate();
			}

			return { success: true, data: data as T };
		} catch (error) {
			if (isAbortError(error)) {
				return { success: false, error: "Aborted" };
			}
			logError(error);
			return {
				success: false,
				error: error instanceof Error ? error.message : "Unknown error",
			};
		} finally {
			this._submissions.delete(submissionKey);
			this.scheduleStatusUpdate();
		}
	}

	private findNavigationEntry(
		targetUrl: string,
	): NavigationEntry | undefined {
		if (this._activeNavigation?.targetUrl === targetUrl) {
			return this._activeNavigation;
		}
		const prefetch = this._prefetchCache.get(targetUrl);
		if (prefetch) {
			return prefetch;
		}
		if (this._pendingRevalidation?.targetUrl === targetUrl) {
			return this._pendingRevalidation;
		}
		return undefined;
	}

	private deleteNavigation(key: string): boolean {
		// Check active navigation
		if (this._activeNavigation?.targetUrl === key) {
			this._activeNavigation = null;
			this.scheduleStatusUpdate();
			return true;
		}

		// Check prefetch cache
		if (this._prefetchCache.has(key)) {
			this._prefetchCache.delete(key);
			// No status update for prefetches
			return true;
		}

		// Check pending revalidation
		if (this._pendingRevalidation?.targetUrl === key) {
			this._pendingRevalidation = null;
			this.scheduleStatusUpdate();
			return true;
		}

		return false;
	}

	removeNavigation(key: string): void {
		const entry = this.findNavigationEntry(key);
		if (entry) {
			entry.control.abortController?.abort();
			this.deleteNavigation(key);
		}
	}

	getNavigation(key: string): NavigationEntry | undefined {
		return this.findNavigationEntry(key);
	}

	hasNavigation(key: string): boolean {
		return this.findNavigationEntry(key) !== undefined;
	}

	getNavigationsSize(): number {
		let size = 0;
		if (this._activeNavigation) size++;
		size += this._prefetchCache.size;
		if (this._pendingRevalidation) size++;
		return size;
	}

	getNavigations(): Map<string, NavigationEntry> {
		// Reconstruct Map for compatibility with existing code
		const map = new Map<string, NavigationEntry>();
		if (this._activeNavigation) {
			map.set(this._activeNavigation.targetUrl, this._activeNavigation);
		}
		for (const [key, entry] of this._prefetchCache) {
			map.set(key, entry);
		}
		if (this._pendingRevalidation) {
			map.set(
				this._pendingRevalidation.targetUrl,
				this._pendingRevalidation,
			);
		}
		return map;
	}

	getStatus(): StatusEventDetail {
		const isNavigating =
			this._activeNavigation !== null &&
			this._activeNavigation.intent === "navigate" &&
			this._activeNavigation.phase !== "complete";

		const isRevalidating =
			this._pendingRevalidation !== null &&
			this._pendingRevalidation.phase !== "complete";

		const isSubmitting = Array.from(this._submissions.values()).some(
			(x) => !x.skipGlobalLoadingIndicator,
		);

		return { isNavigating, isSubmitting, isRevalidating };
	}

	clearAll(): void {
		if (this._activeNavigation) {
			this._activeNavigation.control.abortController?.abort();
			this._activeNavigation = null;
		}

		for (const prefetch of this._prefetchCache.values()) {
			prefetch.control.abortController?.abort();
		}
		this._prefetchCache.clear();

		if (this._pendingRevalidation) {
			this._pendingRevalidation.control.abortController?.abort();
			this._pendingRevalidation = null;
		}

		for (const sub of this._submissions.values()) {
			sub.control.abortController?.abort();
		}
		this._submissions.clear();

		this.scheduleStatusUpdate();
	}

	private scheduleStatusUpdate(): void {
		this.dispatchStatusEventDebounced();
	}

	private dispatchStatusEvent(): void {
		const newStatus = this.getStatus();

		if (jsonDeepEquals(this.lastDispatchedStatus, newStatus)) {
			return;
		}
		this.lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}
}

// Global instance
export const navigationStateManager = new NavigationStateManager();

/////////////////////////////////////////////////////////////////////
// PUBLIC API
/////////////////////////////////////////////////////////////////////

export async function vormaNavigate(
	href: string,
	options?: {
		replace?: boolean;
		scrollToTop?: boolean;
		search?: string;
		hash?: string;
		state?: unknown;
	},
): Promise<void> {
	const url = new URL(href, window.location.href);

	if (options?.search !== undefined) {
		url.search = options.search;
	}
	if (options?.hash !== undefined) {
		url.hash = options.hash;
	}

	await navigationStateManager.navigate({
		href: url.href,
		navigationType: "userNavigation",
		replace: options?.replace,
		scrollToTop: options?.scrollToTop,
		state: options?.state,
	});
}

let lastTriggeredNavOrRevalidateTimestampMS = Date.now();

export function getLastTriggeredNavOrRevalidateTimestampMS(): number {
	return lastTriggeredNavOrRevalidateTimestampMS;
}

export async function revalidate() {
	await navigationStateManager.navigate({
		href: window.location.href,
		navigationType: "revalidation",
	});
}

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipGlobalLoadingIndicator?: boolean;
};

export async function submit<T = any>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<{ success: true; data: T } | { success: false; error: string }> {
	return navigationStateManager.submit(url, requestInit, options);
}

export function beginNavigation(props: NavigateProps): NavigationControl {
	return navigationStateManager.beginNavigation(props);
}

export function getStatus(): StatusEventDetail {
	return navigationStateManager.getStatus();
}

export function getLocation() {
	return {
		pathname: window.location.pathname,
		search: window.location.search,
		hash: window.location.hash,
		state: HistoryManager.getInstance().location.state,
	};
}

export function getBuildID(): string {
	return __vormaClientGlobal.get("buildID");
}

export function getRootEl(): HTMLDivElement {
	return document.getElementById("vorma-root") as HTMLDivElement;
}

export function getHistoryInstance(): historyInstance {
	return HistoryManager.getInstance();
}

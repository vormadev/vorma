import {
	getHrefDetails,
	resolveAbsoluteHref,
	type HrefDetails,
} from "vorma/kit/url";
import {
	__vormaClientGlobal,
	getNavigationStateAccess,
	setRuntimeBuildID,
} from "../app/context.ts";
import { dispatchBuildIDEvent } from "../platform/events.ts";
import { resolveRequestBodyForTransport } from "../platform/request_body.ts";
import { logError } from "../platform/safety.ts";
import {
	isSameDocumentLocation,
	VORMA_HARD_RELOAD_QUERY_PARAM,
} from "../platform/url.ts";
import type { NavigateProps, NavigationEntry } from "./navigation/types.ts";

export type RedirectData = { href: string; hrefDetails: HrefDetails } & (
	| {
			status: "did";
	  }
	| {
			status: "should";
			shouldRedirectStrategy: "hard" | "soft";
			latestBuildID: string;
	  }
);

type HTTPHrefDetails = Extract<HrefDetails, { isHTTP: true }>;
type ShouldRedirectData = Extract<RedirectData, { status: "should" }>;

type RedirectNavigationState = {
	getNavigations: () => Map<string, NavigationEntry>;
	removeNavigation: (targetUrl: string) => void;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

type RedirectRequestFlowInput = {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	redirectCount?: number;
};

type RedirectRequestFlowResult =
	| {
			kind: "too_many_redirects";
	  }
	| {
			kind: "ok";
			response: Response;
	  };

function resolveHTTPRedirectTarget(props: {
	href: string;
	source: string;
	baseHref?: string;
}): {
	hrefDetails: Extract<HrefDetails, { isHTTP: true }>;
} {
	const { href, source, baseHref } = props;
	let absoluteHref: string;
	try {
		absoluteHref = resolveAbsoluteHref({ href, baseHref });
	} catch {
		throw new Error(
			`${source} has invalid redirect target ${JSON.stringify(href)}`,
		);
	}

	const hrefDetails = getHrefDetails(absoluteHref);
	if (!hrefDetails.isHTTP) {
		throw new Error(
			`${source} redirect target ${JSON.stringify(href)} must be an HTTP(S) URL`,
		);
	}
	return { hrefDetails };
}

function getRedirectStrategy(
	hrefDetails: HTTPHrefDetails,
): ShouldRedirectData["shouldRedirectStrategy"] {
	return hrefDetails.isInternal ? "soft" : "hard";
}

function buildShouldRedirectData(props: {
	href: string;
	hrefDetails: HTTPHrefDetails;
	latestBuildID: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
}): ShouldRedirectData {
	return {
		hrefDetails: props.hrefDetails,
		status: "should",
		href: props.href,
		shouldRedirectStrategy:
			props.shouldRedirectStrategy ??
			getRedirectStrategy(props.hrefDetails),
		latestBuildID: props.latestBuildID,
	};
}

function buildShouldRedirectFromHref(props: {
	href: string;
	latestBuildID: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
	normalizeToAbsoluteHref?: boolean;
	baseHref?: string;
	source: string;
}): ShouldRedirectData {
	const resolvedTarget = resolveHTTPRedirectTarget({
		href: props.href,
		baseHref: props.baseHref,
		source: props.source,
	});

	const href = props.normalizeToAbsoluteHref
		? resolvedTarget.hrefDetails.absoluteURL
		: props.href;

	return buildShouldRedirectData({
		href,
		hrefDetails: resolvedTarget.hrefDetails,
		latestBuildID: props.latestBuildID,
		shouldRedirectStrategy: props.shouldRedirectStrategy,
	});
}

function parseBrowserRedirect(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	if (!response.redirected) {
		return null;
	}

	const shouldRedirectData = buildShouldRedirectFromHref({
		href: response.url,
		latestBuildID,
		normalizeToAbsoluteHref: true,
		source: "redirected fetch response URL",
	});
	const isCurrent = isSameDocumentLocation({
		targetHref: shouldRedirectData.href,
		currentHref: window.location.href,
	});
	if (isCurrent) {
		return {
			hrefDetails: shouldRedirectData.hrefDetails,
			status: "did",
			href: shouldRedirectData.href,
		};
	}
	return shouldRedirectData;
}

function parseHeaderRedirect(props: {
	response: Response;
	headerName: "X-Vorma-Reload" | "X-Client-Redirect";
	latestBuildID: string;
	baseHref: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
	normalizeToAbsoluteHref?: boolean;
	shortCircuitCurrentLocation?: boolean;
}): RedirectData | null {
	const headerValue = props.response.headers.get(props.headerName);
	if (!headerValue) {
		return null;
	}

	const shouldRedirectData = buildShouldRedirectFromHref({
		href: headerValue,
		latestBuildID: props.latestBuildID,
		baseHref: props.baseHref,
		shouldRedirectStrategy: props.shouldRedirectStrategy,
		normalizeToAbsoluteHref: props.normalizeToAbsoluteHref,
		source: props.headerName,
	});
	if (!props.shortCircuitCurrentLocation) {
		return shouldRedirectData;
	}

	const isCurrent = isSameDocumentLocation({
		targetHref: shouldRedirectData.href,
		currentHref: window.location.href,
	});
	if (isCurrent) {
		return toDidRedirectData(shouldRedirectData);
	}

	return shouldRedirectData;
}

function canIncludeBodyForMethod(method: string | undefined): boolean {
	const normalizedMethod = method?.toUpperCase();
	if (!normalizedMethod) {
		return false;
	}
	return normalizedMethod !== "GET" && normalizedMethod !== "HEAD";
}

/**
 * Builds request init for redirect-aware fetches.
 * It preserves caller options, injects client-redirect acceptance,
 * and JSON-serializes plain object bodies.
 */
export function buildRedirectRequestInit(
	requestInit: RequestInit | undefined,
	signal: AbortSignal,
): RequestInit {
	const {
		body: rawBody,
		headers: rawHeaders,
		signal: _requestSignal,
		...rest
	} = requestInit ?? {};

	const shouldAttachBody =
		rawBody !== undefined && canIncludeBodyForMethod(requestInit?.method);
	let shouldSetJSONContentType = false;
	const bodyParentObj: Pick<RequestInit, "body"> = {};
	if (shouldAttachBody) {
		const requestBodyResolution = resolveRequestBodyForTransport({
			input: rawBody,
		});
		bodyParentObj.body = requestBodyResolution.body;
		shouldSetJSONContentType = requestBodyResolution.didSerializeJSON;
	}

	const headers = new Headers(rawHeaders);
	if (shouldSetJSONContentType && !headers.has("content-type")) {
		headers.set("Content-Type", "application/json");
	}
	// To temporarily test traditional server redirect behavior,
	// you can set this to "0" instead of "1"
	headers.set("X-Accepts-Client-Redirect", "1");

	return {
		...rest,
		headers,
		signal,
		...bodyParentObj,
	};
}

async function executeRedirectRequestFlow(
	input: RedirectRequestFlowInput,
): Promise<RedirectRequestFlowResult> {
	const MAX_REDIRECTS = 10;
	const redirectCount = input.redirectCount || 0;

	if (redirectCount >= MAX_REDIRECTS) {
		logError("Too many redirects");
		return { kind: "too_many_redirects" };
	}

	const requestInit = buildRedirectRequestInit(
		input.requestInit,
		input.abortController.signal,
	);
	const response = await fetch(input.url, requestInit);

	return {
		kind: "ok",
		response,
	};
}

function toDidRedirectData(redirectData: ShouldRedirectData): RedirectData {
	return {
		hrefDetails: redirectData.hrefDetails,
		status: "did",
		href: redirectData.href,
	};
}

// cleanupRedirectRelatedNavigations aborts and removes redirect/revalidation lanes
// before applying a new redirect decision.
function cleanupRedirectRelatedNavigations(
	navigationState: RedirectNavigationState,
): void {
	const navEntries = navigationState.getNavigations().entries();
	for (const [targetUrl, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationState.removeNavigation(targetUrl);
		}
	}
}

// For internal targets we force a hard reload query marker so the server can
// correlate reload intent against the latest client-known build.
function effectuateHardRedirect(
	redirectData: ShouldRedirectData,
): RedirectData | null {
	if (!redirectData.hrefDetails.isHTTP) {
		return null;
	}

	const absoluteTargetHref = redirectData.hrefDetails.absoluteURL;
	if (redirectData.hrefDetails.isExternal) {
		window.location.href = absoluteTargetHref;
	} else {
		const url = new URL(absoluteTargetHref);
		url.searchParams.set(
			VORMA_HARD_RELOAD_QUERY_PARAM,
			redirectData.latestBuildID,
		);
		window.location.href = url.href;
	}

	return toDidRedirectData(redirectData);
}

async function effectuateSoftRedirect(
	navigationState: RedirectNavigationState,
	redirectData: ShouldRedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	const navigationResult = await navigationState.navigate({
		href: redirectData.href,
		navigationType: "redirect",
		redirectCount: redirectCount + 1,
		state: originalProps?.state,
		replace: originalProps?.replace,
		scrollToTop: originalProps?.scrollToTop,
	});
	if (!navigationResult.didNavigate) {
		return null;
	}

	return toDidRedirectData(redirectData);
}

// getBuildIDFromResponse extracts the build id header used for client/runtime sync.
export function getBuildIDFromResponse(response: Response | undefined): string {
	return response?.headers.get("X-Vorma-Build-Id") || "";
}

// syncBuildIDFromRedirectData updates the global build id when redirect metadata is newer.
export function syncBuildIDFromRedirectData(redirectData: RedirectData): void {
	if (redirectData.status !== "should") {
		return;
	}

	const oldID = __vormaClientGlobal.get("buildID");
	const newID = redirectData.latestBuildID;
	if (newID && newID !== oldID) {
		setRuntimeBuildID({
			buildID: newID,
		});
		dispatchBuildIDEvent({ newID, oldID });
	}
}

// effectuateRedirectDataResult applies redirect instructions and returns the final redirect status.
export async function effectuateRedirectDataResult(
	redirectData: RedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	if (redirectData.status !== "should") {
		return null;
	}

	const navigationState = getNavigationStateAccess();
	cleanupRedirectRelatedNavigations(navigationState);

	if (redirectData.shouldRedirectStrategy === "hard") {
		return effectuateHardRedirect(redirectData);
	}

	return effectuateSoftRedirect(
		navigationState,
		redirectData,
		redirectCount,
		originalProps,
	);
}

// handleRedirects executes fetch-with-redirect-detection and returns parsed redirect metadata.
export async function handleRedirects(props: {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	redirectCount?: number;
}): Promise<{ redirectData: RedirectData | null; response?: Response }> {
	const requestFlow = await executeRedirectRequestFlow(props);
	if (requestFlow.kind === "too_many_redirects") {
		return { redirectData: null, response: undefined };
	}

	const latestBuildID = getBuildIDFromResponse(requestFlow.response);
	const redirectBaseHref =
		requestFlow.response.url.trim().length > 0
			? requestFlow.response.url
			: props.url.href;
	const redirectData =
		parseHeaderRedirect({
			response: requestFlow.response,
			headerName: "X-Vorma-Reload",
			latestBuildID,
			baseHref: redirectBaseHref,
			shouldRedirectStrategy: "hard",
		}) ??
		parseBrowserRedirect(requestFlow.response, latestBuildID) ??
		parseHeaderRedirect({
			response: requestFlow.response,
			headerName: "X-Client-Redirect",
			latestBuildID,
			baseHref: redirectBaseHref,
			normalizeToAbsoluteHref: true,
			shortCircuitCurrentLocation: true,
		});
	return { redirectData, response: requestFlow.response };
}

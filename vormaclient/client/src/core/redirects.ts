import { getHrefDetails, type HrefDetails } from "vorma/kit/url";
import { dispatchBuildIDEvent } from "../platform/events.ts";
import { isSameDocumentLocation } from "../platform/url.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "../platform/url.ts";
import type { NavigateProps, NavigationEntry } from "./navigation/types.ts";
import { getNavigationStateAccess } from "../app/context.ts";
import {
	isArrayBufferView,
	isInstanceOfGlobal,
} from "../platform/safety.ts";
import { logError } from "../platform/safety.ts";
import { __vormaClientGlobal } from "../app/context.ts";

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
	removeNavigation: (key: string) => void;
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

function resolveHTTPRedirectTarget(href: string): {
	newURL: URL;
	hrefDetails: Extract<HrefDetails, { isHTTP: true }>;
} | null {
	const newURL = new URL(href, window.location.href);
	const hrefDetails = getHrefDetails(newURL.href);
	if (!hrefDetails.isHTTP) {
		return null;
	}
	return { newURL, hrefDetails };
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

function parseVormaReloadRedirect(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	const vormaReloadTarget = response.headers.get("X-Vorma-Reload");
	if (!vormaReloadTarget) {
		return null;
	}

	const resolvedTarget = resolveHTTPRedirectTarget(vormaReloadTarget);
	if (!resolvedTarget) {
		return null;
	}
	const { hrefDetails } = resolvedTarget;

	return buildShouldRedirectData({
		href: vormaReloadTarget,
		hrefDetails,
		latestBuildID,
		shouldRedirectStrategy: "hard",
	});
}

function parseBrowserRedirect(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	if (!response.redirected) {
		return null;
	}

	const resolvedTarget = resolveHTTPRedirectTarget(response.url);
	if (!resolvedTarget) {
		return null;
	}
	const { newURL, hrefDetails } = resolvedTarget;

	const isCurrent = isSameDocumentLocation(newURL.href, window.location.href);
	if (isCurrent) {
		return { hrefDetails, status: "did", href: newURL.href };
	}

	return buildShouldRedirectData({
		href: newURL.href,
		hrefDetails,
		latestBuildID,
	});
}

function parseClientRedirectHeader(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	const clientRedirectHeader = response.headers.get("X-Client-Redirect");
	if (!clientRedirectHeader) {
		return null;
	}

	const resolvedTarget = resolveHTTPRedirectTarget(clientRedirectHeader);
	if (!resolvedTarget) {
		return null;
	}
	const { hrefDetails } = resolvedTarget;

	return buildShouldRedirectData({
		href: hrefDetails.absoluteURL,
		hrefDetails,
		latestBuildID,
	});
}

function parseResponseForRedirectData(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	return (
		parseVormaReloadRedirect(response, latestBuildID) ??
		parseBrowserRedirect(response, latestBuildID) ??
		parseClientRedirectHeader(response, latestBuildID)
	);
}

function shouldSerializeBody(body: unknown): boolean {
	if (body === null || body === undefined) return false;
	if (typeof body === "string") return false;
	if (isInstanceOfGlobal(body, "FormData")) return false;
	if (isInstanceOfGlobal(body, "URLSearchParams")) return false;
	if (isInstanceOfGlobal(body, "Blob")) return false;
	if (isInstanceOfGlobal(body, "ArrayBuffer")) return false;
	if (isArrayBufferView(body)) {
		return false;
	}
	if (isInstanceOfGlobal(body, "ReadableStream")) return false;

	return true;
}

function canIncludeBodyForMethod(method: string | undefined): boolean {
	const normalizedMethod = method?.toUpperCase();
	if (!normalizedMethod) {
		return false;
	}
	return normalizedMethod !== "GET" && normalizedMethod !== "HEAD";
}

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

	const bodyParentObj: Pick<RequestInit, "body"> = {};
	if (rawBody !== undefined && canIncludeBodyForMethod(requestInit?.method)) {
		bodyParentObj.body = shouldSerializeBody(rawBody)
			? JSON.stringify(rawBody)
			: rawBody;
	}

	const headers = new Headers(rawHeaders);
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

function cleanupRedirectRelatedNavigations(
	navigationState: RedirectNavigationState,
): void {
	const navEntries = navigationState.getNavigations().entries();
	for (const [key, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationState.removeNavigation(key);
		}
	}
}

function effectuateHardRedirect(
	redirectData: ShouldRedirectData,
): RedirectData | null {
	if (!redirectData.hrefDetails.isHTTP) {
		return null;
	}

	if (redirectData.hrefDetails.isExternal) {
		window.location.href = redirectData.href;
	} else {
		const url = new URL(redirectData.href, window.location.href);
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
): Promise<RedirectData> {
	await navigationState.navigate({
		href: redirectData.href,
		navigationType: "redirect",
		redirectCount: redirectCount + 1,
		state: originalProps?.state,
		replace: originalProps?.replace,
		scrollToTop: originalProps?.scrollToTop,
	});

	return toDidRedirectData(redirectData);
}

export function getBuildIDFromResponse(response: Response | undefined): string {
	return response?.headers.get("X-Vorma-Build-Id") || "";
}

export function syncBuildIDFromRedirectData(redirectData: RedirectData): void {
	if (redirectData.status !== "should") {
		return;
	}

	const oldID = __vormaClientGlobal.get("buildID");
	const newID = redirectData.latestBuildID;
	if (newID && newID !== oldID) {
		__vormaClientGlobal.set("buildID", newID);
		dispatchBuildIDEvent({ newID, oldID });
	}
}

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

	if (redirectData.shouldRedirectStrategy === "soft") {
		return effectuateSoftRedirect(
			navigationState,
			redirectData,
			redirectCount,
			originalProps,
		);
	}

	return null;
}

export async function handleRedirects(props: {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	isPrefetch?: boolean;
	redirectCount?: number;
}): Promise<{ redirectData: RedirectData | null; response?: Response }> {
	const requestFlow = await executeRedirectRequestFlow(props);
	if (requestFlow.kind === "too_many_redirects") {
		return { redirectData: null, response: undefined };
	}

	const latestBuildID = getBuildIDFromResponse(requestFlow.response);
	const redirectData = parseResponseForRedirectData(
		requestFlow.response,
		latestBuildID,
	);
	return { redirectData, response: requestFlow.response };
}

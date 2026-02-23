import {
	getHrefDetails,
	resolveAbsoluteHref,
	type HrefDetails,
} from "vorma/kit/url";
import {
	__vormaClientGlobal,
	getNavigationStateAccess,
} from "../app/context.ts";
import { dispatchBuildIDEvent } from "../platform/events.ts";
import { logError } from "../platform/safety.ts";
import { resolveRequestBodyForTransport } from "../platform/request_body.ts";
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

export type RedirectEffectuationExecutionPlan =
	| {
			type: "stop";
			reason: "redirect_effectuation_redirect_data_not_should";
	  }
	| {
			type: "cleanup_and_effectuate_hard";
			reason: "redirect_effectuation_strategy_hard";
	  }
	| {
			type: "cleanup_and_effectuate_soft";
			reason: "redirect_effectuation_strategy_soft";
	  }
	| {
			type: "cleanup_and_stop";
			reason: "redirect_effectuation_strategy_unknown";
	  };

function resolveRedirectStrategyValue(
	redirectData: ShouldRedirectData,
): string {
	return (
		redirectData as ShouldRedirectData & { shouldRedirectStrategy: string }
	).shouldRedirectStrategy;
}

export function decideRedirectEffectuationExecutionPlan(props: {
	redirectData: RedirectData;
}): RedirectEffectuationExecutionPlan {
	const { redirectData } = props;
	if (redirectData.status !== "should") {
		return {
			type: "stop",
			reason: "redirect_effectuation_redirect_data_not_should",
		};
	}

	switch (resolveRedirectStrategyValue(redirectData)) {
		case "hard":
			return {
				type: "cleanup_and_effectuate_hard",
				reason: "redirect_effectuation_strategy_hard",
			};
		case "soft":
			return {
				type: "cleanup_and_effectuate_soft",
				reason: "redirect_effectuation_strategy_soft",
			};
		default:
			return {
				type: "cleanup_and_stop",
				reason: "redirect_effectuation_strategy_unknown",
			};
	}
}

export type RedirectEffectuationCommand =
	| {
			type: "cleanup_redirect_related_navigations";
			reason:
				| "redirect_effectuation_strategy_hard"
				| "redirect_effectuation_strategy_soft"
				| "redirect_effectuation_strategy_unknown";
	  }
	| {
			type: "effectuate_hard_redirect";
			redirectData: ShouldRedirectData;
			reason: "redirect_effectuation_strategy_hard";
	  }
	| {
			type: "effectuate_soft_redirect";
			redirectData: ShouldRedirectData;
			redirectCount: number;
			originalProps?: NavigateProps;
			reason: "redirect_effectuation_strategy_soft";
	  }
	| {
			type: "return_null";
			reason:
				| "redirect_effectuation_redirect_data_not_should"
				| "redirect_effectuation_strategy_unknown";
	  };

export function buildRedirectEffectuationCommands(props: {
	executionPlan: RedirectEffectuationExecutionPlan;
	redirectData: RedirectData;
	redirectCount: number;
	originalProps?: NavigateProps;
}): RedirectEffectuationCommand[] {
	const { executionPlan, redirectData, redirectCount, originalProps } = props;

	switch (executionPlan.type) {
		case "stop":
			return [
				{
					type: "return_null",
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_effectuate_hard":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "effectuate_hard_redirect",
					redirectData: redirectData as ShouldRedirectData,
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_effectuate_soft":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "effectuate_soft_redirect",
					redirectData: redirectData as ShouldRedirectData,
					redirectCount,
					originalProps,
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_stop":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "return_null",
					reason: executionPlan.reason,
				},
			];
	}
}

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

function resolveHTTPRedirectTarget(props: { href: string; source: string }): {
	hrefDetails: Extract<HrefDetails, { isHTTP: true }>;
} {
	const { href, source } = props;
	let absoluteHref: string;
	try {
		absoluteHref = resolveAbsoluteHref({ href: href });
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
	source: string;
}): ShouldRedirectData {
	const resolvedTarget = resolveHTTPRedirectTarget({
		href: props.href,
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

function parseVormaReloadRedirect(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	return parseHeaderRedirect({
		response,
		headerName: "X-Vorma-Reload",
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

function parseClientRedirectHeader(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	return parseHeaderRedirect({
		response,
		headerName: "X-Client-Redirect",
		latestBuildID,
		normalizeToAbsoluteHref: true,
	});
}

function parseHeaderRedirect(props: {
	response: Response;
	headerName: "X-Vorma-Reload" | "X-Client-Redirect";
	latestBuildID: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
	normalizeToAbsoluteHref?: boolean;
}): RedirectData | null {
	const headerValue = props.response.headers.get(props.headerName);
	if (!headerValue) {
		return null;
	}

	return buildShouldRedirectFromHref({
		href: headerValue,
		latestBuildID: props.latestBuildID,
		shouldRedirectStrategy: props.shouldRedirectStrategy,
		normalizeToAbsoluteHref: props.normalizeToAbsoluteHref,
		source: props.headerName,
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
	for (const [key, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationState.removeNavigation(key);
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

async function executeRedirectEffectuationCommands(props: {
	commands: RedirectEffectuationCommand[];
	navigationState: RedirectNavigationState;
}): Promise<RedirectData | null> {
	const { commands, navigationState } = props;

	for (const command of commands) {
		switch (command.type) {
			case "cleanup_redirect_related_navigations":
				cleanupRedirectRelatedNavigations(navigationState);
				break;
			case "effectuate_hard_redirect":
				return effectuateHardRedirect(command.redirectData);
			case "effectuate_soft_redirect":
				return effectuateSoftRedirect(
					navigationState,
					command.redirectData,
					command.redirectCount,
					command.originalProps,
				);
			case "return_null":
				return null;
		}
	}

	throw new Error(
		"Redirect effectuation command plan ended without a terminal command.",
	);
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
		__vormaClientGlobal.set("buildID", newID);
		dispatchBuildIDEvent({ newID, oldID });
	}
}

// effectuateRedirectDataResult applies redirect instructions and returns the final redirect status.
export async function effectuateRedirectDataResult(
	redirectData: RedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	const executionPlan = decideRedirectEffectuationExecutionPlan({
		redirectData,
	});

	if (executionPlan.type === "stop") {
		return null;
	}

	const commands = buildRedirectEffectuationCommands({
		executionPlan,
		redirectData,
		redirectCount,
		originalProps,
	});
	const navigationState = getNavigationStateAccess();

	return executeRedirectEffectuationCommands({
		commands,
		navigationState,
	});
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
	const redirectData = parseResponseForRedirectData(
		requestFlow.response,
		latestBuildID,
	);
	return { redirectData, response: requestFlow.response };
}

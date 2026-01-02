import {
	getHrefDetails,
	getIsGETRequest,
	type HrefDetails,
} from "vorma/kit/url";
import { navigationStateManager, type NavigateProps } from "../client.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "../hard_reload.ts";
import { logError, logInfo } from "../utils/logging.ts";

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

export function getBuildIDFromResponse(response: Response | undefined) {
	return response?.headers.get("X-Vorma-Build-Id") || "";
}

export function parseFetchResponseForRedirectData(
	reqInit: RequestInit,
	res: Response,
): RedirectData | null {
	const latestBuildID = getBuildIDFromResponse(res);

	const vormaReloadTarget = res.headers.get("X-Vorma-Reload");
	if (vormaReloadTarget) {
		const newURL = new URL(vormaReloadTarget, window.location.href);
		const hrefDetails = getHrefDetails(newURL.href);
		if (!hrefDetails.isHTTP) {
			return null;
		}

		return {
			hrefDetails,
			status: "should",
			href: vormaReloadTarget,
			shouldRedirectStrategy: "hard",
			latestBuildID,
		};
	}

	if (res.redirected) {
		const newURL = new URL(res.url, window.location.href);
		const hrefDetails = getHrefDetails(newURL.href);
		if (!hrefDetails.isHTTP) {
			return null;
		}

		const isCurrent = newURL.href === window.location.href;
		if (isCurrent) {
			return { hrefDetails, status: "did", href: newURL.href };
		}

		const wasGETRequest = getIsGETRequest(reqInit);
		if (!wasGETRequest) {
			logInfo("Not a GET request. No way to handle.");
			return null;
		}

		return {
			hrefDetails,
			status: "should",
			href: newURL.href,
			shouldRedirectStrategy: hrefDetails.isInternal ? "soft" : "hard",
			latestBuildID,
		};
	}

	const clientRedirectHeader = res.headers.get("X-Client-Redirect");

	if (!clientRedirectHeader) {
		return null;
	}

	const newURL = new URL(clientRedirectHeader, window.location.href);
	const hrefDetails = getHrefDetails(newURL.href);
	if (!hrefDetails.isHTTP) {
		return null;
	}

	return {
		hrefDetails,
		status: "should",
		href: hrefDetails.absoluteURL,
		shouldRedirectStrategy: hrefDetails.isInternal ? "soft" : "hard",
		latestBuildID,
	};
}

export async function effectuateRedirectDataResult(
	redirectData: RedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	if (redirectData.status !== "should") {
		return null;
	}

	// Clean up any active redirect or revalidations when redirecting.
	// Otherwise loading state will get stuck.
	const navEntries = navigationStateManager.getNavigations().entries();
	for (const [key, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationStateManager.removeNavigation(key);
		}
	}

	if (redirectData.shouldRedirectStrategy === "hard") {
		if (!redirectData.hrefDetails.isHTTP) return null;

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

		return {
			hrefDetails: redirectData.hrefDetails,
			status: "did",
			href: redirectData.href,
		};
	}

	if (redirectData.shouldRedirectStrategy === "soft") {
		await navigationStateManager.navigate({
			href: redirectData.href,
			navigationType: "redirect",
			redirectCount: redirectCount + 1,
			state: originalProps?.state,
			replace: originalProps?.replace,
			scrollToTop: originalProps?.scrollToTop,
		});

		return {
			hrefDetails: redirectData.hrefDetails,
			status: "did",
			href: redirectData.href,
		};
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
	const MAX_REDIRECTS = 10;
	const redirectCount = props.redirectCount || 0;

	if (redirectCount >= MAX_REDIRECTS) {
		logError("Too many redirects");
		return { redirectData: null, response: undefined };
	}

	// Prepare request
	const bodyParentObj: RequestInit = {};
	const isGET = getIsGETRequest(props.requestInit);

	if (props.requestInit && (props.requestInit.body !== undefined || !isGET)) {
		if (
			props.requestInit.body instanceof FormData ||
			typeof props.requestInit.body === "string"
		) {
			bodyParentObj.body = props.requestInit.body;
		} else {
			bodyParentObj.body = JSON.stringify(props.requestInit.body);
		}
	}

	const headers = new Headers(props.requestInit?.headers);
	// To temporarily test traditional server redirect behavior,
	// you can set this to "0" instead of "1"
	headers.set("X-Accepts-Client-Redirect", "1");
	bodyParentObj.headers = headers;

	const finalRequestInit = {
		signal: props.abortController.signal,
		...props.requestInit,
		...bodyParentObj,
	};

	// Execute request
	const res = await fetch(props.url, finalRequestInit);
	let redirectData = parseFetchResponseForRedirectData(finalRequestInit, res);

	return { redirectData, response: res };
}

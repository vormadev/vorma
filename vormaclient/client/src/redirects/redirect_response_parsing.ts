import type { RedirectData } from "./redirects.ts";
import { resolveHTTPRedirectTarget } from "./redirect_href_resolution.ts";

function buildShouldRedirectData(
	href: string,
	latestBuildID: string,
	shouldRedirectStrategy: "hard" | "soft",
): RedirectData | null {
	const resolvedTarget = resolveHTTPRedirectTarget(href);
	if (!resolvedTarget) {
		return null;
	}
	const { hrefDetails } = resolvedTarget;

	return {
		hrefDetails,
		status: "should",
		href,
		shouldRedirectStrategy,
		latestBuildID,
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

	return buildShouldRedirectData(vormaReloadTarget, latestBuildID, "hard");
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

	const isCurrent = newURL.href === window.location.href;
	if (isCurrent) {
		return { hrefDetails, status: "did", href: newURL.href };
	}

	return {
		hrefDetails,
		status: "should",
		href: newURL.href,
		shouldRedirectStrategy: hrefDetails.isInternal ? "soft" : "hard",
		latestBuildID,
	};
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

	return {
		hrefDetails,
		status: "should",
		href: hrefDetails.absoluteURL,
		shouldRedirectStrategy: hrefDetails.isInternal ? "soft" : "hard",
		latestBuildID,
	};
}

export function parseResponseForRedirectData(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	return (
		parseVormaReloadRedirect(response, latestBuildID) ??
		parseBrowserRedirect(response, latestBuildID) ??
		parseClientRedirectHeader(response, latestBuildID)
	);
}

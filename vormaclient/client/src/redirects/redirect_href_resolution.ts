import { getHrefDetails, type HrefDetails } from "vorma/kit/url";

export function resolveHTTPRedirectTarget(href: string): {
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

export function getIsErrorRes(response: Response) {
	return (
		String(response.status).startsWith("4") ||
		String(response.status).startsWith("5")
	);
}

export function getIsGETRequest(requestInit?: RequestInit) {
	return (
		!requestInit?.method ||
		requestInit.method.toLowerCase() === "get" ||
		requestInit.method.toLowerCase() === "head"
	);
}

export function resolveAbsoluteHref(
	href: string | URL,
	baseHref = window.location.href,
): string {
	return new URL(href, baseHref).href;
}

export function resolveAbsoluteHrefWithOptionalSearchAndHash(props: {
	href: string | URL;
	search?: string;
	hash?: string;
	baseHref?: string;
}): string {
	const url = new URL(props.href, props.baseHref ?? window.location.href);
	if (props.search !== undefined) {
		url.search = props.search;
	}
	if (props.hash !== undefined) {
		url.hash = props.hash;
	}
	return url.href;
}

export function getAnchorDetailsFromEvent(event: MouseEvent) {
	if (!event || !event.target || !(event.target as HTMLElement).closest) {
		return null;
	}

	const anchor = (event.target as HTMLElement).closest("a");

	if (!anchor) {
		return null;
	}

	const isEligibleForDefaultPrevention =
		anchor.target !== "_blank" && // ignore new tabs
		event.button !== 1 && // middle mouse button click
		!anchor.href.startsWith("#") && // ignore hash links
		!anchor.hasAttribute("download") && // ignore downloads
		!event.ctrlKey && // ignore ctrl+click
		!event.shiftKey && // ignore shift+click
		!event.metaKey && // ignore cmd+click
		!event.altKey; // ignore alt+click

	const hrefDetails = getHrefDetails(anchor.href);
	const isInternal = hrefDetails.isHTTP && hrefDetails.isInternal;

	return { anchor, isEligibleForDefaultPrevention, isInternal };
}

export type HrefDetails =
	| {
			url: URL;
			isHTTP: true;
			absoluteURL: string;
			relativeURL: string;
			isExternal: boolean;
			isInternal: boolean;
	  }
	| {
			isHTTP: false;
	  };

export function getHrefDetails(href: string): HrefDetails {
	if (!href) {
		return { isHTTP: false };
	}

	let url: URL;

	try {
		url = new URL(href, window.location.href);
	} catch {
		return { isHTTP: false };
	}

	const isExternal = url.origin !== window.location.origin;
	const isInternal = !isExternal;

	// Filter out things like "#", "tel:", "mailto:", etc.
	const isHTTP = url.protocol.startsWith("http");
	if (!isHTTP) {
		return { isHTTP: false };
	}

	if (isExternal) {
		return {
			url,
			isHTTP: true,
			absoluteURL: url.href,
			relativeURL: "",
			isExternal,
			isInternal,
		};
	}

	return {
		url,
		isHTTP: true,
		absoluteURL: url.href,
		relativeURL: url.href.replace(url.origin, ""),
		isExternal,
		isInternal,
	};
}

export function getPrefetchHandlers(href: string, timeout = 50) {
	const hrefDetails = getHrefDetails(href);
	if (!hrefDetails.isHTTP) {
		return;
	}

	const { relativeURL, isExternal } = hrefDetails;
	if (!relativeURL || isExternal) {
		return;
	}

	let timer: number | undefined;

	return {
		...hrefDetails,
		start() {
			timer = window.setTimeout(() => prefetch(relativeURL), timeout);
		},
		stop() {
			document.querySelector(`link[href="${relativeURL}"]`)?.remove();
			clearTimeout(timer);
		},
	};
}

export function prefetch(relativeURL: string) {
	document.querySelector(`link[href="${relativeURL}"]`)?.remove();

	const link = document.createElement("link");
	link.rel = "prefetch";
	link.href = relativeURL;
	link.id = relativeURL;

	document.head.appendChild(link);
}

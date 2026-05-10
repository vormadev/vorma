const fallback_href_base = "https://v.invalid";

export function parse_href(
	href: string,
	base: string = fallback_href_base,
): URL | null {
	try {
		return new URL(href, base);
	} catch {
		return null;
	}
}

export function route_hrefs_share_document(
	left: string,
	right: string,
): boolean {
	const left_url = parse_href(left);
	if (!left_url) {
		return false;
	}
	const right_url = parse_href(right, left_url.href);
	if (!right_url) {
		return false;
	}
	return (
		left_url.origin === right_url.origin &&
		left_url.pathname === right_url.pathname &&
		left_url.search === right_url.search
	);
}

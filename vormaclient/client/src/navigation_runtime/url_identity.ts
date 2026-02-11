export function hrefWithoutHash(href: string): string {
	const url = new URL(href, window.location.href);
	url.hash = "";
	return url.href;
}

export function hasSameDataTarget(a: string, b: string): boolean {
	return hrefWithoutHash(a) === hrefWithoutHash(b);
}

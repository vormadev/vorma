export function resolveNavigationTargetURL(href: string): string {
	return new URL(href, window.location.href).href;
}

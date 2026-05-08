import { Effect } from "effect";

export type BrowserLocation = {
	current_href: () => string;
	current_url: () => URL;
	resolve_href: (href: string | URL) => string;
	resolve_href_or_null: (href: string | URL) => string | null;
	resolve_url: (href: string | URL) => URL;
	is_same_origin_href: (href: string) => boolean;
	route_key: (href: string) => string;
	hash_fragment: (href: string) => string;
	hard_redirect: (href: string) => Effect.Effect<void>;
};

export type BrowserLocationOptions = {
	get_href?: () => string;
	hard_redirect?: (href: string) => void;
};

export function make_browser_location(
	options: BrowserLocationOptions = {},
): Effect.Effect<BrowserLocation> {
	return Effect.sync(() => {
		const get_href =
			options.get_href ??
			((): string => {
				return window.location.href;
			});
		const hard_redirect =
			options.hard_redirect ??
			((href: string): void => {
				window.location.href = href;
			});

		const current_href = (): string => {
			return get_href();
		};

		const current_url = (): URL => {
			return new URL(current_href());
		};

		const resolve_url = (href: string | URL): URL => {
			return new URL(String(href), current_href());
		};

		const resolve_href = (href: string | URL): string => {
			return resolve_url(href).href;
		};

		const resolve_href_or_null = (href: string | URL): string | null => {
			try {
				return resolve_href(href);
			} catch {
				return null;
			}
		};

		const is_same_origin_href = (href: string): boolean => {
			try {
				return resolve_url(href).origin === current_url().origin;
			} catch {
				return false;
			}
		};

		const route_key = (href: string): string => {
			try {
				const url = resolve_url(href);
				url.hash = "";
				return url.href;
			} catch {
				return href;
			}
		};

		const hash_fragment = (href: string): string => {
			try {
				return resolve_url(href).hash;
			} catch {
				return "";
			}
		};

		return {
			current_href,
			current_url,
			resolve_href,
			resolve_href_or_null,
			resolve_url,
			is_same_origin_href,
			route_key,
			hash_fragment,
			hard_redirect: (href) => {
				return Effect.sync(() => {
					hard_redirect(href);
				});
			},
		};
	});
}

import {
	plugin_name,
	pub_url_fn_name,
	public_url_parse_base,
	public_url_prefix,
} from "./plugin_contract.ts";
import { escape_regex_literal } from "./text.ts";

const regex_gap = `(?:\\s|//[^\\n]*\\n|/\\*[\\s\\S]*?\\*/)*`;
const public_url_regex = new RegExp(
	`${pub_url_fn_name}${regex_gap}\\(${regex_gap}(["'\`])(.*?)\\1${regex_gap}\\)`,
	"g",
);
const public_css_url_regex = new RegExp(
	`url\\(${regex_gap}(["']?)(${escape_regex_literal(public_url_prefix)}[^"')\\s]+)\\1${regex_gap}\\)`,
	"g",
);

export const js_module_regex = /\.[cm]?[jt]sx?($|\?)/;

export type PostCSSDeclaration = {
	value: string;
};

export type PostCSSRoot = {
	walkDecls: (callback: (decl: PostCSSDeclaration) => void) => void;
};

export type PostCSSPlugin = {
	postcssPlugin: string;
	Once: (root: PostCSSRoot) => Promise<void>;
};

export type FetchPublicUrl = (src_path: string, source: string) => Promise<string>;

export type MarkResolvedPublicUrl = (public_url: string) => void;

export async function resolve_public_js_urls(
	code: string,
	fetch_public_url: FetchPublicUrl,
): Promise<string> {
	public_url_regex.lastIndex = 0;
	if (!public_url_regex.test(code)) {
		public_url_regex.lastIndex = 0;
		return code;
	}
	public_url_regex.lastIndex = 0;

	const matches: { full: string; asset_path: string }[] = [];
	let m: RegExpExecArray | null;
	while ((m = public_url_regex.exec(code)) !== null) {
		matches.push({ full: m[0], asset_path: m[2]! });
	}

	const resolved = await Promise.all(
		matches.map(async ({ full, asset_path }) => {
			return {
				full,
				hashed: await fetch_public_url(
					asset_path,
					`${pub_url_fn_name}("${asset_path}")`,
				),
			};
		}),
	);

	let result = code;
	for (const { full, hashed } of resolved) {
		result = result.replace(full, `"${hashed}"`);
	}
	return result;
}

export async function resolve_public_css_urls(
	css_value: string,
	fetch_public_url: FetchPublicUrl,
	mark_resolved_public_url: MarkResolvedPublicUrl,
): Promise<string> {
	public_css_url_regex.lastIndex = 0;
	if (!public_css_url_regex.test(css_value)) {
		public_css_url_regex.lastIndex = 0;
		return css_value;
	}
	public_css_url_regex.lastIndex = 0;

	const matches: {
		full: string;
		asset_path: string;
		lookup_path: string;
		suffix: string;
	}[] = [];
	let m: RegExpExecArray | null;
	while ((m = public_css_url_regex.exec(css_value)) !== null) {
		const asset_path = m[2]!;
		const parsed_public_url = new URL(
			asset_path.slice(public_url_prefix.length),
			public_url_parse_base,
		);
		matches.push({
			full: m[0],
			asset_path,
			lookup_path: parsed_public_url.pathname.slice(1),
			suffix: parsed_public_url.search + parsed_public_url.hash,
		});
	}

	const resolved = await Promise.all(
		matches.map(async ({ full, asset_path, lookup_path, suffix }) => {
			return {
				full,
				hashed: (await fetch_public_url(lookup_path, asset_path)) + suffix,
			};
		}),
	);

	let result = css_value;
	for (const { full, hashed } of resolved) {
		mark_resolved_public_url(hashed);
		result = result.replace(full, `url("${hashed}")`);
	}
	return result;
}

export function public_css_url_postcss_plugin(
	fetch_public_url: FetchPublicUrl,
	mark_resolved_public_url: MarkResolvedPublicUrl,
): PostCSSPlugin {
	return {
		postcssPlugin: `${plugin_name}-public-url`,
		async Once(root: PostCSSRoot) {
			const work: Array<Promise<void>> = [];
			root.walkDecls((decl) => {
				work.push(
					resolve_public_css_urls(
						decl.value,
						fetch_public_url,
						mark_resolved_public_url,
					).then((value) => {
						decl.value = value;
					}),
				);
			});
			await Promise.all(work);
		},
	};
}

import { parseAst } from "vite";
import { public_url_prefix } from "./plugin_contract.gen.ts";
import {
	plugin_name,
	pub_url_fn_name,
	public_url_parse_base,
} from "./plugin_contract.ts";
import { escape_regex_literal } from "./text.ts";

const regex_gap = `(?:\\s|//[^\\n]*\\n|/\\*[\\s\\S]*?\\*/)*`;
const public_css_url_regex = new RegExp(
	`url\\(${regex_gap}(["']?)(${escape_regex_literal(public_url_prefix)}[^"')\\s]+)\\1${regex_gap}\\)`,
	"g",
);

export const js_module_regex = /\.[cm]?[jt]sx?($|\?)/;

export type PostCSSDeclaration = {
	value: string;
};

export type PostCSSRoot = {
	source?: { input?: { file?: string } };
	walkDecls: (callback: (decl: PostCSSDeclaration) => void) => void;
};

export type PostCSSPlugin = {
	postcssPlugin: string;
	Once: (root: PostCSSRoot) => Promise<void>;
};

export type FetchPublicUrl = (src_path: string, source: string) => Promise<string>;

export type MarkResolvedPublicUrl = (public_url: string) => void;

type JsParseLang = "js" | "jsx" | "ts" | "tsx";

type AstRecord = Record<string, unknown>;

type PublicJsUrlCall = {
	asset_path: string;
	end: number;
	source: string;
	start: number;
};

export async function resolve_public_js_urls(
	code: string,
	id: string,
	fetch_public_url: FetchPublicUrl,
): Promise<string> {
	if (!code.includes(pub_url_fn_name)) {
		return code;
	}

	const ast = parseAst(code, { lang: parse_lang_from_id(id) });
	const calls: PublicJsUrlCall[] = [];
	collect_public_js_url_calls(ast as unknown as AstRecord, code, calls);
	if (calls.length === 0) {
		return code;
	}

	const resolved = await Promise.all(
		calls.map(async (call) => {
			return {
				call,
				hashed: await fetch_public_url(call.asset_path, call.source),
			};
		}),
	);

	let result = code;
	for (const { call, hashed } of resolved.sort((a, b) => {
		return b.call.start - a.call.start;
	})) {
		result = result.slice(0, call.start) + `"${hashed}"` + result.slice(call.end);
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
	record_asset_consumer?: (lookup_path: string, consumer_file: string) => void,
): PostCSSPlugin {
	return {
		postcssPlugin: `${plugin_name}-public-url`,
		async Once(root: PostCSSRoot) {
			const consumer_file = root.source?.input?.file;
			const fetch_and_record: FetchPublicUrl =
				consumer_file !== undefined && record_asset_consumer !== undefined
					? (lookup_path, source) => {
							record_asset_consumer(lookup_path, consumer_file);
							return fetch_public_url(lookup_path, source);
						}
					: fetch_public_url;
			const work: Array<Promise<void>> = [];
			root.walkDecls((decl) => {
				work.push(
					resolve_public_css_urls(
						decl.value,
						fetch_and_record,
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

function parse_lang_from_id(id: string): JsParseLang {
	const normalized_id = id.split("?", 1)[0] ?? id;
	if (normalized_id.endsWith(".tsx")) {
		return "tsx";
	}
	if (normalized_id.endsWith(".jsx")) {
		return "jsx";
	}
	if (
		normalized_id.endsWith(".ts") ||
		normalized_id.endsWith(".mts") ||
		normalized_id.endsWith(".cts")
	) {
		return "ts";
	}
	return "js";
}

function collect_public_js_url_calls(
	node: unknown,
	code: string,
	calls: PublicJsUrlCall[],
): void {
	if (!is_record(node)) {
		return;
	}

	if (node.type === "CallExpression" && is_public_url_call(node)) {
		calls.push(public_js_url_call_from_node(node, code));
	}

	for (const value of Object.values(node)) {
		if (Array.isArray(value)) {
			for (const item of value) {
				collect_public_js_url_calls(item, code, calls);
			}
			continue;
		}
		if (is_record(value) && typeof value.type === "string") {
			collect_public_js_url_calls(value, code, calls);
		}
	}
}

function is_record(value: unknown): value is AstRecord {
	return Boolean(value) && typeof value === "object";
}

function is_public_url_call(node: AstRecord): boolean {
	const callee = node.callee;
	return (
		is_record(callee) &&
		callee.type === "Identifier" &&
		callee.name === pub_url_fn_name
	);
}

function public_js_url_call_from_node(node: AstRecord, code: string): PublicJsUrlCall {
	const args = Array.isArray(node.arguments) ? node.arguments : [];
	const first_arg = args[0];
	const asset_path =
		args.length === 1 ? static_string_argument_value(first_arg) : undefined;
	if (asset_path === undefined) {
		throw new Error(
			`[${plugin_name}] ${pub_url_fn_name}() requires exactly one static string argument`,
		);
	}
	if (typeof node.start !== "number" || typeof node.end !== "number") {
		throw new Error(`[${plugin_name}] could not locate ${pub_url_fn_name}() call`);
	}
	return {
		asset_path,
		end: node.end,
		source: code.slice(node.start, node.end),
		start: node.start,
	};
}

function static_string_argument_value(value: unknown): string | undefined {
	if (!is_record(value)) {
		return undefined;
	}
	if (value.type === "Literal" && typeof value.value === "string") {
		return value.value;
	}
	if (
		value.type === "TemplateLiteral" &&
		Array.isArray(value.expressions) &&
		value.expressions.length === 0 &&
		Array.isArray(value.quasis) &&
		value.quasis.length === 1
	) {
		const quasi = value.quasis[0];
		if (!is_record(quasi) || !is_record(quasi.value)) {
			return undefined;
		}
		if (typeof quasi.value.cooked === "string") {
			return quasi.value.cooked;
		}
		if (typeof quasi.value.raw === "string") {
			return quasi.value.raw;
		}
	}
	return undefined;
}

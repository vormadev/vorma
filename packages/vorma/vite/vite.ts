import { createServer } from "node:http";
import { resolve } from "node:path";
import {
	normalizePath,
	type Plugin as Vite_Plugin,
	type ResolvedConfig as Vite_ResolvedConfig,
	type UserConfig as Vite_UserConfig,
} from "vite";
import {
	cfg_changed_path,
	type Config,
	env_key,
	loopback_host,
	plugin_base_path,
	plugin_name,
	public_url_parse_base,
	rpc_path,
	type RpcRequest,
	token_env_key,
	token_header,
} from "./plugin_contract.ts";
import {
	js_module_regex,
	public_css_url_postcss_plugin,
	resolve_public_js_urls,
} from "./public_url_resolution.ts";
import { escape_regex_literal } from "./text.ts";

const server_endpoints = {
	// POST -- tokenized RPC endpoint for config, public URL hashes, and control port.
	rpc: () => {
		return server_url(rpc_path);
	},
};

const vite_endpoints = {
	// POST -- called by the Vorma dev server when the config changes.
	cfg_changed: cfg_changed_path,
};

const node_modules_refresh_exclude = /\/node_modules\//;

async function fetch_public_url(src_path: string, source: string): Promise<string> {
	const res = await server_rpc({ method: "hash", src_path }, false);
	if (res.status === 404) {
		throw new Error(`[${plugin_name}] unresolved static public asset: ${source}`);
	}
	await check_ok(server_endpoints.rpc(), res);
	return res.text();
}

/// HMR preamble injected into view modules during dev.
/// The self-accept callback forwards the new module to the client
/// core, which handles the component/client-loader swap and re-commit.
const hmr_preamble = [
	"if (import.meta.hot) {",
	"  import.meta.hot.accept((mod) => {",
	"    if (mod) {",
	"      window.__vorma_hmr_view_update?.(import.meta.url, mod);",
	"    }",
	"  });",
	"}",
].join("\n");

export default function vorma(): Vite_Plugin {
	let view_module_ids: Set<string> | null = null;
	let view_modules: Array<string> = [];
	const resolved_public_css_urls = new Set<string>();

	const mark_resolved_public_url = (public_url: string) => {
		resolved_public_css_urls.add(public_url);
		const parsed_public_url = new URL(public_url, public_url_parse_base);
		resolved_public_css_urls.add(
			parsed_public_url.pathname +
				parsed_public_url.search +
				parsed_public_url.hash,
		);
		resolved_public_css_urls.add(parsed_public_url.pathname);
	};

	return {
		name: plugin_name,

		// Run before framework plugins so Vorma's view-module
		// transform is applied early. React Fast Refresh is disabled
		// for view modules through `oxc.jsxRefreshExclude` below.
		enforce: "pre",

		// Call Vorma dev server to fetch config.
		async config(_: Vite_UserConfig, { command }) {
			const res = await server_rpc({ method: "cfg" });
			const cfg: Config = await res.json();
			view_modules = cfg.ViewModules;
			const view_refresh_excludes = view_modules.map((p) => {
				return module_id_filter_regex(p);
			});
			const out_prefix = "vorma_out_vite_[name]_[hash]";
			const is_prod = command === "build";
			const config: Vite_UserConfig = {
				base: is_prod ? cfg.PublicStaticBasePath : "/",
				publicDir: false,
				build: {
					target: "es2022",
					emptyOutDir: false,
					modulePreload: { polyfill: false },
					rolldownOptions: {
						external: (url: string) => {
							return resolved_public_css_urls.has(url);
						},
						input: [cfg.EntryModule, ...cfg.ViewModules],
						preserveEntrySignatures: "exports-only",
						output: {
							assetFileNames: out_prefix + "[extname]",
							chunkFileNames: out_prefix + ".js",
							entryFileNames: out_prefix + ".js",
						},
					},
				},
				server: {
					headers: { "cache-control": "no-store" },
					watch: { ignored: cfg.IgnoredPatterns },
				},
				resolve: {
					dedupe: cfg.DedupeList,
				},
				css: {
					postcss: {
						plugins: [
							public_css_url_postcss_plugin(
								fetch_public_url,
								mark_resolved_public_url,
							),
						],
					},
				},
			};
			if (!is_prod) {
				config.oxc = {
					jsxRefreshInclude: js_module_regex,
					jsxRefreshExclude: [
						node_modules_refresh_exclude,
						...view_refresh_excludes,
					],
				};
			}
			return config;
		},

		configResolved(resolved: Vite_ResolvedConfig) {
			view_module_ids = new Set(
				view_modules.map((p) => {
					return normalize_module_id(resolve(resolved.root, p));
				}),
			);
		},

		// Transform URLs with `vormaPublicUrl()` in the source
		// code to their hashed equivalents.
		async transform(code: string, id: string) {
			if (/node_modules/.test(id)) {
				return null;
			}

			let result = code;

			// Public URL resolution
			const is_js_module = js_module_regex.test(id);

			if (is_js_module) {
				result = await resolve_public_js_urls(result, fetch_public_url);
			}

			// HMR self-accept injection for view modules (dev only)
			if (view_module_ids?.has(normalize_module_id(id))) {
				result += "\n" + hmr_preamble;
			}

			return result === code ? null : result;
		},

		// Expose an endpoint that the Vorma dev server can call when the user's
		// config changes, so this plugin can restart Vite and read fresh config.
		configureServer(vite_server) {
			const ctrl = createServer((req, res) => {
				if (req.url === "/") {
					res.writeHead(200).end("ok");
					return;
				}
				if (req.url === vite_endpoints.cfg_changed && req.method === "POST") {
					if (!has_valid_token(req)) {
						res.writeHead(403).end();
						return;
					}
					res.writeHead(200).end("ok");
					void vite_server.restart();
					return;
				}
				res.writeHead(404).end();
			});

			ctrl.listen(0, loopback_host, async () => {
				const addr = ctrl.address();
				const port = typeof addr === "object" && addr ? addr.port : null;
				if (port == null) {
					throw new Error(`[${plugin_name}] failed to bind control server`);
				}
				await server_rpc({ method: "set_port", port });
			});

			vite_server.httpServer?.on("close", () => {
				return ctrl.close();
			});
		},
	};
}

/////////////////////////////////////////////////////////////////////
/////// Utils
/////////////////////////////////////////////////////////////////////

function server_url(path: string): string {
	const port = process.env[env_key];
	if (!port) {
		throw new Error(`[${plugin_name}] ${env_key} is not set`);
	}
	return `http://${loopback_host}:${port}${plugin_base_path}${path}`;
}

async function server_rpc(request: RpcRequest, check_response = true): Promise<Response> {
	const url = server_endpoints.rpc();
	const res = await fetch(url, {
		method: "POST",
		headers: {
			"content-type": "application/json",
			[token_header]: server_token(),
		},
		body: JSON.stringify(request),
	});
	if (check_response) {
		await check_ok(url, res);
	}
	return res;
}

function server_token(): string {
	const token = process.env[token_env_key];
	if (!token) {
		throw new Error(`[${plugin_name}] ${token_env_key} is not set`);
	}
	return token;
}

function has_valid_token(req: {
	headers: Record<string, string | string[] | undefined>;
}): boolean {
	return req.headers[token_header] === server_token();
}

async function check_ok(url: string, res: Response): Promise<void> {
	if (!res.ok) {
		throw new Error(
			`[${plugin_name}] ${url} returned ${res.status}: ${await res.text()}`,
		);
	}
}

function normalize_module_id(id: string): string {
	const [path] = id.split("?", 1);
	return normalizePath(path ?? id);
}

function module_id_filter_regex(id: string): RegExp {
	const escaped_id = escape_regex_literal(normalize_module_id(id));
	return new RegExp(`^${escaped_id}(?:\\?.*)?$`);
}

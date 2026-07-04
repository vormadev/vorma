import { createServer } from "node:http";
import { resolve } from "node:path";
import {
	normalizePath,
	type HmrContext as Vite_HmrContext,
	type ModuleNode as Vite_ModuleNode,
	type Plugin as Vite_Plugin,
	type ResolvedConfig as Vite_ResolvedConfig,
	type UserConfig as Vite_UserConfig,
} from "vite";
import {
	assets_changed_path,
	cfg_changed_path,
	env_key,
	loopback_host,
	plugin_base_path,
	rpc_path,
	type RpcRequest,
	token_env_key,
	token_header,
	type VitePluginConfig,
} from "./plugin_contract.gen.ts";
import { plugin_name, public_url_parse_base } from "./plugin_contract.ts";
import {
	js_module_regex,
	public_css_url_postcss_plugin,
	resolve_public_js_urls,
} from "./public_url_resolution.ts";
import { escape_regex_literal } from "./text.ts";

/*
The `vorma/vite` plugin. This module's only public export is `vorma()`
itself (see its doc comment below) — everything else here is the plugin's
own implementation: bridging Vite's dev/build lifecycle to the Vorma dev
server's RPC control endpoint (config fetch, restart-on-config-change,
targeted module invalidation on public-asset change), resolving
`vormaPublicUrl(...)` calls to hashed asset URLs at both the JS/TS and CSS
layers, and injecting the dev-only HMR self-accept preamble into view
modules.
*/

function rpc_url(): string {
	return server_url(rpc_path);
}

const node_modules_refresh_exclude = /\/node_modules\//;

async function fetch_public_url(src_path: string, source: string): Promise<string> {
	const res = await server_rpc({ method: "hash", src_path }, false);
	if (res.status === 404) {
		throw new Error(`[${plugin_name}] unresolved static public asset: ${source}`);
	}
	await check_ok(rpc_url(), res);
	return res.text();
}

/*
HMR preamble injected into view modules during dev. The self-accept
callback forwards the new module to the client core, which handles the
component/client-loader swap and re-commit.
*/
const hmr_preamble = [
	"if (import.meta.hot) {",
	"  import.meta.hot.accept((mod) => {",
	"    if (mod) {",
	"      window.__vorma_hmr_view_update?.(import.meta.url, mod);",
	"    }",
	"  });",
	"}",
].join("\n");

/**
 * The Vorma Vite plugin. Add it to `vite.config.ts` alongside the
 * framework plugin for the chosen UI adapter (`@vitejs/plugin-react`,
 * `@preact/preset-vite`, or `vite-plugin-solid`) — `enforce: "pre"` means
 * Vorma's view-module transform runs before those, and React Fast Refresh
 * is disabled for view modules specifically (Vorma owns their HMR
 * contract instead, via the self-accept preamble this plugin injects).
 *
 * ```
 * // vite.config.ts
 * import react from "@vitejs/plugin-react";
 * import vorma from "vorma/vite";
 *
 * export default {
 *   plugins: [vorma(), react()],
 * };
 * ```
 *
 * Takes no options — every configurable input (the entry module, the view
 * module list, ignored watch patterns, the dedupe list) comes from the
 * Rust app's own build configuration via the dev server's RPC endpoint,
 * fetched once per Vite `config` hook invocation; there is no
 * `vite.config.ts`-side knob to duplicate that configuration on the
 * TypeScript side.
 */
export default function vorma(): Vite_Plugin {
	let view_module_ids: Set<string> | null = null;
	let view_modules: Array<string> = [];
	let inject_hmr_preamble = false;
	const resolved_public_css_urls = new Set<string>();
	/*
	Normalized public source path -> files whose transforms baked that
	asset's hashed URL. Changed-asset notifications invalidate exactly these
	files instead of restarting Vite.
	*/
	const public_asset_consumers = new Map<string, Set<string>>();

	const record_asset_consumer = (src_path: string, consumer_file: string) => {
		const key = normalize_public_src_path(src_path);
		let consumers = public_asset_consumers.get(key);
		if (!consumers) {
			consumers = new Set();
			public_asset_consumers.set(key, consumers);
		}
		consumers.add(consumer_file);
	};

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
			const cfg: VitePluginConfig = await res.json();
			view_modules = cfg.view_modules;
			const view_refresh_excludes = view_modules.map((p) => {
				return module_id_filter_regex(p);
			});
			const out_prefix = "vorma_out_vite_[name]_[hash]";
			const is_prod = command === "build";
			inject_hmr_preamble = !is_prod;
			const config: Vite_UserConfig = {
				base: is_prod ? cfg.public_static_base_path : "/",
				publicDir: false,
				build: {
					target: "es2022",
					emptyOutDir: false,
					modulePreload: { polyfill: false },
					rolldownOptions: {
						external: (url: string) => {
							return resolved_public_css_urls.has(url);
						},
						input: [cfg.entry_module, ...cfg.view_modules],
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
					watch: { ignored: cfg.ignored_patterns },
				},
				resolve: {
					dedupe: cfg.dedupe_list,
				},
				css: {
					postcss: {
						plugins: [
							public_css_url_postcss_plugin(
								fetch_public_url,
								mark_resolved_public_url,
								record_asset_consumer,
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
				const consumer_file = normalize_module_id(id);
				result = await resolve_public_js_urls(result, id, (src_path, source) => {
					record_asset_consumer(src_path, consumer_file);
					return fetch_public_url(src_path, source);
				});
			}

			// HMR self-accept injection for view modules (dev only)
			if (inject_hmr_preamble && view_module_ids?.has(normalize_module_id(id))) {
				result += "\n" + hmr_preamble;
			}

			return result === code ? null : result;
		},

		handleHotUpdate(ctx: Vite_HmrContext) {
			if (!inject_hmr_preamble || view_module_ids === null) {
				return;
			}
			const view_modules_to_update = new Map<string, Vite_ModuleNode>();
			for (const module_node of ctx.modules) {
				const id = module_node.id;
				if (id === null) {
					continue;
				}
				const normalized_id = normalize_module_id(id);
				if (view_module_ids.has(normalized_id)) {
					view_modules_to_update.set(normalized_id, module_node);
				}
			}
			if (view_modules_to_update.size === 0) {
				return;
			}
			const invalidated_modules = new Set<Vite_ModuleNode>();
			const view_modules = Array.from(view_modules_to_update.values());
			for (const module_node of view_modules) {
				ctx.server.moduleGraph.invalidateModule(
					module_node,
					invalidated_modules,
					ctx.timestamp,
					true,
				);
			}
			return view_modules;
		},

		// Expose an endpoint that the Vorma dev server can call when the user's
		// config changes, so this plugin can restart Vite and read fresh config.
		configureServer(vite_server) {
			let vite_restart_in_progress = false;
			const ctrl = createServer((req, res) => {
				if (req.url === "/") {
					res.writeHead(200).end("ok");
					return;
				}
				if (req.url === assets_changed_path && req.method === "POST") {
					if (!has_valid_token(req)) {
						res.writeHead(403).end();
						return;
					}
					void read_request_body(req)
						.then((body) => {
							const changed: unknown = JSON.parse(body);
							if (
								!Array.isArray(changed) ||
								changed.some((path) => {
									return typeof path !== "string";
								})
							) {
								res.writeHead(400).end("invalid changed-assets payload");
								return;
							}
							const invalidated = new Set<Vite_ModuleNode>();
							for (const src_path of changed as string[]) {
								const consumers = public_asset_consumers.get(
									normalize_public_src_path(src_path),
								);
								if (!consumers) {
									continue;
								}
								for (const consumer_file of consumers) {
									const modules =
										vite_server.moduleGraph.getModulesByFile(
											consumer_file,
										);
									if (!modules) {
										continue;
									}
									for (const module_node of modules) {
										vite_server.moduleGraph.invalidateModule(
											module_node,
											invalidated,
											Date.now(),
											true,
										);
									}
								}
							}
							res.writeHead(200).end("ok");
						})
						.catch((error: unknown) => {
							const message =
								error instanceof Error ? error.message : String(error);
							res.writeHead(400).end(message);
						});
					return;
				}
				if (req.url === cfg_changed_path && req.method === "POST") {
					if (!has_valid_token(req)) {
						res.writeHead(403).end();
						return;
					}
					vite_restart_in_progress = true;
					void vite_server
						.restart()
						.then(() => {
							res.writeHead(200).end("ok");
						})
						.catch((error: unknown) => {
							const message =
								error instanceof Error ? error.message : String(error);
							res.writeHead(500).end(message);
						})
						.finally(() => {
							vite_restart_in_progress = false;
						});
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
				if (vite_restart_in_progress) {
					return;
				}
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
	const url = rpc_url();
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

function normalize_public_src_path(src_path: string): string {
	const trimmed = src_path.trim();
	return trimmed.startsWith("/") ? trimmed.slice(1) : trimmed;
}

function read_request_body(req: {
	on(event: "data", listener: (chunk: Buffer) => void): void;
	on(event: "end", listener: () => void): void;
	on(event: "error", listener: (error: Error) => void): void;
}): Promise<string> {
	return new Promise((resolve_body, reject) => {
		const chunks: Buffer[] = [];
		req.on("data", (chunk) => {
			chunks.push(chunk);
		});
		req.on("end", () => {
			resolve_body(Buffer.concat(chunks).toString("utf8"));
		});
		req.on("error", reject);
	});
}

function normalize_module_id(id: string): string {
	const [path] = id.split("?", 1);
	return normalizePath(path ?? id);
}

function module_id_filter_regex(id: string): RegExp {
	const escaped_id = escape_regex_literal(normalize_module_id(id));
	return new RegExp(`^${escaped_id}(?:\\?.*)?$`);
}

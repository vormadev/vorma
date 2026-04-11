import { createServer } from "node:http";
import { resolve } from "node:path";
import type {
	Plugin as Vite_Plugin,
	ResolvedConfig as Vite_ResolvedConfig,
	UserConfig as Vite_UserConfig,
} from "vite";

/// Plugin contract
const plugin_name = "vorma-vite-plugin";
const env_key = "__VORMA_VITE_PLUGIN_GO_PORT";
const loopback_host = "127.0.0.1";
const go_endpoints = {
	// GET -- returns JSON config for the plugin
	cfg: () => {
		return go_url("/cfg");
	},
	// GET -- returns hashed path or 404 if not found
	hash: (src_path: string) => {
		return go_url(`/hash?src_path=${encodeURIComponent(src_path)}`);
	},
	// POST -- sets the port that the plugin's control server is listening on
	set_port: (port: number) => {
		return go_url(`/set-port?port=${port}`);
	},
};
const vite_endpoints = {
	// POST -- called by the Go dev server when the Vorma config changes
	cfg_changed: "/cfg-changed",
};
type Config = {
	PublicStaticBasePath: string;
	EntryModule: string;
	RouteModules: Array<string>;
	IgnoredPatterns: Array<string>;
	DedupeList: Array<string>;
};

/// Public URL resolution
const pub_url_fn_name = "vormaPublicURL";
const regex_gap = `(?:\\s|//[^\\n]*\\n|/\\*[\\s\\S]*?\\*/)*`;
const public_url_regex = new RegExp(
	`${pub_url_fn_name}${regex_gap}\\(${regex_gap}(["'\`])(.*?)\\1${regex_gap}\\)`,
	"g",
);

async function fetch_public_url(src_path: string): Promise<string> {
	const url = go_endpoints.hash(src_path);
	const res = await fetch(url);
	if (res.status === 404) {
		throw new Error(
			`[${plugin_name}] unresolved static public asset: ${pub_url_fn_name}("${src_path}")`,
		);
	}
	await check_ok(url, res);
	return res.text();
}

/// HMR preamble injected into route modules during dev.
/// The self-accept callback forwards the new module to the client
/// core, which handles the component/loader swap and re-commit.
const hmr_preamble = [
	"if (import.meta.hot) {",
	"  import.meta.hot.accept((mod) => {",
	"    if (mod) {",
	"      window.__vorma_hmr_route_update?.(import.meta.url, mod);",
	"    }",
	"  });",
	"}",
].join("\n");

export default function vorma(): Vite_Plugin {
	let route_module_ids: Set<string> | null = null;
	let route_modules: Array<string> = [];

	return {
		name: plugin_name,

		// Run before framework plugins (React, Preact, Solid) so
		// the self-accept is already present when they inspect the
		// module. This prevents React Fast Refresh from adding its
		// own accept/invalidate that would conflict with ours.
		enforce: "pre",

		// Call Go dev server to fetch config
		async config(_: Vite_UserConfig, { command }) {
			const url = go_endpoints.cfg();
			const res = await fetch(url);
			await check_ok(url, res);
			const cfg: Config = await res.json();
			route_modules = cfg.RouteModules;
			const out_prefix = "vorma_out_vite_[name]_[hash]";
			const is_prod = command === "build";
			return {
				base: is_prod ? cfg.PublicStaticBasePath : "/",
				build: {
					target: "es2022",
					emptyOutDir: false,
					modulePreload: { polyfill: false },
					rolldownOptions: {
						input: [cfg.EntryModule, ...cfg.RouteModules],
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
			};
		},

		configResolved(resolved: Vite_ResolvedConfig) {
			route_module_ids = new Set(
				route_modules.map((p) => {
					return resolve(resolved.root, p);
				}),
			);
		},

		// Transform URLs with `vormaPublicURL()` in the source
		// code to their hashed equivalents.
		async transform(code: string, id: string) {
			if (/node_modules/.test(id)) {
				return null;
			}

			let result = code;

			// Public URL resolution
			if (public_url_regex.test(result)) {
				// Reset lastIndex after the test pass.
				public_url_regex.lastIndex = 0;

				const matches: { full: string; assetPath: string }[] = [];
				let m: RegExpExecArray | null;
				while ((m = public_url_regex.exec(result)) !== null) {
					matches.push({ full: m[0], assetPath: m[2]! });
				}

				const resolved = await Promise.all(
					matches.map(async ({ full, assetPath }) => {
						return {
							full,
							hashed: await fetch_public_url(assetPath),
						};
					}),
				);

				for (const { full, hashed } of resolved) {
					result = result.replace(full, `"${hashed}"`);
				}
			}

			// HMR self-accept injection for route modules (dev only)
			if (route_module_ids?.has(id)) {
				result += "\n" + hmr_preamble;
			}

			return result === code ? null : result;
		},

		// Expose an endpoint that the Go dev server can call when the
		// user's Vorma config changes, so that this plugin can trigger
		// a Vite server restart (which reads the config fresh).
		configureServer(vite_server) {
			const ctrl = createServer((req, res) => {
				if (req.url === "/") {
					res.writeHead(200).end("ok");
					return;
				}
				if (
					req.url === vite_endpoints.cfg_changed &&
					req.method === "POST"
				) {
					res.writeHead(200).end("ok");
					void vite_server.restart();
					return;
				}
				res.writeHead(404).end();
			});

			ctrl.listen(0, loopback_host, async () => {
				const addr = ctrl.address();
				const port =
					typeof addr === "object" && addr ? addr.port : null;
				if (port == null) {
					throw new Error(
						`[${plugin_name}] failed to bind control server`,
					);
				}
				const url = go_endpoints.set_port(port);
				const res = await fetch(url, { method: "POST" });
				await check_ok(url, res);
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

function go_url(path: string): string {
	const port = process.env[env_key];
	if (!port) {
		throw new Error(`[${plugin_name}] ${env_key} is not set`);
	}
	return `http://${loopback_host}:${port}/vite-plugin${path}`;
}

async function check_ok(url: string, res: Response): Promise<void> {
	if (!res.ok) {
		throw new Error(
			`[${plugin_name}] ${url} returned ${res.status}: ${await res.text()}`,
		);
	}
}

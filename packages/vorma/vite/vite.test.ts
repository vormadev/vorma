import {
	createServer,
	type IncomingMessage,
	type Server,
	type ServerResponse,
} from "node:http";
import { resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
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
import vorma from "./vite.ts";

type RpcCall = {
	request: RpcRequest;
};

type TestRpcServer = {
	calls: RpcCall[];
	close: () => Promise<void>;
};

const test_token = "test-token";

const test_config: VitePluginConfig = {
	public_static_base_path: "/static/",
	entry_module: "src/main.tsx",
	view_modules: ["src/views/root.tsx", "src/views/users/[id].tsx"],
	ignored_patterns: ["**/.vorma/**"],
	dedupe_list: ["react", "react-dom"],
};

const test_hashes = {
	"img/logo.png": "/static/logo.abc123.png",
	"img/bg.png": "/static/bg.def456.png",
};

const env_snapshot = {
	port: process.env[env_key],
	token: process.env[token_env_key],
};

const open_rpc_servers: TestRpcServer[] = [];

afterEach(async () => {
	while (open_rpc_servers.length > 0) {
		await open_rpc_servers.pop()!.close();
	}
	set_or_delete_env(env_key, env_snapshot.port);
	set_or_delete_env(token_env_key, env_snapshot.token);
});

describe("vorma Vite plugin", () => {
	it("produces the expected dev and build config from the tokenized RPC config", async () => {
		const rpc_server = await start_test_rpc_server();
		const plugin = vorma() as any;

		const dev_config = await plugin.config({}, { command: "serve" });
		expect(dev_config.base).toBe("/");
		expect(dev_config.publicDir).toBe(false);
		expect(dev_config.build.target).toBe("es2022");
		expect(dev_config.build.emptyOutDir).toBe(false);
		expect(dev_config.build.modulePreload).toEqual({ polyfill: false });
		expect(dev_config.build.rolldownOptions.input).toEqual([
			test_config.entry_module,
			...test_config.view_modules,
		]);
		expect(dev_config.build.rolldownOptions.preserveEntrySignatures).toBe(
			"exports-only",
		);
		expect(dev_config.build.rolldownOptions.output).toEqual({
			assetFileNames: "vorma_out_vite_[name]_[hash][extname]",
			chunkFileNames: "vorma_out_vite_[name]_[hash].js",
			entryFileNames: "vorma_out_vite_[name]_[hash].js",
		});
		expect(dev_config.server.headers).toEqual({ "cache-control": "no-store" });
		expect(dev_config.server.watch.ignored).toEqual(test_config.ignored_patterns);
		expect(dev_config.resolve.dedupe).toEqual(test_config.dedupe_list);
		expect(dev_config.oxc.jsxRefreshInclude.test("src/main.tsx")).toBe(true);
		expect(
			dev_config.oxc.jsxRefreshExclude.some((regex: RegExp) => {
				return regex.test("src/views/root.tsx");
			}),
		).toBe(true);

		const build_config = await plugin.config({}, { command: "build" });
		expect(build_config.base).toBe(test_config.public_static_base_path);
		expect(build_config.oxc).toBeUndefined();
		expect(
			rpc_server.calls.map((call) => {
				return call.request;
			}),
		).toEqual([{ method: "cfg" }, { method: "cfg" }]);
	});

	it("rewrites JS public URLs and appends HMR only for resolved view modules", async () => {
		await start_test_rpc_server();
		const plugin = vorma() as any;
		const root = "/test-root";

		await plugin.config({}, { command: "serve" });
		plugin.configResolved({ root });

		const view_id = resolve(root, test_config.view_modules[0]!);
		const transformed = await plugin.transform(
			`const logo = vormaPublicUrl("img/logo.png");`,
			view_id,
		);
		expect(transformed).toBe(
			`const logo = "${test_hashes["img/logo.png"]}";\n` +
				[
					"if (import.meta.hot) {",
					"  import.meta.hot.accept((mod) => {",
					"    if (mod) {",
					"      window.__vorma_hmr_view_update?.(import.meta.url, mod);",
					"    }",
					"  });",
					"}",
				].join("\n"),
		);

		const non_view_transformed = await plugin.transform(
			`const logo = vormaPublicUrl("img/logo.png");`,
			resolve(root, "src/not-a-view.ts"),
		);
		expect(non_view_transformed).toBe(
			`const logo = "${test_hashes["img/logo.png"]}";`,
		);

		const template_literal_transformed = await plugin.transform(
			"const logo = vormaPublicUrl(`img/logo.png`);",
			resolve(root, "src/template.ts"),
		);
		expect(template_literal_transformed).toBe(
			`const logo = "${test_hashes["img/logo.png"]}";`,
		);

		const decoy_transformed = await plugin.transform(
			[
				`const text = 'vormaPublicUrl("img/logo.png")';`,
				`// vormaPublicUrl("img/logo.png")`,
				`const logo = vormaPublicUrl("img/logo.png");`,
			].join("\n"),
			resolve(root, "src/decoys.ts"),
		);
		expect(decoy_transformed).toBe(
			[
				`const text = 'vormaPublicUrl("img/logo.png")';`,
				`// vormaPublicUrl("img/logo.png")`,
				`const logo = "${test_hashes["img/logo.png"]}";`,
			].join("\n"),
		);

		const tsx_transformed = await plugin.transform(
			`const el = <img src={vormaPublicUrl("img/logo.png")} />;`,
			resolve(root, "src/component.tsx"),
		);
		expect(tsx_transformed).toBe(
			`const el = <img src={"${test_hashes["img/logo.png"]}"} />;`,
		);

		await expect(
			plugin.transform(
				`const logo = vormaPublicUrl(path);`,
				resolve(root, "src/dynamic.ts"),
			),
		).rejects.toThrow("requires exactly one static string argument");
		await expect(
			plugin.transform(
				"const logo = vormaPublicUrl(`img/${name}.png`);",
				resolve(root, "src/dynamic-template.ts"),
			),
		).rejects.toThrow("requires exactly one static string argument");

		const untouched = await plugin.transform(
			`const logo = vormaPublicUrl("img/logo.png");`,
			resolve(root, "src/style.css"),
		);
		expect(untouched).toBeNull();
	});

	it("does not append HMR preamble during build transforms", async () => {
		await start_test_rpc_server();
		const plugin = vorma() as any;
		const root = "/test-root";

		await plugin.config({}, { command: "build" });
		plugin.configResolved({ root });

		const view_id = resolve(root, test_config.view_modules[0]!);
		const transformed = await plugin.transform(
			`const logo = vormaPublicUrl("img/logo.png");`,
			view_id,
		);
		expect(transformed).toBe(`const logo = "${test_hashes["img/logo.png"]}";`);
	});

	it("routes direct view module HMR through the view boundary and leaves dependencies to Vite", async () => {
		await start_test_rpc_server();
		const plugin = vorma() as any;
		const root = "/test-root";
		const timestamp = 12345;
		const invalidated_modules: TestModuleNode[] = [];

		await plugin.config({}, { command: "serve" });
		plugin.configResolved({ root });

		const view_module = test_module(
			resolve(root, test_config.view_modules[0]!),
			"/src/views/root.tsx",
		);
		const view_dependency = test_module(
			resolve(root, "src/views/hmr-probe.tsx"),
			"/src/views/hmr-probe.tsx",
		);
		const view_dependency_importer = test_module(
			resolve(root, "src/views/hmr-probe-wrapper.tsx"),
			"/src/views/hmr-probe-wrapper.tsx",
		);
		view_dependency.importers.add(view_dependency_importer);
		view_dependency_importer.importers.add(view_module);

		const ctx = {
			modules: [view_dependency],
			timestamp,
			server: {
				moduleGraph: {
					invalidateModule(module_node: TestModuleNode) {
						invalidated_modules.push(module_node);
					},
				},
			},
		};
		expect(plugin.handleHotUpdate(ctx)).toBeUndefined();
		expect(invalidated_modules).toEqual([]);

		invalidated_modules.length = 0;
		expect(plugin.handleHotUpdate({ ...ctx, modules: [view_module] })).toEqual([
			view_module,
		]);
		expect(invalidated_modules).toEqual([view_module]);

		invalidated_modules.length = 0;
		expect(
			plugin.handleHotUpdate({
				...ctx,
				modules: [test_module(resolve(root, "src/unrelated.ts"))],
			}),
		).toBeUndefined();
		expect(invalidated_modules).toEqual([]);
	});

	it("rewrites CSS public URLs and marks the resolved outputs as external", async () => {
		await start_test_rpc_server();
		const plugin = vorma() as any;

		const config = await plugin.config({}, { command: "build" });
		const css_plugin = config.css.postcss.plugins[0];
		const decls = [
			{ value: `background: url("@public/img/bg.png?v=1#hero")` },
			{ value: "color: red" },
		];

		await css_plugin.Once({
			walkDecls(callback: (decl: { value: string }) => void) {
				for (const decl of decls) {
					callback(decl);
				}
			},
		});

		const expected_url = `${test_hashes["img/bg.png"]}?v=1#hero`;
		expect(decls[0]!.value).toBe(`background: url("${expected_url}")`);
		expect(decls[1]!.value).toBe("color: red");
		expect(config.build.rolldownOptions.external(expected_url)).toBe(true);
		expect(config.build.rolldownOptions.external(test_hashes["img/bg.png"])).toBe(
			true,
		);
		expect(config.build.rolldownOptions.external("/static/other.png")).toBe(false);
	});

	it("exposes a token-gated config-change endpoint for Vite restarts", async () => {
		const rpc_server = await start_test_rpc_server();
		const restart_calls: Array<void> = [];
		const close_listeners: Array<() => void> = [];
		const plugin = vorma() as any;

		plugin.configureServer({
			async restart() {
				restart_calls.push(undefined);
				for (const listener of close_listeners) {
					listener();
				}
			},
			httpServer: {
				on(event: string, listener: () => void) {
					if (event === "close") {
						close_listeners.push(listener);
					}
				},
			},
		});

		await wait_for(() => {
			return rpc_server.calls.some((call) => {
				return call.request.method === "set_port";
			});
		});
		const set_port_call = rpc_server.calls.find((call) => {
			return call.request.method === "set_port";
		});
		if (!set_port_call || set_port_call.request.method !== "set_port") {
			throw new Error("Vite control port was not published");
		}
		const control_url = `http://${loopback_host}:${set_port_call.request.port}${cfg_changed_path}`;

		const forbidden = await fetch(control_url, {
			method: "POST",
			headers: { [token_header]: "wrong-token" },
		});
		expect(forbidden.status).toBe(403);
		expect(restart_calls).toHaveLength(0);

		const ok = await fetch(control_url, {
			method: "POST",
			headers: { [token_header]: test_token },
		});
		expect(ok.status).toBe(200);
		expect(await ok.text()).toBe("ok");
		await wait_for(() => {
			return restart_calls.length === 1;
		});
		expect(restart_calls).toHaveLength(1);

		const second_forbidden = await fetch(control_url, {
			method: "POST",
			headers: { [token_header]: "wrong-token" },
		});
		expect(second_forbidden.status).toBe(403);

		for (const listener of close_listeners) {
			listener();
		}
	});

	it("invalidates exactly the modules that referenced changed public assets", async () => {
		const rpc_server = await start_test_rpc_server();
		const plugin = vorma() as any;
		const root = "/test-root";

		await plugin.config({}, { command: "serve" });
		plugin.configResolved({ root });

		const consumer_id = resolve(root, "src/uses-logo.ts");
		await plugin.transform(
			`const logo = vormaPublicUrl("img/logo.png");`,
			consumer_id,
		);

		const invalidate_calls: Array<string> = [];
		const file_lookups: Array<string> = [];
		const close_listeners: Array<() => void> = [];
		const consumer_module = { id: consumer_id };
		plugin.configureServer({
			moduleGraph: {
				getModulesByFile(file: string) {
					file_lookups.push(file);
					if (file === consumer_id) {
						return new Set([consumer_module]);
					}
					return undefined;
				},
				invalidateModule(module_node: { id: string }) {
					invalidate_calls.push(module_node.id);
				},
			},
			async restart() {},
			httpServer: {
				on(event: string, listener: () => void) {
					if (event === "close") {
						close_listeners.push(listener);
					}
				},
			},
		});

		await wait_for(() => {
			return rpc_server.calls.some((call) => {
				return call.request.method === "set_port";
			});
		});
		const set_port_call = rpc_server.calls.find((call) => {
			return call.request.method === "set_port";
		});
		if (!set_port_call || set_port_call.request.method !== "set_port") {
			throw new Error("Vite control port was not published");
		}
		const invalidate_url = `http://${loopback_host}:${set_port_call.request.port}${assets_changed_path}`;

		const forbidden = await fetch(invalidate_url, {
			method: "POST",
			headers: { [token_header]: "wrong-token" },
			body: JSON.stringify(["img/logo.png"]),
		});
		expect(forbidden.status).toBe(403);
		expect(invalidate_calls).toHaveLength(0);

		const invalid_body = await fetch(invalidate_url, {
			method: "POST",
			headers: { [token_header]: test_token },
			body: "not json",
		});
		expect(invalid_body.status).toBe(400);
		expect(invalidate_calls).toHaveLength(0);

		const unrelated = await fetch(invalidate_url, {
			method: "POST",
			headers: { [token_header]: test_token },
			body: JSON.stringify(["img/other.png"]),
		});
		expect(unrelated.status).toBe(200);
		expect(invalidate_calls).toHaveLength(0);

		const ok = await fetch(invalidate_url, {
			method: "POST",
			headers: { [token_header]: test_token },
			// The build sends source keys without a leading slash; a slashed
			// path must hit the same consumers.
			body: JSON.stringify(["/img/logo.png"]),
		});
		expect(ok.status).toBe(200);
		expect(await ok.text()).toBe("ok");
		expect(file_lookups).toEqual([consumer_id]);
		expect(invalidate_calls).toEqual([consumer_id]);

		for (const listener of close_listeners) {
			listener();
		}
	});
});

type TestModuleNode = {
	id: string;
	url: string;
	importers: Set<TestModuleNode>;
};

function test_module(id: string, url = id): TestModuleNode {
	return {
		id,
		url,
		importers: new Set(),
	};
}

async function start_test_rpc_server(): Promise<TestRpcServer> {
	const calls: RpcCall[] = [];
	const server = createServer(async (req, res) => {
		if (req.url !== `${plugin_base_path}${rpc_path}` || req.method !== "POST") {
			res.writeHead(404).end();
			return;
		}
		if (req.headers[token_header] !== test_token) {
			res.writeHead(403).end();
			return;
		}
		const request = JSON.parse(await read_request_body(req)) as RpcRequest;
		calls.push({ request });
		if (request.method === "cfg") {
			write_json(res, test_config);
			return;
		}
		if (request.method === "hash") {
			const hashed = test_hashes[request.src_path as keyof typeof test_hashes];
			if (!hashed) {
				res.writeHead(404).end();
				return;
			}
			res.writeHead(200, { "content-type": "text/plain" }).end(hashed);
			return;
		}
		res.writeHead(200, { "content-type": "text/plain" }).end("ok");
	});

	await new Promise<void>((resolve_listen, reject_listen) => {
		server.once("error", reject_listen);
		server.listen(0, loopback_host, resolve_listen);
	});
	const address = server.address();
	if (!address || typeof address !== "object") {
		throw new Error("test RPC server did not expose a port");
	}
	process.env[env_key] = String(address.port);
	process.env[token_env_key] = test_token;

	const rpc_server = {
		calls,
		close: () => {
			return close_server(server);
		},
	};
	open_rpc_servers.push(rpc_server);
	return rpc_server;
}

async function read_request_body(req: IncomingMessage): Promise<string> {
	const chunks: Buffer[] = [];
	for await (const chunk of req) {
		chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
	}
	return Buffer.concat(chunks).toString("utf8");
}

function write_json(res: ServerResponse<IncomingMessage>, value: unknown): void {
	res.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(value));
}

function close_server(server: Server): Promise<void> {
	return new Promise((resolve_close, reject_close) => {
		server.close((err) => {
			if (err) {
				reject_close(err);
				return;
			}
			resolve_close();
		});
	});
}

function set_or_delete_env(key: string, value: string | undefined): void {
	if (value === undefined) {
		delete process.env[key];
		return;
	}
	process.env[key] = value;
}

async function wait_for(predicate: () => boolean): Promise<void> {
	for (let i = 0; i < 100; i++) {
		if (predicate()) {
			return;
		}
		await new Promise((resolve_timeout) => {
			setTimeout(resolve_timeout, 10);
		});
	}
	throw new Error("condition did not become true");
}

export const plugin_name = "vorma-vite-plugin";
export const env_key = "__VORMA_VITE_PLUGIN_SERVER_PORT";
export const token_env_key = "__VORMA_VITE_PLUGIN_SERVER_TOKEN";
export const token_header = "x-vorma-vite-plugin-token";
export const loopback_host = "127.0.0.1";
export const plugin_base_path = "/vite-plugin";
export const rpc_path = "/rpc";
export const cfg_changed_path = "/cfg-changed";
export const public_url_prefix = "@public/";
export const public_url_parse_base = "http://does-not-matter/";
export const pub_url_fn_name = "vormaPublicUrl";

export type Config = {
	PublicStaticBasePath: string;
	EntryModule: string;
	ViewModules: Array<string>;
	IgnoredPatterns: Array<string>;
	DedupeList: Array<string>;
};

export type RpcRequest =
	| { method: "cfg" }
	| { method: "hash"; src_path: string }
	| { method: "set_port"; port: number };

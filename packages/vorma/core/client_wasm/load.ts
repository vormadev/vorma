/// <reference types="vite/client" />
/// <reference types="node" />

export type RawMatcherExports = {
	memory: WebAssembly.Memory;
	vorma_client_matcher_alloc: (len: number) => number;
	vorma_client_matcher_dealloc: (ptr: number, len: number) => void;
	vorma_client_matcher_find_nested_matches: (
		matcher_id: number,
		ptr: number,
		len: number,
	) => number;
	vorma_client_matcher_free: (matcher_id: number) => void;
	vorma_client_matcher_new: () => number;
	vorma_client_matcher_output_len: () => number;
	vorma_client_matcher_output_ptr: () => number;
	vorma_client_matcher_register_pattern: (
		matcher_id: number,
		ptr: number,
		len: number,
	) => number;
};

let init_promise: Promise<RawMatcherExports> | null = null;

const wasm_url = new URL("./vorma_client_wasm_bg.wasm", import.meta.url);
const vite_fs_url_prefix = "/@fs/";

async function instantiate_wasm(): Promise<RawMatcherExports> {
	if (!import.meta.env || import.meta.env.MODE === "test") {
		const fs = await import(/* @vite-ignore */ "node:fs/promises");
		const process = await import(/* @vite-ignore */ "node:process");
		let file_url = wasm_url;
		if (wasm_url.protocol !== "file:") {
			const pathname = wasm_url.pathname.startsWith(vite_fs_url_prefix)
				? wasm_url.pathname.slice(vite_fs_url_prefix.length - 1)
				: `.${wasm_url.pathname}`;
			file_url = new URL(pathname, `file://${process.cwd()}/`);
		}
		const bytes = await fs.readFile(file_url);
		const result = await WebAssembly.instantiate(bytes as BufferSource, {});
		return result.instance.exports as RawMatcherExports;
	}
	const response = await fetch(wasm_url);
	try {
		const result = await WebAssembly.instantiateStreaming(
			Promise.resolve(response.clone()),
			{},
		);
		return result.instance.exports as RawMatcherExports;
	} catch {
		const result = await WebAssembly.instantiate(await response.arrayBuffer(), {});
		return result.instance.exports as RawMatcherExports;
	}
}

export function init_client_wasm(): Promise<RawMatcherExports> {
	if (!init_promise) {
		init_promise = instantiate_wasm();
	}
	return init_promise;
}

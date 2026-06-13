/*
TypeScript-owned plugin facts. Everything shared with the Rust build lives in
plugin_contract.gen.ts, rendered from the Rust definitions and golden-pinned.
*/

export const plugin_name = "vorma-vite-plugin";
export const public_url_parse_base = "http://does-not-matter/";
export const pub_url_fn_name = "vormaPublicUrl";

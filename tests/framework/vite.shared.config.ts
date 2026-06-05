import preact from "@preact/preset-vite";
import react from "@vitejs/plugin-react";
import type { PluginOption, UserConfig } from "vite";
import solid from "vite-plugin-solid";
import vorma from "vorma/vite";

declare const process: { env: Record<string, string | undefined> };

type BombadilVariant = "react" | "preact" | "solid";

const deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT";
const mode_env_key = "VORMA_BOMBADIL_MODE";

function from_root(path: string): string {
	return new URL(path, import.meta.url).pathname;
}

function hmr_probe_path(variant: BombadilVariant): string {
	if (variant === "react") {
		return "./runtime/react_hmr_probe.tsx";
	}
	if (variant === "preact") {
		return "./runtime/preact_hmr_probe.tsx";
	}
	return "./runtime/solid_hmr_probe.tsx";
}

function variant_plugins(variant: BombadilVariant): PluginOption[] {
	if (variant === "react") {
		return [react() as PluginOption];
	}
	if (variant === "preact") {
		return [preact() as PluginOption];
	}
	return [solid() as PluginOption];
}

export function define_bombadil_vite_config(variant: BombadilVariant): UserConfig {
	const deployment = process.env[deployment_env_key] ?? "A";
	const mode = process.env[mode_env_key] ?? "prod";
	const gen_file =
		mode === "dev" ? `./vorma.${variant}.dev.gen.ts` : `./vorma.${variant}.gen.ts`;

	return {
		cacheDir: `.bombadil/vite-cache/${mode}-${variant}`,
		define: {
			__BOMBADIL_CLIENT_BUILD_TAG__: JSON.stringify(`client-${deployment}`),
		},
		plugins: [...variant_plugins(variant), vorma() as PluginOption],
		resolve: {
			alias: {
				"#hmr-probe": from_root(hmr_probe_path(variant)),
				"#variant-runtime": from_root(`./runtime/${variant}.ts`),
				"#vorma-client": `vorma/${variant}`,
				"#vorma-gen": from_root(gen_file),
			},
		},
	} as UserConfig;
}

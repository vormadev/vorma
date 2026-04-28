import preact from "@preact/preset-vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import vorma from "vorma/vite";

declare const process: { env: Record<string, string | undefined> };

type BombadilVariant = "react" | "preact" | "solid";

const deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT";
const mode_env_key = "VORMA_BOMBADIL_MODE";

function from_root(path: string) {
	return new URL(path, import.meta.url).pathname;
}

function variant_plugin(variant: BombadilVariant) {
	if (variant === "react") {
		return react();
	}
	if (variant === "preact") {
		return preact();
	}
	return solid();
}

export function define_bombadil_vite_config(variant: BombadilVariant) {
	const deployment = process.env[deployment_env_key] ?? "A";
	const mode = process.env[mode_env_key] ?? "prod";
	const gen_file =
		mode === "dev"
			? `./vorma.${variant}.dev.gen.ts`
			: `./vorma.${variant}.gen.ts`;

	return defineConfig({
		cacheDir: `.bombadil/vite-cache/${mode}-${variant}`,
		define: {
			__BOMBADIL_CLIENT_BUILD_TAG__: JSON.stringify(
				`client-${deployment}`,
			),
		},
		plugins: [variant_plugin(variant), vorma()],
		resolve: {
			alias: {
				"#variant-runtime": from_root(`./runtime/${variant}.ts`),
				"#vorma-client": `vorma/${variant}`,
				"#vorma-gen": from_root(gen_file),
			},
		},
	});
}

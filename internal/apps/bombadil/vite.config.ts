import preact from "@preact/preset-vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import vorma from "vorma/vite";

declare const process: { env: Record<string, string | undefined> };

const variant_env_key = "VORMA_BOMBADIL_VARIANT";
const deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT";
const variant = process.env[variant_env_key];
const deployment = process.env[deployment_env_key] ?? "A";

function from_root(path: string) {
	return new URL(path, import.meta.url).pathname;
}

function variant_plugin() {
	if (variant === "react") {
		return react();
	}
	if (variant === "preact") {
		return preact();
	}
	if (variant === "solid") {
		return solid();
	}
	throw new Error(`${variant_env_key} must be one of: react, preact, solid`);
}

export default defineConfig({
	define: {
		__BOMBADIL_CLIENT_BUILD_TAG__: JSON.stringify(`client-${deployment}`),
	},
	plugins: [variant_plugin(), vorma()],
	resolve: {
		alias: {
			"#variant-runtime": from_root(`./runtime/${variant}.ts`),
			"#vorma-client": `vorma/${variant}`,
			"#vorma-gen": from_root(`./vorma.${variant}.gen.ts`),
		},
	},
});

import preact from "@preact/preset-vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import vorma from "vorma/vite";
import { vormaViteConfig } from "./gen/vorma/index.ts";

export default defineConfig({
	plugins: [preact(), vorma(vormaViteConfig), tailwindcss()],
});

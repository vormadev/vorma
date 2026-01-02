import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import vorma from "vorma/vite";
import { vormaViteConfig } from "./frontend/src/vorma.gen.ts";

export default defineConfig({
	plugins: [solid(), vorma(vormaViteConfig), tailwindcss()],
});

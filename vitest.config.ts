import solid from "vite-plugin-solid";
import { defineConfig } from "vitest/config";

export default defineConfig({
	plugins: [solid({ include: "./packages/vorma/ui/solid/**" })],
});

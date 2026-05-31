import solid from "unplugin-solid/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
	plugins: [solid({ include: "./packages/vorma/ui/solid/**" })],
});

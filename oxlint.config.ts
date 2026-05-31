import { defineConfig } from "oxlint";

export default defineConfig({
	ignorePatterns: ["**/node_modules/**", "**/.dist/**"],
	jsPlugins: ["./oxlint.plugin.ts"],
	rules: {
		"vorma-lint-rules/smart-explicit-returns": "error",
		curly: "error",
	},
	options: {
		typeAware: true,
	},
});

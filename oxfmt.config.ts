import { defineConfig } from "oxfmt";

export default defineConfig({
	ignorePatterns: [
		"**/target/**",
		"**/node_modules/**",
		"**/.vorma/**",
		"packages/vorma/core/wire_contract_fixtures.json",
	],
	useTabs: true,
	tabWidth: 4,
	proseWrap: "always",
	printWidth: 90,
	sortImports: { newlinesBetween: false },
	sortPackageJson: false,
	overrides: [{ files: ["*.jsonc"], options: { trailingComma: "none" } }],
});

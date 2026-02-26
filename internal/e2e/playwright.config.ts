import { defineConfig } from "playwright/test";

/////////////////////////////////////////////////////////////////////
/////// Configuration
/////////////////////////////////////////////////////////////////////

/** Playwright configuration for framework-level wave/vorma integration tests. */
export default defineConfig({
	testDir: ".",
	testMatch: "framework.integration.spec.ts",
	timeout: 240_000,
	fullyParallel: false,
	workers: 1,
	forbidOnly: Boolean(process.env.CI),
	retries: process.env.CI ? 1 : 0,
	reporter: [["list"]],
	outputDir: "./artifacts",
	expect: {
		timeout: 15_000,
	},
	use: {
		browserName: "chromium",
		headless: true,
		viewport: { width: 1366, height: 900 },
		trace: "retain-on-failure",
		screenshot: "only-on-failure",
		video: "retain-on-failure",
	},
});

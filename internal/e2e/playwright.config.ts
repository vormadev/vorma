import { defineConfig } from "playwright/test";

/////////////////////////////////////////////////////////////////////
/////// Configuration
/////////////////////////////////////////////////////////////////////

type E2ERunMode = "dev" | "prod";
type E2EUIAdapter = "solid" | "react" | "preact";

type E2ELaneProjectSpec = {
	name: string;
	runMode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
	specFileName: string;
};

const laneProjectSpecs: ReadonlyArray<E2ELaneProjectSpec> = [
	{
		name: "wave-vorma-runtime-dev-solid",
		runMode: "dev",
		uiAdapter: "solid",
		specFileName: "framework.integration.dev.solid.spec.ts",
	},
	{
		name: "wave-vorma-runtime-dev-react",
		runMode: "dev",
		uiAdapter: "react",
		specFileName: "framework.integration.dev.react.spec.ts",
	},
	{
		name: "wave-vorma-runtime-dev-preact",
		runMode: "dev",
		uiAdapter: "preact",
		specFileName: "framework.integration.dev.preact.spec.ts",
	},
	{
		name: "wave-vorma-runtime-prod-solid",
		runMode: "prod",
		uiAdapter: "solid",
		specFileName: "framework.integration.prod.solid.spec.ts",
	},
	{
		name: "wave-vorma-runtime-prod-react",
		runMode: "prod",
		uiAdapter: "react",
		specFileName: "framework.integration.prod.react.spec.ts",
	},
	{
		name: "wave-vorma-runtime-prod-preact",
		runMode: "prod",
		uiAdapter: "preact",
		specFileName: "framework.integration.prod.preact.spec.ts",
	},
];

const selectedRunModes = resolveRunModesFromEnvironment();
const selectedUIAdapters = resolveUIAdaptersFromEnvironment();

const selectedLaneProjects = laneProjectSpecs
	.filter((laneProjectSpec) =>
		selectedRunModes.includes(laneProjectSpec.runMode),
	)
	.filter((laneProjectSpec) =>
		selectedUIAdapters.includes(laneProjectSpec.uiAdapter),
	);

const defaultWorkerCount =
	process.env.CI === undefined || process.env.CI === ""
		? selectedLaneProjects.length
		: Math.min(3, selectedLaneProjects.length);

const configuredWorkerCount = resolveConfiguredWorkerCount({
	defaultWorkerCount,
});

/** Playwright configuration for framework-level wave/vorma integration tests. */
export default defineConfig({
	testDir: ".",
	testMatch: "framework.integration.*.spec.ts",
	timeout: 240_000,
	fullyParallel: false,
	workers: configuredWorkerCount,
	forbidOnly: Boolean(process.env.CI),
	retries: process.env.CI ? 1 : 0,
	reporter: [["list"]],
	outputDir: "./artifacts",
	projects: selectedLaneProjects.map((laneProjectSpec) => ({
		name: laneProjectSpec.name,
		testMatch: laneProjectSpec.specFileName,
	})),
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

function resolveConfiguredWorkerCount(input: {
	defaultWorkerCount: number;
}): number {
	const workerCountEnv = process.env.VORMA_E2E_WORKERS;
	if (workerCountEnv === undefined || workerCountEnv.trim() === "") {
		return input.defaultWorkerCount;
	}

	const parsedWorkerCount = Number.parseInt(workerCountEnv, 10);
	if (!Number.isFinite(parsedWorkerCount) || parsedWorkerCount < 1) {
		throw new Error(
			`VORMA_E2E_WORKERS must be a positive integer, got ${workerCountEnv}`,
		);
	}

	return parsedWorkerCount;
}

function resolveRunModesFromEnvironment(): ReadonlyArray<E2ERunMode> {
	const runModeEnv = process.env.VORMA_E2E_MODE;
	if (
		runModeEnv === undefined ||
		runModeEnv.trim() === "" ||
		runModeEnv === "all"
	) {
		return ["dev", "prod"];
	}

	if (runModeEnv === "dev" || runModeEnv === "prod") {
		return [runModeEnv];
	}

	throw new Error(
		`unsupported VORMA_E2E_MODE=${runModeEnv}; expected "dev", "prod", or "all"`,
	);
}

function resolveUIAdaptersFromEnvironment(): ReadonlyArray<E2EUIAdapter> {
	const uiAdaptersEnv =
		process.env.VORMA_E2E_UI_ADAPTERS ?? process.env.VORMA_E2E_UI_ADAPTER;
	if (
		uiAdaptersEnv === undefined ||
		uiAdaptersEnv.trim() === "" ||
		uiAdaptersEnv === "all"
	) {
		return ["solid", "react", "preact"];
	}

	const parsedUIAdapters = uiAdaptersEnv
		.split(",")
		.map((uiAdapter) => uiAdapter.trim())
		.filter((uiAdapter) => uiAdapter !== "");
	if (parsedUIAdapters.length === 0) {
		throw new Error(
			`unsupported VORMA_E2E_UI_ADAPTERS=${uiAdaptersEnv}; expected comma-separated adapters`,
		);
	}

	const normalizedUIAdapters = Array.from(new Set(parsedUIAdapters));
	for (const uiAdapter of normalizedUIAdapters) {
		if (
			uiAdapter !== "solid" &&
			uiAdapter !== "react" &&
			uiAdapter !== "preact"
		) {
			throw new Error(
				`unsupported UI adapter=${uiAdapter}; expected "solid", "react", or "preact"`,
			);
		}
	}

	return normalizedUIAdapters as E2EUIAdapter[];
}

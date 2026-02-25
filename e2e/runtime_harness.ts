import { type ChildProcess, spawn } from "node:child_process";
import { once } from "node:events";
import fs from "node:fs";
import { request as httpRequest } from "node:http";
import { createServer } from "node:net";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import { fileURLToPath } from "node:url";

/////////////////////////////////////////////////////////////////////
/////// Types
/////////////////////////////////////////////////////////////////////

/** Supported run modes for framework E2E tests. */
export type E2ERunMode = "dev" | "prod";

/** Supported UI adapters for framework E2E tests. */
export type E2EUIAdapter = "solid" | "react" | "preact";

/** Running fixture process details for Playwright tests. */
export type RunningFixtureSite = {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
	baseURL: string;
	frontendSourceDir: string;
	readRecentOutput: () => string;
	stop: () => Promise<void>;
};

type SpawnedCommand = {
	child: ChildProcess;
	commandLine: string;
	outputLines: string[];
};

type CommandSpec = {
	command: string;
	args: string[];
	cwd: string;
	env: NodeJS.ProcessEnv;
};

type PreparedFixtureVariant = {
	frontendSourceDir: string;
	restore: () => Promise<void>;
};

type AdapterRuntimeSettings = {
	uiAdapter: E2EUIAdapter;
	frontendSourceDir: string;
	clientEntryPath: string;
	clientRouteDefinitionPattern: string;
	tsgenOutDir: string;
	jsxImportSource: "solid-js" | "react" | "preact";
	viteConfigSource: string;
};

type FixtureWaveConfig = {
	Vorma?: {
		UIVariant?: string;
		ClientEntry?: string;
		ClientRouteDefinitionPatterns?: string[];
		TSGenOutDir?: string;
	};
};

/////////////////////////////////////////////////////////////////////
/////// Constants
/////////////////////////////////////////////////////////////////////

const e2eRootDir = path.dirname(fileURLToPath(import.meta.url));
const fixtureRootDir = path.join(e2eRootDir, "fixture_app");
const fixtureWaveConfigPath = path.join(
	fixtureRootDir,
	"backend",
	"wave.config.json",
);
const fixtureViteConfigPath = path.join(fixtureRootDir, "vite.config.ts");
const fixtureTypeScriptConfigPath = path.join(fixtureRootDir, "tsconfig.json");
const outputLineLimit = 800;
const readinessTimeoutMS = 240_000;
const shutdownTimeoutMS = 12_000;

/////////////////////////////////////////////////////////////////////
/////// Public API
/////////////////////////////////////////////////////////////////////

/** Starts the fixture in dev or prod mode and waits for /healthz readiness. */
export async function startFixtureSiteForE2E(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): Promise<RunningFixtureSite> {
	assertFixtureFilesExist();
	const preparedFixtureVariant = await prepareFixtureVariantForAdapter({
		uiAdapter: props.uiAdapter,
	});

	const port = await reserveOpenPort();
	const baseURL = `http://127.0.0.1:${port}`;
	const commonEnv = {
		...process.env,
		PORT: String(port),
		GOCACHE: process.env.GOCACHE ?? "/tmp/go-build",
		VORMA_E2E_UI_ADAPTER: props.uiAdapter,
	};

	let spawnedRuntimeCommand: SpawnedCommand | null = null;

	try {
		if (props.mode === "prod") {
			await runOneShotCommand({
				command: "go",
				args: ["run", "./backend/cmd/build"],
				cwd: fixtureRootDir,
				env: commonEnv,
			});

			spawnedRuntimeCommand = spawnManagedCommand({
				command: resolveProdBinaryPath(),
				args: [],
				cwd: fixtureRootDir,
				env: commonEnv,
			});
		} else {
			spawnedRuntimeCommand = spawnManagedCommand({
				command: "go",
				args: ["run", "./backend/cmd/build", "--dev"],
				cwd: fixtureRootDir,
				env: commonEnv,
			});
		}

		await waitForHealthEndpoint({
			baseURL,
			spawnedCommand: spawnedRuntimeCommand,
		});
		if (spawnedRuntimeCommand === null) {
			throw new Error(
				"runtime command unexpectedly missing after readiness check",
			);
		}
		const readyRuntimeCommand = spawnedRuntimeCommand;

		return {
			mode: props.mode,
			uiAdapter: props.uiAdapter,
			baseURL,
			frontendSourceDir: preparedFixtureVariant.frontendSourceDir,
			readRecentOutput: () =>
				formatRecentCommandOutput({
					spawnedCommand: readyRuntimeCommand,
				}),
			stop: async () => {
				await stopManagedCommand({
					spawnedCommand: readyRuntimeCommand,
				});
				await preparedFixtureVariant.restore();
			},
		};
	} catch (error) {
		if (spawnedRuntimeCommand !== null) {
			try {
				await stopManagedCommand({
					spawnedCommand: spawnedRuntimeCommand,
				});
			} catch {
				// Ignore cleanup errors so the original startup error is preserved.
			}
		}
		await preparedFixtureVariant.restore();
		throw error;
	}
}

/////////////////////////////////////////////////////////////////////
/////// Command Helpers
/////////////////////////////////////////////////////////////////////

/** Spawns a long-lived command and captures stdout/stderr for diagnostics. */
function spawnManagedCommand(props: CommandSpec): SpawnedCommand {
	const commandLine = [props.command, ...props.args].join(" ");
	const outputLines: string[] = [];

	const child = spawn(props.command, props.args, {
		cwd: props.cwd,
		env: props.env,
		stdio: ["ignore", "pipe", "pipe"],
		detached: process.platform !== "win32",
	});

	child.stdout?.on("data", (chunk: Buffer) => {
		appendOutputChunk({ chunk, outputLines });
	});
	child.stderr?.on("data", (chunk: Buffer) => {
		appendOutputChunk({ chunk, outputLines });
	});

	return { child, commandLine, outputLines };
}

/** Runs a one-shot command and throws on non-zero exit. */
async function runOneShotCommand(props: CommandSpec): Promise<void> {
	const spawnedCommand = spawnManagedCommand(props);
	const commandResult = await waitForChildExit({
		child: spawnedCommand.child,
	});

	if (commandResult.exitCode === 0) {
		return;
	}

	throw new Error(
		[
			`command failed: ${spawnedCommand.commandLine}`,
			`exit code: ${commandResult.exitCode ?? "null"}`,
			`signal: ${commandResult.signal ?? "null"}`,
			"recent output:",
			formatRecentCommandOutput({ spawnedCommand }),
		].join("\n"),
	);
}

/** Attempts graceful shutdown before forcing process termination. */
async function stopManagedCommand(props: {
	spawnedCommand: SpawnedCommand;
}): Promise<void> {
	const { child } = props.spawnedCommand;
	if (child.exitCode !== null || child.signalCode !== null) {
		return;
	}

	try {
		terminateProcessGroup({ child, signal: "SIGTERM" });
	} catch {
		// Best-effort graceful stop; fallback to SIGKILL below if needed.
	}

	const gracefulExit = await Promise.race([
		waitForChildExit({ child }),
		sleep(shutdownTimeoutMS).then(() => null),
	]);
	if (gracefulExit !== null) {
		return;
	}

	try {
		terminateProcessGroup({ child, signal: "SIGKILL" });
	} catch {
		// Process may have exited between timeout and forced kill.
	}
	await waitForChildExit({ child });
}

/////////////////////////////////////////////////////////////////////
/////// Fixture Variant Preparation
/////////////////////////////////////////////////////////////////////

/** Rewrites fixture config files for the selected adapter and returns restore. */
async function prepareFixtureVariantForAdapter(props: {
	uiAdapter: E2EUIAdapter;
}): Promise<PreparedFixtureVariant> {
	const adapterRuntimeSettings = resolveAdapterRuntimeSettings({
		uiAdapter: props.uiAdapter,
	});
	const mutableFixtureFiles = [
		fixtureWaveConfigPath,
		fixtureViteConfigPath,
		fixtureTypeScriptConfigPath,
	] as const;
	const snapshotByFilePath = await readFileSnapshotByPath({
		filePaths: mutableFixtureFiles,
	});
	const rawWaveConfigJSON = snapshotByFilePath[fixtureWaveConfigPath];
	if (rawWaveConfigJSON === undefined) {
		throw new Error("missing wave config snapshot");
	}

	const fixtureWaveConfig = parseFixtureWaveConfig({
		rawJSON: rawWaveConfigJSON,
	});

	const vormaConfig = fixtureWaveConfig.Vorma;
	if (vormaConfig === undefined) {
		throw new Error(
			"fixture wave config missing Vorma section; cannot select adapter",
		);
	}

	vormaConfig.UIVariant = adapterRuntimeSettings.uiAdapter;
	vormaConfig.ClientEntry = adapterRuntimeSettings.clientEntryPath;
	vormaConfig.ClientRouteDefinitionPatterns = [
		adapterRuntimeSettings.clientRouteDefinitionPattern,
	];
	vormaConfig.TSGenOutDir = adapterRuntimeSettings.tsgenOutDir;

	await writeTextFile({
		filePath: fixtureWaveConfigPath,
		contents: `${JSON.stringify(fixtureWaveConfig, null, "\t")}\n`,
	});
	await writeTextFile({
		filePath: fixtureViteConfigPath,
		contents: adapterRuntimeSettings.viteConfigSource,
	});
	const rawTypeScriptConfigJSON =
		snapshotByFilePath[fixtureTypeScriptConfigPath];
	if (rawTypeScriptConfigJSON === undefined) {
		throw new Error("missing TypeScript config snapshot");
	}

	const fixtureTypeScriptConfig = parseFixtureTypeScriptConfig({
		rawJSON: rawTypeScriptConfigJSON,
	});

	const fixtureCompilerOptions = fixtureTypeScriptConfig.compilerOptions;
	if (fixtureCompilerOptions === undefined) {
		throw new Error(
			"fixture tsconfig missing compilerOptions; cannot set jsx import source",
		);
	}

	if (adapterRuntimeSettings.uiAdapter === "solid") {
		fixtureCompilerOptions.jsx = "preserve";
	} else {
		fixtureCompilerOptions.jsx = "react-jsx";
	}
	fixtureCompilerOptions.jsxImportSource =
		adapterRuntimeSettings.jsxImportSource;

	await writeTextFile({
		filePath: fixtureTypeScriptConfigPath,
		contents: `${JSON.stringify(fixtureTypeScriptConfig, null, "\t")}\n`,
	});

	return {
		frontendSourceDir: path.join(
			fixtureRootDir,
			adapterRuntimeSettings.frontendSourceDir,
		),
		restore: async () => {
			await restoreFileSnapshots({ snapshotByFilePath });
		},
	};
}

/** Returns adapter-specific frontend paths and build tool settings. */
function resolveAdapterRuntimeSettings(props: {
	uiAdapter: E2EUIAdapter;
}): AdapterRuntimeSettings {
	const solidSourceDir = "frontend/src";

	switch (props.uiAdapter) {
		case "solid": {
			return {
				uiAdapter: props.uiAdapter,
				frontendSourceDir: solidSourceDir,
				clientEntryPath: `${solidSourceDir}/vorma.entry.tsx`,
				clientRouteDefinitionPattern: `${solidSourceDir}/**/*vorma.routes.ts`,
				tsgenOutDir: `${solidSourceDir}/vorma.gen`,
				jsxImportSource: "solid-js",
				viteConfigSource: [
					'import { defineConfig } from "vite";',
					'import solid from "vite-plugin-solid";',
					'import vorma from "vorma/vite";',
					'import { vormaViteConfig } from "./frontend/src/vorma.gen/index.ts";',
					"",
					"export default defineConfig({",
					"\tplugins: [solid(), vorma(vormaViteConfig)],",
					"});",
					"",
				].join("\n"),
			};
		}
		case "react":
		case "preact": {
			const adapterSourceDir = `frontend/src_${props.uiAdapter}`;
			return {
				uiAdapter: props.uiAdapter,
				frontendSourceDir: adapterSourceDir,
				clientEntryPath: `${adapterSourceDir}/vorma.entry.tsx`,
				clientRouteDefinitionPattern: `${adapterSourceDir}/**/*vorma.routes.ts`,
				tsgenOutDir: `${adapterSourceDir}/vorma.gen`,
				jsxImportSource: props.uiAdapter,
				viteConfigSource: [
					'import { defineConfig } from "vite";',
					'import vorma from "vorma/vite";',
					`import { vormaViteConfig } from "./${adapterSourceDir}/vorma.gen/index.ts";`,
					"",
					"export default defineConfig({",
					"\tplugins: [vorma(vormaViteConfig)],",
					"});",
					"",
				].join("\n"),
			};
		}
	}

	throw new Error(`unsupported ui adapter: ${props.uiAdapter}`);
}

/** Parses fixture wave config JSON and validates expected top-level shape. */
function parseFixtureWaveConfig(props: { rawJSON: string }): FixtureWaveConfig {
	const parsedJSON = JSON.parse(props.rawJSON) as unknown;
	if (parsedJSON === null || typeof parsedJSON !== "object") {
		throw new Error("fixture wave config must be a JSON object");
	}
	return parsedJSON as FixtureWaveConfig;
}

/** Parses fixture tsconfig JSON and validates expected top-level shape. */
function parseFixtureTypeScriptConfig(props: { rawJSON: string }): {
	compilerOptions?: {
		jsx?: string;
		jsxImportSource?: string;
	};
} {
	const parsedJSON = JSON.parse(props.rawJSON) as unknown;
	if (parsedJSON === null || typeof parsedJSON !== "object") {
		throw new Error("fixture tsconfig must be a JSON object");
	}
	return parsedJSON as {
		compilerOptions?: {
			jsx?: string;
			jsxImportSource?: string;
		};
	};
}

/** Reads a text snapshot for each mutable fixture file. */
async function readFileSnapshotByPath(props: {
	filePaths: ReadonlyArray<string>;
}): Promise<Record<string, string>> {
	const snapshotByFilePath: Record<string, string> = {};
	for (const filePath of props.filePaths) {
		snapshotByFilePath[filePath] = await fs.promises.readFile(
			filePath,
			"utf8",
		);
	}
	return snapshotByFilePath;
}

/** Restores all mutable fixture files using their startup snapshots. */
async function restoreFileSnapshots(props: {
	snapshotByFilePath: Record<string, string>;
}): Promise<void> {
	for (const [filePath, contents] of Object.entries(
		props.snapshotByFilePath,
	)) {
		await writeTextFile({ filePath, contents });
	}
}

/** Writes UTF-8 file contents, creating parent directories when needed. */
async function writeTextFile(props: {
	filePath: string;
	contents: string;
}): Promise<void> {
	await fs.promises.mkdir(path.dirname(props.filePath), { recursive: true });
	await fs.promises.writeFile(props.filePath, props.contents, "utf8");
}

/////////////////////////////////////////////////////////////////////
/////// Readiness
/////////////////////////////////////////////////////////////////////

/** Polls /healthz until runtime is ready or exits unexpectedly. */
async function waitForHealthEndpoint(props: {
	baseURL: string;
	spawnedCommand: SpawnedCommand;
}): Promise<void> {
	const deadlineMS = Date.now() + readinessTimeoutMS;
	const healthcheckURL = `${props.baseURL}/healthz`;

	for (;;) {
		if (
			props.spawnedCommand.child.exitCode !== null ||
			props.spawnedCommand.child.signalCode !== null
		) {
			throw new Error(
				[
					`runtime exited before readiness: ${props.spawnedCommand.commandLine}`,
					`exit code: ${props.spawnedCommand.child.exitCode ?? "null"}`,
					`signal: ${props.spawnedCommand.child.signalCode ?? "null"}`,
					"recent output:",
					formatRecentCommandOutput({
						spawnedCommand: props.spawnedCommand,
					}),
				].join("\n"),
			);
		}

		const statusCode = await readHTTPStatusCode({ url: healthcheckURL });
		if (statusCode === 200) {
			return;
		}

		if (Date.now() >= deadlineMS) {
			throw new Error(
				[
					`timed out waiting for ${healthcheckURL}`,
					`command: ${props.spawnedCommand.commandLine}`,
					"recent output:",
					formatRecentCommandOutput({
						spawnedCommand: props.spawnedCommand,
					}),
				].join("\n"),
			);
		}

		await sleep(250);
	}
}

/** Reads an HTTP response status code for liveness checks. */
async function readHTTPStatusCode(props: {
	url: string;
}): Promise<number | null> {
	const parsedURL = new URL(props.url);

	return new Promise((resolve) => {
		const request = httpRequest(
			{
				method: "GET",
				hostname: parsedURL.hostname,
				port: parsedURL.port,
				path: `${parsedURL.pathname}${parsedURL.search}`,
				timeout: 1_000,
			},
			(response) => {
				response.resume();
				resolve(response.statusCode ?? null);
			},
		);

		request.on("timeout", () => {
			request.destroy();
			resolve(null);
		});
		request.on("error", () => {
			resolve(null);
		});
		request.end();
	});
}

/////////////////////////////////////////////////////////////////////
/////// Lower-Level Helpers
/////////////////////////////////////////////////////////////////////

/** Allocates an available loopback port for an upcoming runtime process. */
async function reserveOpenPort(): Promise<number> {
	const probeServer = createServer();
	probeServer.unref();
	probeServer.listen(0, "127.0.0.1");
	await once(probeServer, "listening");

	const listeningAddress = probeServer.address();
	if (listeningAddress === null || typeof listeningAddress === "string") {
		probeServer.close();
		throw new Error("failed to reserve loopback port");
	}

	probeServer.close();
	await once(probeServer, "close");
	return listeningAddress.port;
}

/** Ensures required fixture files exist before runtime boot attempts. */
function assertFixtureFilesExist(): void {
	for (const relativePath of [
		"go.mod",
		"backend/wave.config.json",
		"backend/cmd/build/main.go",
		"backend/cmd/serve/main.go",
		"frontend/src/vorma.entry.tsx",
		"frontend/src_react/vorma.entry.tsx",
		"frontend/src_preact/vorma.entry.tsx",
	]) {
		const absolutePath = path.join(fixtureRootDir, relativePath);
		if (!fs.existsSync(absolutePath)) {
			throw new Error(`missing fixture file: ${absolutePath}`);
		}
	}
}

/** Resolves production runtime binary path created by wave build. */
function resolveProdBinaryPath(): string {
	const binaryPath =
		process.platform === "win32"
			? path.join(fixtureRootDir, "backend", "dist", "main.exe")
			: path.join(fixtureRootDir, "backend", "dist", "main");

	if (!fs.existsSync(binaryPath)) {
		throw new Error(
			`expected production runtime binary at ${binaryPath}, but it was not found`,
		);
	}

	return binaryPath;
}

/** Waits for child process exit/error and returns final status. */
async function waitForChildExit(props: {
	child: ChildProcess;
}): Promise<{ exitCode: number | null; signal: NodeJS.Signals | null }> {
	if (props.child.exitCode !== null || props.child.signalCode !== null) {
		return {
			exitCode: props.child.exitCode,
			signal: props.child.signalCode,
		};
	}

	const childError = once(props.child, "error").then(([error]) => {
		throw error as Error;
	});
	const childExit = once(props.child, "exit").then(([exitCode, signal]) => ({
		exitCode: exitCode as number | null,
		signal: signal as NodeJS.Signals | null,
	}));

	return Promise.race([childError, childExit]);
}

/** Sends a signal to the child process or process group where possible. */
function terminateProcessGroup(props: {
	child: ChildProcess;
	signal: NodeJS.Signals;
}): void {
	const childPID = props.child.pid;
	if (childPID === undefined) {
		return;
	}

	if (process.platform === "win32") {
		props.child.kill(props.signal);
		return;
	}

	process.kill(-childPID, props.signal);
}

/** Returns captured command output in a compact diagnostic format. */
function formatRecentCommandOutput(props: {
	spawnedCommand: SpawnedCommand;
}): string {
	if (props.spawnedCommand.outputLines.length === 0) {
		return "<no output captured>";
	}
	return props.spawnedCommand.outputLines.join("\n");
}

/** Appends chunked output while keeping diagnostic history bounded. */
function appendOutputChunk(props: {
	chunk: Buffer;
	outputLines: string[];
}): void {
	const normalizedLines = props.chunk
		.toString("utf8")
		.split(/\r?\n/)
		.map((line) => line.trimEnd())
		.filter((line) => line.length > 0);

	if (normalizedLines.length === 0) {
		return;
	}

	props.outputLines.push(...normalizedLines);
	if (props.outputLines.length > outputLineLimit) {
		props.outputLines.splice(0, props.outputLines.length - outputLineLimit);
	}
}

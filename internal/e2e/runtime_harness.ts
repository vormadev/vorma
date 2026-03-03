import { type ChildProcess, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { once } from "node:events";
import fs from "node:fs";
import { request as httpRequest } from "node:http";
import { createServer } from "node:net";
import os from "node:os";
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
	restartRuntime: () => Promise<void>;
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
	fixtureRootDir: string;
	frontendSourceDir: string;
	dispose: () => Promise<void>;
};

/////////////////////////////////////////////////////////////////////
/////// Constants
/////////////////////////////////////////////////////////////////////

const e2eRootDir = path.dirname(fileURLToPath(import.meta.url));
const repositoryRootDir = path.dirname(path.dirname(e2eRootDir));
const fixtureOverlayTemplatesRootDir = path.join(
	e2eRootDir,
	"overlay_templates",
);
const isolatedFixtureRootDirectoryPrefix = path.join(
	os.tmpdir(),
	"wave-vorma-e2e-fixture-",
);
const sharedFixtureTemplateDirectoryPrefix = path.join(
	os.tmpdir(),
	"wave-vorma-e2e-fixture-template-",
);
const fixtureTemplateCacheVersion = "v2";
const outputLineLimit = 800;
const readinessTimeoutMS = 240_000;
const shutdownTimeoutMS = 12_000;
const runtimeStartupPortRetryCount = 3;

const trackedLongLivedRuntimeCommands = new Set<SpawnedCommand>();
let hasInstalledTrackedRuntimeCommandCleanupHooks = false;

/////////////////////////////////////////////////////////////////////
/////// Public API
/////////////////////////////////////////////////////////////////////

/** Starts the fixture in dev or prod mode and waits for /healthz readiness. */
export async function startFixtureSiteForE2E(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): Promise<RunningFixtureSite> {
	assertFixtureFilesExist();
	const viteDefaultPort = resolvePreferredViteDefaultPortForLane({
		mode: props.mode,
		uiAdapter: props.uiAdapter,
	});
	const preparedFixtureVariant = await prepareFixtureVariantForAdapter({
		uiAdapter: props.uiAdapter,
		viteDefaultPort,
	});

	let runtimePort = await resolvePortForLane({
		mode: props.mode,
		uiAdapter: props.uiAdapter,
	});
	let runtimeBaseURL = `http://127.0.0.1:${runtimePort}`;
	const baseRuntimeEnv = {
		...process.env,
		GOCACHE: process.env.GOCACHE ?? "/tmp/go-build",
		VORMA_E2E_UI_ADAPTER: props.uiAdapter,
	};

	let spawnedRuntimeCommand: SpawnedCommand | null = null;
	let hasDisposedFixtureVariant = false;

	async function disposeFixtureVariantOnce(): Promise<void> {
		if (hasDisposedFixtureVariant) {
			return;
		}
		hasDisposedFixtureVariant = true;
		await preparedFixtureVariant.dispose();
	}

	function mustGetRunningRuntimeCommand(): SpawnedCommand {
		if (spawnedRuntimeCommand === null) {
			throw new Error("runtime command is not currently running");
		}
		return spawnedRuntimeCommand;
	}

	function resolveRuntimeEnvironmentForPort(port: number): NodeJS.ProcessEnv {
		return {
			...baseRuntimeEnv,
			PORT: String(port),
		};
	}

	async function startRuntimeProcess(runtimeStartupOptions: {
		runProdBuild: boolean;
	}): Promise<void> {
		if (runtimeStartupOptions.runProdBuild && props.mode === "prod") {
			await runOneShotCommand({
				command: "go",
				args: ["run", "./backend/cmd/build"],
				cwd: preparedFixtureVariant.fixtureRootDir,
				env: resolveRuntimeEnvironmentForPort(runtimePort),
			});
		}

		for (
			let runtimeStartupAttemptIndex = 0;
			runtimeStartupAttemptIndex < runtimeStartupPortRetryCount;
			runtimeStartupAttemptIndex += 1
		) {
			const runtimeCommandSpec = resolveRuntimeCommandSpec({
				mode: props.mode,
				fixtureRootDir: preparedFixtureVariant.fixtureRootDir,
				env: resolveRuntimeEnvironmentForPort(runtimePort),
			});
			spawnedRuntimeCommand = spawnManagedCommand(runtimeCommandSpec);
			trackLongLivedRuntimeCommandForCleanup({
				spawnedCommand: mustGetRunningRuntimeCommand(),
			});

			try {
				await waitForHealthEndpoint({
					baseURL: runtimeBaseURL,
					spawnedCommand: mustGetRunningRuntimeCommand(),
				});
				return;
			} catch (error) {
				const currentRuntimeCommand = spawnedRuntimeCommand;
				spawnedRuntimeCommand = null;
				if (currentRuntimeCommand !== null) {
					untrackLongLivedRuntimeCommandForCleanup({
						spawnedCommand: currentRuntimeCommand,
					});
					await stopManagedCommand({
						spawnedCommand: currentRuntimeCommand,
					});
				}

				const hasAnotherPortRetryAvailable =
					runtimeStartupAttemptIndex <
					runtimeStartupPortRetryCount - 1;
				if (
					!hasAnotherPortRetryAvailable ||
					!isAddressInUseStartupError({ error })
				) {
					throw error;
				}

				runtimePort = await reserveOpenPort();
				runtimeBaseURL = `http://127.0.0.1:${runtimePort}`;
			}
		}

		throw new Error("runtime startup retries exhausted unexpectedly");
	}

	try {
		await startRuntimeProcess({
			runProdBuild: props.mode === "prod",
		});

		return {
			mode: props.mode,
			uiAdapter: props.uiAdapter,
			baseURL: runtimeBaseURL,
			frontendSourceDir: preparedFixtureVariant.frontendSourceDir,
			readRecentOutput: () =>
				formatRecentCommandOutput({
					spawnedCommand: mustGetRunningRuntimeCommand(),
				}),
			restartRuntime: async () => {
				const currentRuntimeCommand = mustGetRunningRuntimeCommand();
				untrackLongLivedRuntimeCommandForCleanup({
					spawnedCommand: currentRuntimeCommand,
				});
				await stopManagedCommand({
					spawnedCommand: currentRuntimeCommand,
				});
				spawnedRuntimeCommand = null;
				await startRuntimeProcess({
					runProdBuild: false,
				});
			},
			stop: async () => {
				const currentRuntimeCommand = spawnedRuntimeCommand;
				spawnedRuntimeCommand = null;
				if (currentRuntimeCommand !== null) {
					untrackLongLivedRuntimeCommandForCleanup({
						spawnedCommand: currentRuntimeCommand,
					});
					await stopManagedCommand({
						spawnedCommand: currentRuntimeCommand,
					});
				}
				await disposeFixtureVariantOnce();
			},
		};
	} catch (error) {
		if (spawnedRuntimeCommand !== null) {
			try {
				untrackLongLivedRuntimeCommandForCleanup({
					spawnedCommand: spawnedRuntimeCommand,
				});
				await stopManagedCommand({
					spawnedCommand: spawnedRuntimeCommand,
				});
			} catch {
				// Ignore cleanup errors so the original startup error is preserved.
			}
		}
		await disposeFixtureVariantOnce();
		throw error;
	}
}

/////////////////////////////////////////////////////////////////////
/////// Command Helpers
/////////////////////////////////////////////////////////////////////

/** Resolves the command used to launch the runtime for a given mode. */
function resolveRuntimeCommandSpec(props: {
	mode: E2ERunMode;
	fixtureRootDir: string;
	env: NodeJS.ProcessEnv;
}): CommandSpec {
	if (props.mode === "prod") {
		return {
			command: resolveProdBinaryPath({
				fixtureRootDir: props.fixtureRootDir,
			}),
			args: [],
			cwd: props.fixtureRootDir,
			env: props.env,
		};
	}

	return {
		command: "go",
		args: ["run", "./backend/cmd/build", "--dev"],
		cwd: props.fixtureRootDir,
		env: props.env,
	};
}

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

function trackLongLivedRuntimeCommandForCleanup(props: {
	spawnedCommand: SpawnedCommand;
}): void {
	installTrackedRuntimeCommandCleanupHooksIfNeeded();
	trackedLongLivedRuntimeCommands.add(props.spawnedCommand);
	void waitForChildExit({ child: props.spawnedCommand.child }).finally(() => {
		trackedLongLivedRuntimeCommands.delete(props.spawnedCommand);
	});
}

function untrackLongLivedRuntimeCommandForCleanup(props: {
	spawnedCommand: SpawnedCommand;
}): void {
	trackedLongLivedRuntimeCommands.delete(props.spawnedCommand);
}

function installTrackedRuntimeCommandCleanupHooksIfNeeded(): void {
	if (hasInstalledTrackedRuntimeCommandCleanupHooks) {
		return;
	}
	hasInstalledTrackedRuntimeCommandCleanupHooks = true;
	process.once("exit", () => {
		terminateAllTrackedRuntimeCommandsSync();
	});
	process.once("SIGINT", () => {
		terminateAllTrackedRuntimeCommandsSync();
		process.exit(130);
	});
	process.once("SIGTERM", () => {
		terminateAllTrackedRuntimeCommandsSync();
		process.exit(143);
	});
	process.once("SIGHUP", () => {
		terminateAllTrackedRuntimeCommandsSync();
		process.exit(129);
	});
}

function terminateAllTrackedRuntimeCommandsSync(): void {
	for (const spawnedCommand of trackedLongLivedRuntimeCommands) {
		try {
			terminateProcessGroup({
				child: spawnedCommand.child,
				signal: "SIGTERM",
			});
		} catch {
			// Process may have already exited.
		}
		try {
			terminateProcessGroup({
				child: spawnedCommand.child,
				signal: "SIGKILL",
			});
		} catch {
			// Process may have already exited.
		}
	}
}

/** Runs a one-shot command and throws on non-zero exit. */
async function runOneShotCommand(props: CommandSpec): Promise<void> {
	const spawnedCommand = spawnManagedCommand(props);
	trackLongLivedRuntimeCommandForCleanup({
		spawnedCommand,
	});

	let commandResult: {
		exitCode: number | null;
		signal: NodeJS.Signals | null;
	};
	try {
		commandResult = await waitForChildExit({
			child: spawnedCommand.child,
		});
	} finally {
		untrackLongLivedRuntimeCommandForCleanup({
			spawnedCommand,
		});
	}

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

/** Creates and configures an isolated fixture copy for the selected adapter. */
async function prepareFixtureVariantForAdapter(props: {
	uiAdapter: E2EUIAdapter;
	viteDefaultPort: number;
}): Promise<PreparedFixtureVariant> {
	const isolatedFixtureRootDir = await createIsolatedFixtureRootDirectory({
		uiAdapter: props.uiAdapter,
		viteDefaultPort: props.viteDefaultPort,
	});

	return {
		fixtureRootDir: isolatedFixtureRootDir,
		frontendSourceDir: path.join(isolatedFixtureRootDir, "frontend", "src"),
		dispose: async () => {
			await fs.promises.rm(isolatedFixtureRootDir, {
				recursive: true,
				force: true,
			});
		},
	};
}

/** Generates a temp fixture app with fully prepared dependencies. */
async function createIsolatedFixtureRootDirectory(props: {
	uiAdapter: E2EUIAdapter;
	viteDefaultPort: number;
}): Promise<string> {
	const sharedFixtureTemplateRootDir =
		await ensureSharedFixtureTemplateForAdapter({
			uiAdapter: props.uiAdapter,
		});

	const isolatedFixtureRootDir = await fs.promises.mkdtemp(
		isolatedFixtureRootDirectoryPrefix,
	);
	await copyDirectoryContents({
		sourceDirectoryPath: sharedFixtureTemplateRootDir,
		destinationDirectoryPath: isolatedFixtureRootDir,
	});
	await configureFixtureViteDefaultPort({
		fixtureRootDir: isolatedFixtureRootDir,
		viteDefaultPort: props.viteDefaultPort,
	});

	return isolatedFixtureRootDir;
}

/** Ensures a shared adapter template exists to avoid re-installing deps per lane. */
async function ensureSharedFixtureTemplateForAdapter(props: {
	uiAdapter: E2EUIAdapter;
}): Promise<string> {
	const sharedFixtureTemplateRootDir =
		buildSharedFixtureTemplateRootDirectoryPath({
			uiAdapter: props.uiAdapter,
		});

	await runOneShotCommand({
		command: "go",
		args: [
			"run",
			"./internal/cmd/e2e_fixture_gen",
			"--output-dir",
			sharedFixtureTemplateRootDir,
			"--repository-root",
			repositoryRootDir,
			"--ui-adapter",
			props.uiAdapter,
			"--reuse-if-present",
		],
		cwd: repositoryRootDir,
		env: {
			...process.env,
			GOCACHE: process.env.GOCACHE ?? "/tmp/go-build",
		},
	});

	return sharedFixtureTemplateRootDir;
}

function buildSharedFixtureTemplateRootDirectoryPath(props: {
	uiAdapter: E2EUIAdapter;
}): string {
	const repositoryRootHash = createHash("sha256")
		.update(repositoryRootDir)
		.digest("hex")
		.slice(0, 12);

	return `${sharedFixtureTemplateDirectoryPrefix}${repositoryRootHash}-${fixtureTemplateCacheVersion}-${props.uiAdapter}`;
}

async function copyDirectoryContents(props: {
	sourceDirectoryPath: string;
	destinationDirectoryPath: string;
}): Promise<void> {
	const sourceDirectoryEntryNames = await fs.promises.readdir(
		props.sourceDirectoryPath,
	);

	for (const sourceDirectoryEntryName of sourceDirectoryEntryNames) {
		const sourceEntryPath = path.join(
			props.sourceDirectoryPath,
			sourceDirectoryEntryName,
		);
		const destinationEntryPath = path.join(
			props.destinationDirectoryPath,
			sourceDirectoryEntryName,
		);
		await fs.promises.cp(sourceEntryPath, destinationEntryPath, {
			recursive: true,
			force: true,
			errorOnExist: false,
		});
	}
}

async function configureFixtureViteDefaultPort(props: {
	fixtureRootDir: string;
	viteDefaultPort: number;
}): Promise<void> {
	if (props.viteDefaultPort === 0) {
		return;
	}

	const waveConfigFilePath = path.join(
		props.fixtureRootDir,
		"backend",
		"wave.config.json",
	);
	const waveConfigFileContents = await fs.promises.readFile(
		waveConfigFilePath,
		"utf8",
	);
	const parsedWaveConfigObject = JSON.parse(
		waveConfigFileContents,
	) as unknown;
	if (!isRecordObject(parsedWaveConfigObject)) {
		throw new Error(
			`expected wave config object in ${waveConfigFilePath}, got non-object JSON`,
		);
	}

	const waveConfigObject: Record<string, unknown> = parsedWaveConfigObject;
	const rawViteConfigObject = waveConfigObject["Vite"];
	if (
		rawViteConfigObject !== undefined &&
		!isRecordObject(rawViteConfigObject)
	) {
		throw new Error(
			`expected wave config Vite object in ${waveConfigFilePath}, got non-object JSON`,
		);
	}

	const viteConfigObject: Record<string, unknown> =
		rawViteConfigObject === undefined ? {} : rawViteConfigObject;
	viteConfigObject["DefaultPort"] = props.viteDefaultPort;
	waveConfigObject["Vite"] = viteConfigObject;

	await fs.promises.writeFile(
		waveConfigFilePath,
		`${JSON.stringify(waveConfigObject, null, "\t")}\n`,
		"utf8",
	);
}

function isRecordObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
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
		throwIfRuntimeExitedBeforeReadiness({
			spawnedCommand: props.spawnedCommand,
		});

		const statusCode = await readHTTPStatusCode({ url: healthcheckURL });
		if (statusCode === 200) {
			const readinessIsStable =
				await didRuntimeReadinessStabilizeAfterHealthSuccess({
					healthcheckURL,
					spawnedCommand: props.spawnedCommand,
				});
			if (readinessIsStable) {
				return;
			}
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

async function didRuntimeReadinessStabilizeAfterHealthSuccess(input: {
	healthcheckURL: string;
	spawnedCommand: SpawnedCommand;
}): Promise<boolean> {
	const readinessStabilizationWindowMS = 900;
	const readinessStabilizationPollIntervalMS = 75;
	const stabilizationDeadlineMS = Date.now() + readinessStabilizationWindowMS;

	while (Date.now() < stabilizationDeadlineMS) {
		throwIfRuntimeExitedBeforeReadiness({
			spawnedCommand: input.spawnedCommand,
		});
		await sleep(readinessStabilizationPollIntervalMS);
	}

	const statusCodeAfterStabilizationWindow = await readHTTPStatusCode({
		url: input.healthcheckURL,
	});
	return statusCodeAfterStabilizationWindow === 200;
}

function throwIfRuntimeExitedBeforeReadiness(input: {
	spawnedCommand: SpawnedCommand;
}): void {
	if (
		input.spawnedCommand.child.exitCode === null &&
		input.spawnedCommand.child.signalCode === null
	) {
		return;
	}

	throw new Error(
		[
			`runtime exited before readiness: ${input.spawnedCommand.commandLine}`,
			`exit code: ${input.spawnedCommand.child.exitCode ?? "null"}`,
			`signal: ${input.spawnedCommand.child.signalCode ?? "null"}`,
			"recent output:",
			formatRecentCommandOutput({
				spawnedCommand: input.spawnedCommand,
			}),
		].join("\n"),
	);
}

function isAddressInUseStartupError(input: { error: unknown }): boolean {
	const errorMessage =
		input.error instanceof Error
			? input.error.message
			: String(input.error);
	return errorMessage.toLowerCase().includes("address already in use");
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

/**
 * Returns a deterministic per-lane port when available to avoid cross-lane
 * races in concurrent runs; falls back to any available loopback port.
 */
async function resolvePortForLane(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): Promise<number> {
	const preferredPort = resolvePreferredPortForLane({
		mode: props.mode,
		uiAdapter: props.uiAdapter,
	});
	const canUsePreferredPort = await canBindLoopbackPort({
		port: preferredPort,
	});
	if (canUsePreferredPort) {
		return preferredPort;
	}

	return reserveOpenPort();
}

/** Computes a stable non-overlapping port for each mode+adapter lane. */
function resolvePreferredPortForLane(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): number {
	return (
		58_100 +
		resolveLanePortOffset({
			mode: props.mode,
			uiAdapter: props.uiAdapter,
		})
	);
}

/** Computes a stable Vite default port for each mode+adapter lane. */
function resolvePreferredViteDefaultPortForLane(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): number {
	return (
		59_100 +
		resolveLanePortOffset({
			mode: props.mode,
			uiAdapter: props.uiAdapter,
		})
	);
}

/** Resolves a deterministic numeric lane offset shared across port families. */
function resolveLanePortOffset(props: {
	mode: E2ERunMode;
	uiAdapter: E2EUIAdapter;
}): number {
	const modeOffset = props.mode === "dev" ? 0 : 100;
	const uiAdapterOffset =
		props.uiAdapter === "solid" ? 1 : props.uiAdapter === "react" ? 2 : 3;
	return modeOffset + uiAdapterOffset;
}

/** Checks whether a specific loopback port can be bound right now. */
async function canBindLoopbackPort(props: { port: number }): Promise<boolean> {
	return await new Promise((resolve) => {
		const probeServer = createServer();
		let didSettle = false;
		const settle = (canBind: boolean) => {
			if (didSettle) {
				return;
			}
			didSettle = true;
			resolve(canBind);
		};

		probeServer.once("error", () => {
			settle(false);
		});
		probeServer.once("listening", () => {
			probeServer.close();
		});
		probeServer.once("close", () => {
			settle(true);
		});
		probeServer.unref();
		probeServer.listen(props.port, "127.0.0.1");
	});
}

/** Ensures required fixture files exist before runtime boot attempts. */
function assertFixtureFilesExist(): void {
	for (const absolutePath of [
		path.join(
			fixtureOverlayTemplatesRootDir,
			"common",
			"backend",
			"src",
			"router",
			"router.go.txt",
		),
		path.join(fixtureOverlayTemplatesRootDir, "common", "go.mod.tmpl"),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"common",
			"package.json.tmpl",
		),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"common",
			"frontend",
			"src",
			"routes",
			"core.vorma.routes.ts.txt",
		),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"react_like",
			"vite.config.ts.txt",
		),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"react_like",
			"frontend",
			"src",
			"vorma.entry.tsx.txt",
		),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"solid",
			"frontend",
			"src",
			"vorma.entry.tsx.txt",
		),
		path.join(
			fixtureOverlayTemplatesRootDir,
			"preact_overrides",
			"frontend",
			"src",
			"vorma.entry.tsx.txt",
		),
		path.join(
			repositoryRootDir,
			"internal",
			"cmd",
			"e2e_fixture_gen",
			"main.go",
		),
	]) {
		if (!fs.existsSync(absolutePath)) {
			throw new Error(`missing fixture file: ${absolutePath}`);
		}
	}
}

/** Resolves production runtime binary path created by wave build. */
function resolveProdBinaryPath(props: { fixtureRootDir: string }): string {
	const binaryPath =
		process.platform === "win32"
			? path.join(props.fixtureRootDir, "backend", "dist", "main.exe")
			: path.join(props.fixtureRootDir, "backend", "dist", "main");

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

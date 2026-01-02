#!/usr/bin/env node

import {
	cancel,
	confirm,
	intro,
	isCancel,
	log,
	select,
	spinner,
	text,
} from "@clack/prompts";
import { execSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

async function main() {
	const args = process.argv.slice(2);
	const isLocalTest = args.includes("--local-test");

	console.log();
	intro("Welcome to the Vorma new app creator!");

	// Check Go installation
	let goVersion = "";
	try {
		goVersion = execSync("go version", { encoding: "utf8" }).trim();
		const versionMatch = goVersion.match(/go(\d+)\.(\d+)/);
		if (versionMatch) {
			if (!versionMatch[1] || !versionMatch[2]) {
				cancel(
					"Go version not recognized. Please ensure Go is installed correctly. See https://go.dev/doc/install for installation instructions.",
				);
				process.exit(1);
			}
			const major = parseInt(versionMatch[1]);
			const minor = parseInt(versionMatch[2]);
			if (major < 1 || (major === 1 && minor < 24)) {
				cancel(
					"Go version 1.24 or higher is required. See https://go.dev/doc/install for installation instructions.",
				);
				process.exit(1);
			}
		}
	} catch {
		cancel(
			"Go is not installed. See https://go.dev/doc/install for installation instructions.",
		);
		process.exit(1);
	}

	// Check Node version
	const nodeVersion = process.version;
	const firstPart = nodeVersion.split(".")[0];
	if (!firstPart || firstPart.length < 2 || !firstPart.startsWith("v")) {
		cancel(
			"Node.js version not recognized. Please ensure Node.js is installed correctly.",
		);
		process.exit(1);
	}
	const nodeMajor = parseInt(firstPart.substring(1));
	if (nodeMajor < 22) {
		cancel("Node.js version 22.11 or higher is required");
		process.exit(1);
	}

	// Option to create a new directory at start if not already in desired location
	const createNewDir = await confirm({
		message: "Create a new directory for your Vorma app?",
		initialValue: true,
	});

	if (isCancel(createNewDir)) {
		cancel("Operation cancelled");
		process.exit(0);
	}

	let targetDir = process.cwd();

	if (createNewDir) {
		const dirName = await text({
			message: "Enter directory name:",
			validate: (value) => {
				if (!value || value.trim() === "")
					return "Directory name is required";
				// Check for invalid characters in directory name
				if (!/^[a-zA-Z0-9-_]+$/.test(value)) {
					return "Directory name can only contain letters, numbers, hyphens, and underscores";
				}
				// Check if directory already exists
				const proposedPath = path.join(process.cwd(), value);
				if (fs.existsSync(proposedPath)) {
					return `Directory "${value}" already exists`;
				}
				return undefined;
			},
		});

		if (isCancel(dirName)) {
			cancel("Operation cancelled");
			process.exit(0);
		}

		// Create the directory and change to it
		targetDir = path.join(process.cwd(), dirName as string);
		fs.mkdirSync(targetDir, { recursive: true });
		process.chdir(targetDir);
		log.success(`Created directory: ${dirName}`);
	}

	// Find go.mod and determine import path
	let goModPath: string | null = null;
	let moduleRoot: string | null = null;
	let moduleName: string | null = null;
	let createNewModule = false;

	// Search for go.mod
	let currentDir = process.cwd();
	while (currentDir !== path.dirname(currentDir)) {
		const modPath = path.join(currentDir, "go.mod");
		if (fs.existsSync(modPath)) {
			goModPath = modPath;
			moduleRoot = currentDir;
			break;
		}
		currentDir = path.dirname(currentDir);
	}

	// Parse module name if found
	if (goModPath) {
		const modContent = fs.readFileSync(goModPath, "utf8");
		const moduleMatch = modContent.match(/^module\s+(.+)$/m);
		if (moduleMatch && moduleMatch[1]) {
			moduleName = moduleMatch[1].trim();
		}

		// When module is found, give option to use it or create nested module
		const moduleChoice = await select({
			message: `Found parent Go module: ${moduleName}`,
			options: [
				{ value: "new", label: "Create new go.mod" },
				{ value: "use", label: "Use parent go.mod" },
			],
		});

		if (isCancel(moduleChoice)) {
			cancel("Operation cancelled");
			process.exit(0);
		}

		if (moduleChoice === "new") {
			createNewModule = true;
			// Reset module info since we're creating a new one
			goModPath = null;
			moduleRoot = null;
			moduleName = null;
		}
	} else {
		// No module found, we need to create one
		createNewModule = true;
	}

	// Handle go.mod initialization if needed
	if (createNewModule) {
		const modNameInput = await text({
			message:
				'Enter module name (e.g., "myapp" or "github.com/user/myapp"):',
			validate: (value) => {
				if (!value || value.trim() === "")
					return "Module name is required";
				return undefined;
			},
		});

		if (isCancel(modNameInput)) {
			cancel("Operation cancelled");
			process.exit(0);
		}

		moduleName = modNameInput as string;
		moduleRoot = process.cwd();

		const s = spinner();
		s.start("Initializing Go module");
		try {
			execSync(`go mod init ${moduleName}`, { cwd: moduleRoot });
			if (isLocalTest) {
				const vormaPath = path.resolve(__dirname, "../../../../../");
				execSync(
					`go mod edit -replace github.com/vormadev/vorma=${vormaPath}`,
					{ cwd: moduleRoot },
				);
				log.info("Using local Vorma code for testing");
			}
			s.stop("Go module initialized");
		} catch (error) {
			s.stop("Failed to initialize module");
			cancel(`Error: ${error}`);
			process.exit(1);
		}
	}

	// Check for underscore directories from module root to current directory
	const pathFromModuleRoot = moduleRoot
		? path.relative(moduleRoot, process.cwd())
		: "";
	const pathSegments = pathFromModuleRoot
		.split(path.sep)
		.filter((seg) => seg.length > 0);
	const underscoreSegments = pathSegments.filter((seg) =>
		seg.startsWith("_"),
	);
	if (underscoreSegments.length > 0) {
		cancel(
			`Cannot create Vorma app in a path containing directories that start with underscores:\n   ${underscoreSegments.join(", ")}\n` +
				`   Go ignores directories starting with underscores, which will cause build issues.`,
		);
		process.exit(1);
	}

	// Calculate import path automatically
	let importPath = moduleName!;
	if (moduleRoot !== process.cwd()) {
		const relativePath = path.relative(moduleRoot!, process.cwd());
		importPath = path.posix.join(
			moduleName!,
			...relativePath.split(path.sep),
		);
	}

	// Collect options
	const uiVariant = await select({
		message: "Choose frontend UI library:",
		options: [
			{ value: "react", label: "React" },
			{ value: "preact", label: "Preact" },
			{ value: "solid", label: "Solid" },
		],
	});

	if (isCancel(uiVariant)) {
		cancel("Operation cancelled");
		process.exit(0);
	}

	const packageManager = (await select({
		message: "Choose JS package manager:",
		options: [
			{ value: "npm", label: "npm" },
			{ value: "pnpm", label: "pnpm" },
			{ value: "yarn", label: "yarn" },
			{ value: "bun", label: "bun" },
		],
	})) as string;

	if (isCancel(packageManager)) {
		cancel("Operation cancelled");
		process.exit(0);
	}

	const deploymentTarget = await select({
		message: "Choose deployment target:",
		options: [
			{
				value: "docker",
				label: "Docker",
			},
			{
				value: "vercel",
				label: "Vercel",
			},
			{
				value: "none",
				label: "None (I'll figure it out myself)",
			},
		],
	});

	if (isCancel(deploymentTarget)) {
		cancel("Operation cancelled");
		process.exit(0);
	}

	// Ask about Tailwind CSS
	const includeTailwind = await confirm({
		message: "Include Tailwind CSS?",
		initialValue: false,
	});

	if (isCancel(includeTailwind)) {
		cancel("Operation cancelled");
		process.exit(0);
	}

	console.log();
	console.log("🛠️  Preparing bootstrapper...");

	// Create temporary directory
	const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "create-vorma-"));
	const bootstrapFile = path.join(tempDir, "main.go");

	// Extract Go version string (e.g., "go1.24.0")
	const goVersionMatch = goVersion.match(/go\d+\.\d+\.\d+/);
	const goVersionString = goVersionMatch ? goVersionMatch[0] : "";

	try {
		// Write bootstrap Go file
		const goCode = `package main

import "github.com/vormadev/vorma/bootstrap"

func main() {
	bootstrap.Init(bootstrap.Options{
		GoImportBase:     "${importPath}",
		UIVariant:        "${uiVariant}",
		JSPackageManager: "${packageManager}",
		DeploymentTarget: "${deploymentTarget}",
		IncludeTailwind:  ${includeTailwind},
		NodeMajorVersion: "${nodeMajor}",
		GoVersion:        "${goVersionString}",
		${createNewDir ? `CreatedInDir:     "${path.basename(targetDir)}",` : ""}
		ModuleRoot:       "${moduleRoot || process.cwd()}",
		CurrentDir:       "${process.cwd()}",
		HasParentModule:  ${moduleRoot !== null && moduleRoot !== process.cwd()},
	})
}
`;
		fs.writeFileSync(bootstrapFile, goCode);

		// Install Vorma dependency
		const packageJsonPath = path.join(__dirname, "../package.json");
		const packageJson = JSON.parse(
			fs.readFileSync(packageJsonPath, "utf8"),
		);
		const version = packageJson.version;
		const usingExistingModule = !createNewModule;
		const skipVormaGet = isLocalTest && usingExistingModule;
		if (!skipVormaGet) {
			execSync(`go get github.com/vormadev/vorma@v${version}`, {
				cwd: process.cwd(),
				stdio: "pipe",
			});
		}

		// Run bootstrap
		console.log("🛠️  Running bootstrapper...");
		try {
			execSync(`go run ${bootstrapFile}`, {
				cwd: process.cwd(),
				stdio: "inherit",
			});
		} catch (error) {
			console.error("Failed to create app");
			throw error;
		}
	} catch (error) {
		cancel(`Error: ${error}`);
		process.exit(1);
	} finally {
		// Clean up temp directory
		try {
			fs.rmSync(tempDir, { recursive: true, force: true });
		} catch {
			// Ignore cleanup errors
		}
	}
}

main().catch((error) => {
	console.error("Unexpected error:", error);
	process.exit(1);
});

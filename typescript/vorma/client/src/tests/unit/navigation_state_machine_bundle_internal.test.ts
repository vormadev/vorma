// @vitest-environment node
import { build } from "esbuild";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const DEBUG_ONLY_TOKENS = [
	"superseded_by_",
	"reused_by_",
	"prefetch_aborted_immediately",
] as const;

async function bundleNavigationStateMachineWithDevFlag(props: {
	isDev: boolean;
}): Promise<string> {
	const tempDir = await mkdtemp(
		join(tmpdir(), "vorma-navigation-state-machine-bundle-"),
	);
	const runtimeStateMachinePath = fileURLToPath(
		new URL("../../runtime.ts", import.meta.url),
	);
	const entryPath = join(tempDir, "entry.ts");

	try {
		await writeFile(
			entryPath,
			`import { createNavigationRuntimeStateMachine } from ${JSON.stringify(
				runtimeStateMachinePath,
			)};
const machine = createNavigationRuntimeStateMachine();
machine.dispatchTransitionEvent({
	type: "navigation_failed",
	targetUrl: "http://localhost:3000/bundle-check",
	entry: undefined,
	reason: "bundle_check"
});
console.log(machine.getDebugJournal().length);
`,
			"utf8",
		);

		const buildResult = await build({
			entryPoints: [entryPath],
			bundle: true,
			format: "esm",
			platform: "browser",
			minify: true,
			treeShaking: true,
			write: false,
			logLevel: "silent",
			define: {
				"import.meta.env.DEV": props.isDev ? "true" : "false",
			},
		});
		const output = buildResult.outputFiles?.[0]?.text;
		if (!output) {
			throw new Error("Expected bundled output text to be available.");
		}
		return output;
	} finally {
		await rm(tempDir, { recursive: true, force: true });
	}
}

describe("navigation runtime state machine production bundle gating", () => {
	it("excludes debug transition machinery from production bundles", async () => {
		const productionBundle = await bundleNavigationStateMachineWithDevFlag({
			isDev: false,
		});

		for (const token of DEBUG_ONLY_TOKENS) {
			expect(productionBundle).not.toContain(token);
		}
	});

	it("retains debug transition machinery in development bundles", async () => {
		const developmentBundle = await bundleNavigationStateMachineWithDevFlag(
			{
				isDev: true,
			},
		);

		expect(
			DEBUG_ONLY_TOKENS.some((token) =>
				developmentBundle.includes(token),
			),
		).toBe(true);
	});
});

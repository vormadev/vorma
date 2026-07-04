/*
Minimal jsdoc coverage gate for the TypeScript public API surface (jsdoc IS
the user documentation for TS, same standing as rust-doc's `deny(missing_docs)`
on the Rust side — see AGENTS.md's user-facing documentation strategy).

Walks every entry point tsdown actually bundles (imported directly from
`tsdown.config.ts`'s default export, so this check can never silently drift
from what is really published) except `vorma/__internal`
(`packages/vorma/core/_index.ts`) — the census's own ruling is that the `__`
prefix IS the stance: internal, unstable, undocumented, by name and
convention (`docs/maintainer/tickets/board-api-coverage/
PRESSURE_TEST_CENSUS.md`). For every other entry point, every exported
symbol is resolved past re-export aliases to its real declaration (so
documentation lives once, at the shared definition, exactly where an
adapter's own re-export of a core type expects it to) and must carry a
jsdoc comment, with two exemptions:

- A `__`-prefixed export name (the same internal-surface convention,
  applied per-symbol rather than per-package).
- A symbol whose real declaration lives in a generated `*.gen.ts` file —
  those files are never hand-edited, so no doc comment can be added to fix
  a finding here; see the generated-types docs finding in this packet's
  report for why TsGen does not carry Rust doc comments through, and the
  ticket filed against the generator. These are reported separately as
  ACKNOWLEDGED, not silently skipped, so a newly-generated undocumented
  export is still visible in the check's output rather than disappearing.

Entry points resolve `vorma/*` self-imports (e.g. `vorma/__internal`,
`vorma/kit/json`, used the same way an app would import the built package)
straight back to source via an explicit `paths` map, so this check runs
against what is actually being edited — never a stale `.dist` build — and
never needs `ts-build` to run first. The map's keys mirror
`packages/vorma/package.json`'s own `exports` map exactly; update both
together if an entry point is ever added, removed, or renamed.

No new dependency: runs on the `typescript` compiler API already a
devDependency, invoked directly via Node's native TypeScript support
(`node check_jsdoc_coverage.ts`, no build step, no `tsx`/`ts-node`).
*/

import ts from "typescript";
import tsdown_configs from "./tsdown.config.ts";

const INTERNAL_ENTRY_POINT = "./packages/vorma/core/_index.ts";

const SELF_IMPORT_SOURCE_PATHS: Record<string, string> = {
	"vorma/__internal": "./packages/vorma/core/_index.ts",
	"vorma/react": "./packages/vorma/ui/react/react.tsx",
	"vorma/preact": "./packages/vorma/ui/preact/preact.tsx",
	"vorma/solid": "./packages/vorma/ui/solid/solid.tsx",
	"vorma/kit/converters": "./packages/vorma/kit/converters/converters.ts",
	"vorma/kit/cookies": "./packages/vorma/kit/cookies/cookies.ts",
	"vorma/kit/csrf": "./packages/vorma/kit/csrf/csrf.ts",
	"vorma/kit/debounce": "./packages/vorma/kit/debounce/debounce.ts",
	"vorma/kit/fmt": "./packages/vorma/kit/fmt/fmt.ts",
	"vorma/kit/json": "./packages/vorma/kit/json/json.ts",
	"vorma/kit/listeners": "./packages/vorma/kit/listeners/listeners.ts",
	"vorma/kit/result": "./packages/vorma/kit/result/result.ts",
	"vorma/kit/theme": "./packages/vorma/kit/theme/theme.ts",
};

function to_absolute(relative: string): string {
	return new URL(relative, import.meta.url).pathname;
}

function entry_point_source_files(): string[] {
	return tsdown_configs
		.flatMap((config) =>
			Array.isArray(config.entry) ? config.entry : [config.entry],
		)
		.filter((entry): entry is string => typeof entry === "string")
		.filter((entry) => entry !== INTERNAL_ENTRY_POINT)
		.map(to_absolute);
}

function is_generated_file(file_name: string): boolean {
	return file_name.endsWith(".gen.ts") || file_name.endsWith(".gen.tsx");
}

type Finding = {
	entry_point: string;
	name: string;
	kind: string;
	file: string;
	line: number;
};

function undocumented_exports_for_entry_point(
	program: ts.Program,
	checker: ts.TypeChecker,
	entry_point: string,
): { missing: Finding[]; generated_acknowledged: Finding[] } {
	const source_file = program.getSourceFile(entry_point);
	if (!source_file) {
		throw new Error(`entry point not found by the program: ${entry_point}`);
	}
	const module_symbol = checker.getSymbolAtLocation(source_file);
	if (!module_symbol) {
		throw new Error(`entry point has no module symbol: ${entry_point}`);
	}

	const missing: Finding[] = [];
	const generated_acknowledged: Finding[] = [];

	for (const export_symbol of checker.getExportsOfModule(module_symbol)) {
		const name = export_symbol.getName();
		if (name.startsWith("__")) {
			continue;
		}
		const resolved =
			export_symbol.flags & ts.SymbolFlags.Alias
				? checker.getAliasedSymbol(export_symbol)
				: export_symbol;
		const declarations = resolved.getDeclarations() ?? [];
		if (declarations.length === 0) {
			continue;
		}
		const has_doc = declarations.some(
			(decl) => ts.getJSDocCommentsAndTags(decl).length > 0,
		);
		if (has_doc) {
			continue;
		}
		const decl = declarations[0]!;
		const decl_source_file = decl.getSourceFile();
		const { line } = decl_source_file.getLineAndCharacterOfPosition(decl.getStart());
		const finding: Finding = {
			entry_point,
			name,
			kind: ts.SyntaxKind[decl.kind],
			file: decl_source_file.fileName,
			line: line + 1,
		};
		if (is_generated_file(decl_source_file.fileName)) {
			generated_acknowledged.push(finding);
		} else {
			missing.push(finding);
		}
	}

	return { missing, generated_acknowledged };
}

function main(): void {
	const entry_points = entry_point_source_files();
	const paths: Record<string, string[]> = {};
	for (const [specifier, relative_source] of Object.entries(SELF_IMPORT_SOURCE_PATHS)) {
		paths[specifier] = [to_absolute(relative_source)];
	}

	const program = ts.createProgram({
		rootNames: entry_points,
		options: {
			target: ts.ScriptTarget.ES2022,
			module: ts.ModuleKind.ESNext,
			moduleResolution: ts.ModuleResolutionKind.Bundler,
			jsx: ts.JsxEmit.ReactJSX,
			skipLibCheck: true,
			strict: true,
			baseUrl: ".",
			paths,
		},
	});
	const checker = program.getTypeChecker();

	const all_missing: Finding[] = [];
	const all_generated_acknowledged: Finding[] = [];

	for (const entry_point of entry_points) {
		const { missing, generated_acknowledged } = undocumented_exports_for_entry_point(
			program,
			checker,
			entry_point,
		);
		all_missing.push(...missing);
		all_generated_acknowledged.push(...generated_acknowledged);
	}

	if (all_generated_acknowledged.length > 0) {
		console.log(
			`ACKNOWLEDGED (generated-file exports; TsGen does not carry Rust doc comments through — see the generator ticket): ${all_generated_acknowledged.length}`,
		);
		for (const finding of all_generated_acknowledged) {
			console.log(
				`  ${finding.name} (${finding.kind}) — ${finding.file}:${finding.line}, via ${finding.entry_point}`,
			);
		}
	}

	if (all_missing.length > 0) {
		console.error(`\nMissing jsdoc on ${all_missing.length} exported symbol(s):`);
		for (const finding of all_missing) {
			console.error(
				`  ${finding.name} (${finding.kind}) — ${finding.file}:${finding.line}, via ${finding.entry_point}`,
			);
		}
		process.exitCode = 1;
		return;
	}

	console.log(
		`jsdoc coverage: every non-generated, non-__-prefixed export across ${entry_points.length} entry points is documented.`,
	);
}

main();

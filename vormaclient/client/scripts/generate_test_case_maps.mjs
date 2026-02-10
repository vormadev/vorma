#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const clientRoot = path.resolve(__dirname, "..");
const srcRoot = path.join(clientRoot, "src");

const callRegex =
	/\b(?<fn>it|test)(?:\.(?<modifier>only|skip))?\s*\(\s*(?<quote>["'`])(?<title>(?:\\.|(?!\k<quote>)[\s\S])*?)\k<quote>\s*,/g;

function walk(dir) {
	const entries = fs.readdirSync(dir, { withFileTypes: true });
	const files = [];
	for (const entry of entries) {
		const fullPath = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			files.push(...walk(fullPath));
			continue;
		}
		if (entry.isFile() && entry.name.endsWith(".test.ts")) {
			files.push(fullPath);
		}
	}
	return files;
}

function normalizeTitle(title) {
	return title
		.trim()
		.toLowerCase()
		.replace(/^should\s+/i, "")
		.replace(/[`"'()[\]{}]/g, "")
		.replace(/[^a-z0-9]+/g, " ")
		.replace(/\s+/g, " ")
		.trim();
}

function toLineNumber(text, index) {
	return text.slice(0, index).split("\n").length;
}

function collectCases() {
	const testFiles = walk(srcRoot).sort((a, b) => a.localeCompare(b));
	const cases = [];

	for (const filePath of testFiles) {
		const relFromClient = path
			.relative(clientRoot, filePath)
			.replaceAll("\\", "/");
		const relFromSrc = path
			.relative(srcRoot, filePath)
			.replaceAll("\\", "/");
		const content = fs.readFileSync(filePath, "utf8");

		for (const match of content.matchAll(callRegex)) {
			const fn = match.groups?.fn ?? "it";
			const modifier = match.groups?.modifier ?? "";
			const title = (match.groups?.title ?? "")
				.replaceAll(/\s+/g, " ")
				.trim();
			const index = match.index ?? 0;
			const line = toLineNumber(content, index);
			const isShould = /^should\b/i.test(title);
			const isContract = relFromSrc.startsWith("contracts/");

			cases.push({
				file: relFromClient,
				line,
				fn,
				modifier,
				title,
				isShould,
				normalizedTitle: normalizeTitle(title),
				suiteType: isContract ? "contract" : "legacy",
			});
		}
	}

	return cases;
}

function main() {
	const allCases = collectCases();
	const generatedAt = new Date().toISOString();

	const legacyCases = allCases.filter(
		(entry) => entry.suiteType === "legacy",
	);
	const contractCases = allCases.filter(
		(entry) => entry.suiteType === "contract",
	);
	const legacyShouldCases = legacyCases.filter((entry) => entry.isShould);
	const contractShouldCases = contractCases.filter((entry) => entry.isShould);

	const jsonOutput = {
		generatedAt,
		summary: {
			legacyCases: legacyCases.length,
			legacyShouldCases: legacyShouldCases.length,
			contractCases: contractCases.length,
			contractShouldCases: contractShouldCases.length,
		},
		legacyCases,
		contractCases,
		legacyShouldCases,
		contractShouldCases,
	};

	fs.writeFileSync(
		path.join(clientRoot, "TEST_REWRITE_SHOULD_MAP.json"),
		`${JSON.stringify(jsonOutput, null, 2)}\n`,
	);

	console.log(
		`Generated TEST_REWRITE_SHOULD_MAP.json with ${legacyShouldCases.length} legacy should-cases and ${contractShouldCases.length} contract should-cases.`,
	);
}

main();

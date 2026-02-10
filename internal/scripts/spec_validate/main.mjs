#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { isDeepStrictEqual } from "node:util";

function parseArgs(argv) {
	const args = {};
	for (let i = 0; i < argv.length; i += 1) {
		const token = argv[i];
		if (!token.startsWith("--")) {
			continue;
		}
		const key = token.slice(2);
		const next = argv[i + 1];
		if (!next || next.startsWith("--")) {
			args[key] = true;
			continue;
		}
		args[key] = next;
		i += 1;
	}
	return args;
}

function usageAndExit() {
	console.error(
		"Usage: node internal/scripts/spec_validate/main.mjs --pkg <package-path> [--contract-schema <path>] [--matrix-schema <path>]",
	);
	process.exit(2);
}

function readJSON(filePath) {
	try {
		return JSON.parse(fs.readFileSync(filePath, "utf8"));
	} catch (err) {
		throw new Error(
			`Failed to read JSON from ${filePath}: ${String(err.message || err)}`,
		);
	}
}

function joinPath(parent, segment) {
	if (parent === "$") {
		if (typeof segment === "number") {
			return `$[${segment}]`;
		}
		return `$.${segment}`;
	}
	if (typeof segment === "number") {
		return `${parent}[${segment}]`;
	}
	return `${parent}.${segment}`;
}

function typeOfJSON(value) {
	if (Array.isArray(value)) {
		return "array";
	}
	if (value === null) {
		return "null";
	}
	if (Number.isInteger(value)) {
		return "integer";
	}
	return typeof value;
}

function matchesType(expectedType, value) {
	switch (expectedType) {
		case "object":
			return (
				value !== null &&
				typeof value === "object" &&
				!Array.isArray(value)
			);
		case "array":
			return Array.isArray(value);
		case "string":
			return typeof value === "string";
		case "number":
			return typeof value === "number" && Number.isFinite(value);
		case "integer":
			return Number.isInteger(value);
		case "boolean":
			return typeof value === "boolean";
		case "null":
			return value === null;
		default:
			return true;
	}
}

function resolveRef(rootSchema, ref) {
	if (typeof ref !== "string" || !ref.startsWith("#/")) {
		throw new Error(`Unsupported $ref: ${String(ref)}`);
	}
	const parts = ref
		.slice(2)
		.split("/")
		.map((p) => p.replace(/~1/g, "/").replace(/~0/g, "~"));

	let current = rootSchema;
	for (const part of parts) {
		if (
			current &&
			typeof current === "object" &&
			Object.hasOwn(current, part)
		) {
			current = current[part];
			continue;
		}
		throw new Error(`Unable to resolve $ref ${ref}`);
	}
	return current;
}

function validateAgainstSchema(schema, instance) {
	const errors = [];
	validateNode(schema, instance, "$", schema, errors);
	return errors;
}

function isValid(schema, instance, dataPath, rootSchema) {
	const tempErrors = [];
	validateNode(schema, instance, dataPath, rootSchema, tempErrors);
	return tempErrors.length === 0;
}

function validateNode(schema, instance, dataPath, rootSchema, errors) {
	if (!schema || typeof schema !== "object") {
		return;
	}

	if (Object.hasOwn(schema, "$ref")) {
		try {
			const resolved = resolveRef(rootSchema, schema.$ref);
			validateNode(resolved, instance, dataPath, rootSchema, errors);
		} catch (err) {
			errors.push(`${dataPath}: ${String(err.message || err)}`);
			return;
		}
	}

	if (Object.hasOwn(schema, "type")) {
		const expectedType = schema.type;
		if (!matchesType(expectedType, instance)) {
			errors.push(
				`${dataPath}: expected type ${expectedType}, got ${typeOfJSON(instance)}`,
			);
			return;
		}
	}

	if (
		Object.hasOwn(schema, "const") &&
		!isDeepStrictEqual(instance, schema.const)
	) {
		errors.push(
			`${dataPath}: expected const ${JSON.stringify(schema.const)}`,
		);
	}

	if (Array.isArray(schema.enum)) {
		const allowed = schema.enum.some((value) =>
			isDeepStrictEqual(value, instance),
		);
		if (!allowed) {
			errors.push(`${dataPath}: value is not in enum`);
		}
	}

	if (typeof schema.pattern === "string" && typeof instance === "string") {
		const pattern = new RegExp(schema.pattern);
		if (!pattern.test(instance)) {
			errors.push(
				`${dataPath}: string does not match pattern ${schema.pattern}`,
			);
		}
	}

	if (typeof schema.minLength === "number" && typeof instance === "string") {
		if (instance.length < schema.minLength) {
			errors.push(
				`${dataPath}: string length ${instance.length} is < minLength ${schema.minLength}`,
			);
		}
	}

	if (typeof schema.minimum === "number" && typeof instance === "number") {
		if (instance < schema.minimum) {
			errors.push(
				`${dataPath}: number ${instance} is < minimum ${schema.minimum}`,
			);
		}
	}

	if (typeof schema.minProperties === "number") {
		if (
			instance &&
			typeof instance === "object" &&
			!Array.isArray(instance)
		) {
			const count = Object.keys(instance).length;
			if (count < schema.minProperties) {
				errors.push(
					`${dataPath}: object has ${count} properties, minProperties is ${schema.minProperties}`,
				);
			}
		}
	}

	if (Array.isArray(schema.required)) {
		if (
			instance &&
			typeof instance === "object" &&
			!Array.isArray(instance)
		) {
			for (const prop of schema.required) {
				if (!Object.hasOwn(instance, prop)) {
					errors.push(
						`${joinPath(dataPath, prop)}: required property is missing`,
					);
				}
			}
		}
	}

	if (
		schema.properties &&
		instance &&
		typeof instance === "object" &&
		!Array.isArray(instance)
	) {
		for (const [prop, subSchema] of Object.entries(schema.properties)) {
			if (Object.hasOwn(instance, prop)) {
				validateNode(
					subSchema,
					instance[prop],
					joinPath(dataPath, prop),
					rootSchema,
					errors,
				);
			}
		}
	}

	if (
		schema.additionalProperties === false &&
		instance &&
		typeof instance === "object" &&
		!Array.isArray(instance)
	) {
		const declared = schema.properties
			? new Set(Object.keys(schema.properties))
			: new Set();
		for (const key of Object.keys(instance)) {
			if (!declared.has(key)) {
				errors.push(
					`${joinPath(dataPath, key)}: additional property is not allowed`,
				);
			}
		}
	}

	if (typeof schema.minItems === "number" && Array.isArray(instance)) {
		if (instance.length < schema.minItems) {
			errors.push(
				`${dataPath}: array length ${instance.length} is < minItems ${schema.minItems}`,
			);
		}
	}

	if (typeof schema.maxItems === "number" && Array.isArray(instance)) {
		if (instance.length > schema.maxItems) {
			errors.push(
				`${dataPath}: array length ${instance.length} is > maxItems ${schema.maxItems}`,
			);
		}
	}

	if (schema.uniqueItems === true && Array.isArray(instance)) {
		for (let i = 0; i < instance.length; i += 1) {
			for (let j = i + 1; j < instance.length; j += 1) {
				if (isDeepStrictEqual(instance[i], instance[j])) {
					errors.push(
						`${dataPath}: array items at ${i} and ${j} must be unique`,
					);
					break;
				}
			}
		}
	}

	if (schema.items && Array.isArray(instance)) {
		for (let i = 0; i < instance.length; i += 1) {
			validateNode(
				schema.items,
				instance[i],
				joinPath(dataPath, i),
				rootSchema,
				errors,
			);
		}
	}

	if (Array.isArray(schema.allOf)) {
		for (const subSchema of schema.allOf) {
			validateNode(subSchema, instance, dataPath, rootSchema, errors);
		}
	}

	if (Array.isArray(schema.oneOf)) {
		let validCount = 0;
		for (const subSchema of schema.oneOf) {
			if (isValid(subSchema, instance, dataPath, rootSchema)) {
				validCount += 1;
			}
		}
		if (validCount !== 1) {
			errors.push(
				`${dataPath}: oneOf matched ${validCount} branches, expected exactly 1`,
			);
		}
	}

	if (schema.if && typeof schema.if === "object") {
		const branchMatches = isValid(
			schema.if,
			instance,
			dataPath,
			rootSchema,
		);
		if (branchMatches) {
			if (schema.then) {
				validateNode(
					schema.then,
					instance,
					dataPath,
					rootSchema,
					errors,
				);
			}
		} else if (schema.else) {
			validateNode(schema.else, instance, dataPath, rootSchema, errors);
		}
	}

	if (schema.not && typeof schema.not === "object") {
		if (isValid(schema.not, instance, dataPath, rootSchema)) {
			errors.push(`${dataPath}: value matches disallowed 'not' schema`);
		}
	}
}

function toPosixPath(p) {
	return p.split(path.sep).join(path.posix.sep);
}

function run() {
	const args = parseArgs(process.argv.slice(2));
	if (args.help) {
		usageAndExit();
	}

	const pkg = args.pkg;
	const contractSchemaPath =
		args["contract-schema"] || "spec_templates/contract.schema.json";
	const matrixSchemaPath =
		args["matrix-schema"] || "spec_templates/test-matrix.schema.json";

	let contractPath = args.contract;
	let matrixPath = args.matrix;

	if (pkg) {
		contractPath = contractPath || path.join(pkg, "spec", "contract.json");
		matrixPath = matrixPath || path.join(pkg, "spec", "test-matrix.json");
	}

	if (!contractPath || !matrixPath) {
		usageAndExit();
	}

	const contractSchema = readJSON(contractSchemaPath);
	const matrixSchema = readJSON(matrixSchemaPath);
	const contractDoc = readJSON(contractPath);
	const matrixDoc = readJSON(matrixPath);

	const contractErrors = validateAgainstSchema(contractSchema, contractDoc);
	const matrixErrors = validateAgainstSchema(matrixSchema, matrixDoc);

	if (contractErrors.length === 0) {
		console.log(`[spec-validate] OK ${toPosixPath(contractPath)}`);
	} else {
		console.error(
			`[spec-validate] FAILED ${toPosixPath(contractPath)} (${contractErrors.length} errors)`,
		);
		for (const err of contractErrors) {
			console.error(`  - ${err}`);
		}
	}

	if (matrixErrors.length === 0) {
		console.log(`[spec-validate] OK ${toPosixPath(matrixPath)}`);
	} else {
		console.error(
			`[spec-validate] FAILED ${toPosixPath(matrixPath)} (${matrixErrors.length} errors)`,
		);
		for (const err of matrixErrors) {
			console.error(`  - ${err}`);
		}
	}

	if (contractErrors.length > 0 || matrixErrors.length > 0) {
		process.exit(1);
	}
}

run();

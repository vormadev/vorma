export type CSSVariableName = `--${string}`;
export type CSSVariableMap = Record<CSSVariableName, string | number>;
export type TokenPrimitive = string | number;

export type TokenReferences<T> = T extends TokenPrimitive
	? string
	: {
			readonly [K in keyof T]: TokenReferences<T[K]>;
		};

const css_variable_segment_pattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/////////////////////////////////////////////////////////////////////
/////// CSS Variable Contract
/////////////////////////////////////////////////////////////////////

// Token path segments are already contract strings, so we validate them instead of rewriting them.
function assert_css_variable_segment(segment: string, label: string): void {
	if (css_variable_segment_pattern.test(segment)) {
		return;
	}

	throw new Error(
		`Invalid CSS variable ${label} "${segment}". Use lowercase ASCII letters and digits separated by single hyphens.`,
	);
}

export function createCSSVariableName(
	variablePrefix: string,
	path: readonly string[],
): CSSVariableName {
	assert_css_variable_segment(variablePrefix, "prefix");

	for (const segment of path) {
		assert_css_variable_segment(segment, "path segment");
	}

	return `--${[variablePrefix, ...path].join("-")}` as CSSVariableName;
}

export function createCSSVariableReference(
	variablePrefix: string,
	path: readonly string[],
): string {
	return `var(${createCSSVariableName(variablePrefix, path)})`;
}

function is_token_primitive(value: unknown): value is TokenPrimitive {
	return typeof value === "string" || typeof value === "number";
}

function read_token_object(value: unknown): Record<string, unknown> {
	return value as Record<string, unknown>;
}

function create_token_variables_from_unknown(
	variable_prefix: string,
	tokens: unknown,
	path: readonly string[] = [],
): CSSVariableMap {
	const variables: CSSVariableMap = {};

	if (is_token_primitive(tokens)) {
		variables[createCSSVariableName(variable_prefix, path)] = tokens;
		return variables;
	}

	for (const [key, value] of Object.entries(read_token_object(tokens))) {
		Object.assign(
			variables,
			create_token_variables_from_unknown(variable_prefix, value, [...path, key]),
		);
	}

	return variables;
}

export function createTokenVariables<T extends object>(
	variablePrefix: string,
	tokens: T,
): CSSVariableMap {
	return create_token_variables_from_unknown(variablePrefix, tokens);
}

function create_token_references_from_unknown(
	variable_prefix: string,
	tokens: unknown,
	path: readonly string[] = [],
): unknown {
	if (is_token_primitive(tokens)) {
		return createCSSVariableReference(variable_prefix, path);
	}

	const references: Record<string, unknown> = Object.fromEntries(
		Object.entries(read_token_object(tokens)).map(([key, value]) => {
			return [
				key,
				create_token_references_from_unknown(variable_prefix, value, [
					...path,
					key,
				]),
			];
		}),
	);

	return references;
}

export function createTokenReferences<T extends object>(
	variablePrefix: string,
	tokens: T,
): TokenReferences<T> {
	return create_token_references_from_unknown(
		variablePrefix,
		tokens,
	) as TokenReferences<T>;
}

export function renderCSSVariables(selector: string, variables: CSSVariableMap): string {
	const declarations = Object.entries(variables)
		.map(([property, value]) => {
			return `\t${property}: ${value};`;
		})
		.join("\n");

	return `${selector} {\n${declarations}\n}`;
}

import { formatOKLCH, is_oklch } from "./color.ts";
import { createCSSVariableReference } from "./css.ts";
import { compact_object } from "./object.ts";
import {
	token_spec_fields,
	token_spec_kind,
	token_spec_kinds,
	type RawTokenValue,
	type ResolvedTokenTree,
	type TokenSpec,
} from "./types.ts";

/////////////////////////////////////////////////////////////////////
/////// Token Resolution
/////////////////////////////////////////////////////////////////////

export function resolve_token_spec(
	spec: TokenSpec,
	variable_prefix: string,
): string | number {
	if (typeof spec === "string" || typeof spec === "number") {
		return spec;
	}

	if (is_oklch(spec)) {
		return formatOKLCH(spec);
	}

	if (spec[token_spec_kind] === token_spec_kinds.ref) {
		return createCSSVariableReference(
			variable_prefix,
			spec[token_spec_fields.path],
		);
	}

	if (spec[token_spec_kind] === token_spec_kinds.template) {
		return spec[token_spec_fields.parts]
			.map((part) => {
				return resolve_token_spec(part, variable_prefix);
			})
			.join("");
	}

	return `color-mix(in ${
		spec[token_spec_fields.mix].space
	}, ${resolve_token_spec(
		spec[token_spec_fields.mix].color,
		variable_prefix,
	)} ${spec[token_spec_fields.mix].amount}, ${resolve_token_spec(
		spec[token_spec_fields.mix].with,
		variable_prefix,
	)})`;
}

export function is_token_spec(value: unknown): value is TokenSpec {
	if (typeof value === "string" || typeof value === "number") {
		return true;
	}

	if (typeof value !== "object" || value === null) {
		return false;
	}

	return is_oklch(value) || token_spec_kind in value;
}

export function resolve_token_tree<T>(
	input: T,
	variable_prefix: string,
): ResolvedTokenTree<T> {
	if (is_token_spec(input)) {
		return resolve_token_spec(
			input,
			variable_prefix,
		) as ResolvedTokenTree<T>;
	}

	return compact_object(
		Object.fromEntries(
			Object.entries(input as Record<string, unknown>).map(
				([key, value]) => {
					if (value === undefined) {
						return [key, undefined];
					}

					return [key, resolve_token_tree(value, variable_prefix)];
				},
			),
		),
	) as ResolvedTokenTree<T>;
}

export function resolve_raw_value(value: RawTokenValue): string | number {
	if (typeof value === "string" || typeof value === "number") {
		return value;
	}

	return formatOKLCH(value);
}

export function resolve_raw_record(
	input: Record<string, RawTokenValue> | undefined,
): Record<string, string | number> | undefined {
	if (!input) {
		return undefined;
	}

	return Object.fromEntries(
		Object.entries(input).map(([key, value]) => {
			return [key, resolve_raw_value(value)];
		}),
	);
}

export function resolve_spec_record<T>(
	input: Record<string, T> | undefined,
	resolve_value: (value: T) => string | number,
): Record<string, string | number> | undefined {
	if (!input) {
		return undefined;
	}

	return Object.fromEntries(
		Object.entries(input).map(([key, value]) => {
			return [key, resolve_value(value)];
		}),
	);
}

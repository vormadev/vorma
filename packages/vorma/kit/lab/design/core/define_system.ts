import type {
	NonEmptyReadonlyArray,
	PrimitiveInput,
	PrimitiveTokensForInput,
	RecipeStyleDefinitions,
	ResolvedTokenTree,
	SemanticInput,
	TokenMixSpec,
	TokenPath,
	TokenReferenceSpec,
	TokenSpec,
	TokenTemplateSpec,
} from "./types.ts";
import {
	primitive_token_keys,
	token_groups,
	token_spec_fields,
	token_spec_kind,
	token_spec_kinds,
} from "./types.ts";

type AppendPath<TPath extends TokenPath, TKey extends string> = readonly [
	...TPath,
	TKey,
] extends TokenPath
	? readonly [...TPath, TKey]
	: never;

type NoInferFromUsage<T> = [T][T extends unknown ? 0 : never];

export type TokenRefTree<T, TPath extends TokenPath> = T extends string | number
	? TokenReferenceSpec<TPath>
	: T extends object
		? {
				readonly [K in keyof T & string]: TokenRefTree<
					Exclude<T[K], undefined>,
					AppendPath<TPath, K>
				>;
			}
		: never;

type PrimitiveRefTree<TPrimitive> = TokenRefTree<
	PrimitiveTokensForInput<Exclude<TPrimitive, undefined>>,
	readonly [typeof token_groups.primitive]
>;

type SemanticRefTree<TSemantic> = TokenRefTree<
	ResolvedTokenTree<Exclude<TSemantic, undefined>[keyof Exclude<TSemantic, undefined>]>,
	readonly [typeof token_groups.semantic]
>;

type TextStyleName<TPrimitive> = TPrimitive extends {
	typography?: {
		[primitive_token_keys.text_style]?: infer TTextStyle;
	};
}
	? keyof TTextStyle & string
	: never;

type FocusRingName<TPrimitive> = TPrimitive extends {
	focus?: {
		ring?: infer TRing;
	};
}
	? keyof TRing & string
	: never;

type TextStyleReferences<TName extends string> = Partial<{
	[primitive_token_keys.font_family]: TokenReferenceSpec<
		readonly [
			typeof token_groups.primitive,
			"typography",
			typeof primitive_token_keys.text_style,
			TName,
			typeof primitive_token_keys.font_family,
		]
	>;
	[primitive_token_keys.font_size]: TokenReferenceSpec<
		readonly [
			typeof token_groups.primitive,
			"typography",
			typeof primitive_token_keys.text_style,
			TName,
			typeof primitive_token_keys.font_size,
		]
	>;
	[primitive_token_keys.font_weight]: TokenReferenceSpec<
		readonly [
			typeof token_groups.primitive,
			"typography",
			typeof primitive_token_keys.text_style,
			TName,
			typeof primitive_token_keys.font_weight,
		]
	>;
	[primitive_token_keys.letter_spacing]: TokenReferenceSpec<
		readonly [
			typeof token_groups.primitive,
			"typography",
			typeof primitive_token_keys.text_style,
			TName,
			typeof primitive_token_keys.letter_spacing,
		]
	>;
	[primitive_token_keys.line_height]: TokenReferenceSpec<
		readonly [
			typeof token_groups.primitive,
			"typography",
			typeof primitive_token_keys.text_style,
			TName,
			typeof primitive_token_keys.line_height,
		]
	>;
}>;

export type StyleAuthoringHelpers<TPrimitive> = {
	template(strings: TemplateStringsArray, ...values: TokenSpec[]): TokenTemplateSpec;
	focusRing<TName extends FocusRingName<TPrimitive>>(
		name: TName,
		options?: {
			offset?: TokenSpec;
		},
	): {
		outline: TokenReferenceSpec<
			readonly [typeof token_groups.primitive, "focus", "ring", TName]
		>;
		"outline-offset"?: TokenSpec;
	};
	outline(
		outline:
			| TokenSpec
			| {
					color: TokenSpec;
					style: TokenSpec;
					width: TokenSpec;
			  },
		options?: {
			offset?: TokenSpec;
		},
	): {
		outline?: TokenSpec;
		"outline-color"?: TokenSpec;
		"outline-offset"?: TokenSpec;
		"outline-style"?: TokenSpec;
		"outline-width"?: TokenSpec;
	};
	textStyle<TName extends TextStyleName<TPrimitive>>(
		name: TName,
	): TextStyleReferences<TName>;
};

export type SemanticAuthoringContext<TPrimitive> = {
	mix: typeof tokenMix;
	ref: {
		primitive: PrimitiveRefTree<TPrimitive>;
	};
	style: StyleAuthoringHelpers<TPrimitive>;
};

export type RecipeAuthoringContext<TPrimitive, TSemantic> = {
	mix: typeof tokenMix;
	ref: {
		primitive: PrimitiveRefTree<TPrimitive>;
		semantic: SemanticRefTree<TSemantic>;
	};
	style: StyleAuthoringHelpers<TPrimitive>;
};

/////////////////////////////////////////////////////////////////////
/////// Authoring Specs
/////////////////////////////////////////////////////////////////////

// Ref proxies let authors write token paths naturally while preserving the path as data for the compiler.
export function tokenMix(
	input: TokenMixSpec[typeof token_spec_fields.mix],
): TokenMixSpec {
	return {
		[token_spec_kind]: token_spec_kinds.mix,
		[token_spec_fields.mix]: input,
	};
}

function create_token_ref_proxy(path: TokenPath): unknown {
	const handler: ProxyHandler<object> = {
		get(_target, property): unknown {
			if (property === token_spec_kind) {
				return token_spec_kinds.ref;
			}

			if (property === token_spec_fields.path) {
				return path;
			}

			if (typeof property !== "string") {
				return undefined;
			}

			return create_token_ref_proxy([...path, property] as TokenPath);
		},
		has(_target, property): boolean {
			return property === token_spec_kind || property === token_spec_fields.path;
		},
	};

	return new Proxy({}, handler);
}

function create_style_authoring_helpers<TPrimitive>(
	primitive: PrimitiveRefTree<TPrimitive>,
	primitive_input?: TPrimitive,
): StyleAuthoringHelpers<TPrimitive> {
	const token = primitive as unknown as {
		focus: {
			ring: Record<string, TokenReferenceSpec<TokenPath>>;
		};
		typography: {
			[primitive_token_keys.text_style]: Record<
				string,
				{
					[primitive_token_keys.font_family]: TokenReferenceSpec<TokenPath>;
					[primitive_token_keys.font_size]: TokenReferenceSpec<TokenPath>;
					[primitive_token_keys.font_weight]: TokenReferenceSpec<TokenPath>;
					[primitive_token_keys.letter_spacing]: TokenReferenceSpec<TokenPath>;
					[primitive_token_keys.line_height]: TokenReferenceSpec<TokenPath>;
				}
			>;
		};
	};

	return {
		template(strings, ...values): TokenTemplateSpec {
			return {
				[token_spec_kind]: token_spec_kinds.template,
				[token_spec_fields.parts]: strings.flatMap(
					(string, index): TokenSpec[] => {
						const value = values[index];

						if (value === undefined) {
							return [string];
						}

						return [string, value];
					},
				),
			};
		},
		focusRing(name, options) {
			return {
				outline: token.focus.ring[name] as TokenReferenceSpec<
					readonly [typeof token_groups.primitive, "focus", "ring", typeof name]
				>,
				"outline-offset": options?.offset,
			};
		},
		outline(outline, options) {
			if (
				typeof outline === "object" &&
				outline !== null &&
				!(token_spec_kind in outline) &&
				"color" in outline &&
				"style" in outline &&
				"width" in outline
			) {
				return {
					"outline-color": outline.color,
					"outline-offset": options?.offset,
					"outline-style": outline.style,
					"outline-width": outline.width,
				};
			}

			return {
				outline,
				"outline-offset": options?.offset,
			};
		},
		textStyle(name) {
			const text_style = token.typography[primitive_token_keys.text_style][name]!;
			const typography =
				typeof primitive_input === "object" && primitive_input !== null
					? (
							primitive_input as {
								typography?: {
									family?: Record<string, unknown>;
									[primitive_token_keys.letter_spacing]?: Record<
										string,
										unknown
									>;
									[primitive_token_keys.line_height]?: Record<
										string,
										unknown
									>;
									size?: { step: Record<string, unknown> };
									[primitive_token_keys.text_style]?: Record<
										string,
										{
											family: string;
											letterSpacing: string;
											lineHeight: string;
											size: string;
											weight: string;
										}
									>;
									weight?: Record<string, unknown>;
								};
							}
						).typography
					: undefined;
			const text_style_input =
				typography?.[primitive_token_keys.text_style]?.[name];
			const letter_spacing = typography?.[primitive_token_keys.letter_spacing];
			const line_height = typography?.[primitive_token_keys.line_height];

			return {
				...(text_style_input &&
				typography?.family &&
				text_style_input.family in typography.family
					? {
							[primitive_token_keys.font_family]: text_style[
								primitive_token_keys.font_family
							] as TextStyleReferences<
								typeof name
							>[typeof primitive_token_keys.font_family],
						}
					: {}),
				...(text_style_input &&
				typography?.size &&
				text_style_input.size in typography.size.step
					? {
							[primitive_token_keys.font_size]: text_style[
								primitive_token_keys.font_size
							] as TextStyleReferences<
								typeof name
							>[typeof primitive_token_keys.font_size],
						}
					: {}),
				...(text_style_input &&
				typography?.weight &&
				text_style_input.weight in typography.weight
					? {
							[primitive_token_keys.font_weight]: text_style[
								primitive_token_keys.font_weight
							] as TextStyleReferences<
								typeof name
							>[typeof primitive_token_keys.font_weight],
						}
					: {}),
				...(text_style_input &&
				letter_spacing &&
				text_style_input.letterSpacing in letter_spacing
					? {
							[primitive_token_keys.letter_spacing]: text_style[
								primitive_token_keys.letter_spacing
							] as TextStyleReferences<
								typeof name
							>[typeof primitive_token_keys.letter_spacing],
						}
					: {}),
				...(text_style_input &&
				line_height &&
				text_style_input.lineHeight in line_height
					? {
							[primitive_token_keys.line_height]: text_style[
								primitive_token_keys.line_height
							] as TextStyleReferences<
								typeof name
							>[typeof primitive_token_keys.line_height],
						}
					: {}),
			};
		},
	};
}

export function defineSystem<
	const TModes extends NonEmptyReadonlyArray<string>,
	const TColorSource extends string,
	const TFontFamily extends string,
	const TLetterSpacing extends string,
	const TLineHeight extends string,
	const TFontSize extends string,
	const TFontWeight extends string,
	const TPrimitive extends PrimitiveInput<
		TModes[number],
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	>,
	const TSemantic extends Partial<Record<TModes[number], object>>,
>(system: {
	css: {
		variablePrefix: string;
	};
	modes: TModes;
	primitive: TPrimitive;
	semantic: (context: SemanticAuthoringContext<TPrimitive>) => TSemantic;
}): {
	css: {
		variablePrefix: string;
	};
	modes: TModes;
	primitive: TPrimitive;
	semantic: TSemantic & Partial<Record<TModes[number], SemanticInput>>;
};

export function defineSystem<
	const TModes extends NonEmptyReadonlyArray<string>,
	const TColorSource extends string,
	const TFontFamily extends string,
	const TLetterSpacing extends string,
	const TLineHeight extends string,
	const TFontSize extends string,
	const TFontWeight extends string,
	const TPrimitive extends PrimitiveInput<
		TModes[number],
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	> = PrimitiveInput<
		TModes[number],
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	>,
	const TSemantic extends Partial<Record<TModes[number], object>> = {},
>(system: {
	css: {
		variablePrefix: string;
	};
	modes: TModes;
	primitive?: TPrimitive;
	semantic?: (context: SemanticAuthoringContext<TPrimitive>) => TSemantic;
}): {
	css: {
		variablePrefix: string;
	};
	modes: TModes;
	primitive?: TPrimitive;
	semantic?: TSemantic & Partial<Record<TModes[number], SemanticInput>>;
} {
	const primitive = create_token_ref_proxy([
		token_groups.primitive,
	]) as PrimitiveRefTree<TPrimitive>;
	const style = create_style_authoring_helpers(primitive, system.primitive);
	const semantic_input = system.semantic?.({
		mix: tokenMix,
		ref: {
			primitive,
		},
		style,
	});

	return {
		css: system.css,
		modes: system.modes,
		...(system.primitive
			? {
					primitive: system.primitive,
				}
			: {}),
		...(semantic_input
			? {
					semantic: semantic_input as TSemantic &
						Partial<Record<TModes[number], SemanticInput>>,
				}
			: {}),
	};
}

export function defineRecipeStyles<
	const TPrimitive,
	const TSemantic extends Partial<Record<string, object>>,
	const TRecipes extends RecipeStyleDefinitions,
>(
	system: {
		primitive?: TPrimitive;
		semantic?: TSemantic;
	},
	recipe: (
		context: RecipeAuthoringContext<TPrimitive, NoInferFromUsage<TSemantic>>,
	) => TRecipes,
): TRecipes {
	const primitive = create_token_ref_proxy([
		token_groups.primitive,
	]) as PrimitiveRefTree<TPrimitive>;
	const semantic = create_token_ref_proxy([
		token_groups.semantic,
	]) as SemanticRefTree<TSemantic>;
	const style = create_style_authoring_helpers(primitive, system.primitive);

	return recipe({
		mix: tokenMix,
		ref: {
			primitive,
			semantic,
		},
		style,
	}) as TRecipes;
}

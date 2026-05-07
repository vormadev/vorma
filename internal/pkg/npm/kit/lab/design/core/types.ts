import type { OKLCH, Palette, PaletteCurve } from "./color.ts";
import type { CSSVariableMap, TokenReferences } from "./css.ts";
import type {
	RecipeCompoundVariantInput,
	RecipeConditionMap,
	RecipeInput,
	RecipeSlotInput,
	RecipeSlotMapInput,
	RecipeVariantSlotMapInput,
	RecipeVariantsInput,
} from "./recipe/types.ts";

const token_spec_symbol_key = "@vorma/kit/lab/design/core/token-spec";

/////////////////////////////////////////////////////////////////////
/////// Contract Strings
/////////////////////////////////////////////////////////////////////

export const token_groups = {
	primitive: "primitive",
	recipe: "recipe",
	semantic: "semantic",
} as const;

export const primitive_token_keys = {
	aspect_ratio: "aspect-ratio",
	data_visualization: "data-visualization",
	font_family: "font-family",
	font_size: "font-size",
	font_weight: "font-weight",
	letter_spacing: "letter-spacing",
	line_height: "line-height",
	stroke_width: "stroke-width",
	text_style: "text-style",
} as const;

export const token_spec_fields = {
	mix: "mix",
	parts: "parts",
	path: "path",
} as const;

export const token_spec_kinds = {
	mix: "mix",
	ref: "ref",
	template: "template",
} as const;

export type ModeName = string;
export type NonEmptyReadonlyArray<T> = readonly [T, ...T[]];
export type TokenPath = readonly [string, ...string[]];
export type LengthUnit = "px" | "rem";
export type RawTokenValue = string | number | OKLCH;
export const token_spec_kind = Symbol.for(token_spec_symbol_key);
export type TokenSpecKind = typeof token_spec_kind;

export type TokenReferenceSpec<Path extends TokenPath = TokenPath> = {
	readonly [token_spec_kind]: typeof token_spec_kinds.ref;
	readonly [token_spec_fields.path]: Path;
};

export type TokenMixSpec = {
	readonly [token_spec_kind]: typeof token_spec_kinds.mix;
	readonly [token_spec_fields.mix]: {
		amount: string;
		color: TokenSpec;
		space: string;
		with: TokenSpec;
	};
};

export type TokenTemplateSpec = {
	readonly [token_spec_kind]: typeof token_spec_kinds.template;
	readonly [token_spec_fields.parts]: readonly TokenSpec[];
};

export type TokenSpec =
	| string
	| number
	| OKLCH
	| TokenReferenceSpec
	| TokenMixSpec
	| TokenTemplateSpec;

export type TokenDeclarationValue = TokenSpec | undefined;
export type TokenDeclarationBlock<TProperty extends string = string> = {
	readonly [property in TProperty]?: TokenDeclarationValue;
};
export type TokenSlotMap<TSlot extends string = string> = {
	readonly [slot in TSlot]?: TokenDeclarationBlock;
};
export type TokenVariantMap<TVariant extends string = string> = {
	readonly [variant in TVariant]?: TokenGroupInput;
};
export type TokenContract<TShape extends object> = TShape;
export type TokenContractReferences<TShape extends object> = TokenReferences<
	TokenGroupOutput<TShape>
>;

export type LengthScaleInput<TKey extends string = string> = {
	baseFontSizePx: number;
	basePx: number;
	multiplier: Record<TKey, number>;
	unit: LengthUnit;
};

export type ModularScaleInput<TKey extends string = string> = {
	baseFontSizePx: number;
	basePx: number;
	ratio: number;
	step: Record<TKey, number>;
	unit: LengthUnit;
};

export type PaletteInput<
	TMode extends string = string,
	TColorSource extends string = string,
	TPaletteStep extends string = string,
> = {
	chromaLimit: number;
	curve: Record<
		TMode,
		{
			chroma: PaletteCurve<TPaletteStep>;
			lightness: PaletteCurve<TPaletteStep>;
		}
	>;
	source: TColorSource;
};

export type TextStyleInput<
	TFontFamily extends string = string,
	TLetterSpacing extends string = string,
	TLineHeight extends string = string,
	TFontSize extends string = string,
	TFontWeight extends string = string,
> = {
	family: TFontFamily;
	letterSpacing: TLetterSpacing;
	lineHeight: TLineHeight;
	size: TFontSize;
	weight: TFontWeight;
};

export type PrimitiveTextStyleTokens = Partial<{
	[primitive_token_keys.font_family]: string;
	[primitive_token_keys.font_size]: string;
	[primitive_token_keys.font_weight]: string;
	[primitive_token_keys.letter_spacing]: string;
	[primitive_token_keys.line_height]: string;
}>;

export type BorderCompositeInput = {
	color: TokenSpec;
	style: TokenSpec;
	width: TokenSpec;
};

export type ShadowLayerInput = {
	blur: TokenSpec;
	color: TokenSpec;
	spread?: TokenSpec;
	x: TokenSpec;
	y: TokenSpec;
};

export type ShadowCompositeInput =
	| ShadowLayerInput
	| readonly ShadowLayerInput[];

export type TransitionInput = {
	delay?: TokenSpec;
	duration: TokenSpec;
	easing: TokenSpec;
	property: TokenSpec;
};

export type TransitionCompositeInput =
	| TransitionInput
	| readonly TransitionInput[];

export type TokenGroupInput = {
	readonly [key: string]: TokenSpec | TokenGroupInput | undefined;
};

export type TokenGroupOutput<T> = T extends TokenSpec
	? string | number
	: {
			[K in keyof T as undefined extends T[K]
				? never
				: K]: TokenGroupOutput<Exclude<T[K], undefined>>;
		};

export type PrimitiveInput<
	TMode extends string = string,
	TColorSource extends string = string,
	TFontFamily extends string = string,
	TLetterSpacing extends string = string,
	TLineHeight extends string = string,
	TFontSize extends string = string,
	TFontWeight extends string = string,
> = {
	border?: {
		radius?: LengthScaleInput;
		shorthand?: Record<string, BorderCompositeInput>;
		style?: Record<string, RawTokenValue>;
		width?: LengthScaleInput;
	};
	color?: {
		palette?: Record<string, PaletteInput<TMode, TColorSource>>;
		source: Record<TColorSource, OKLCH>;
	};
	content?: {
		"aspect-ratio"?: Record<string, RawTokenValue>;
		measure?: Record<string, RawTokenValue>;
	};
	"data-visualization"?: {
		color?: Record<string, RawTokenValue>;
		shape?: Record<string, RawTokenValue>;
		stroke?: Record<string, RawTokenValue>;
	};
	dimension?: {
		"aspect-ratio"?: Record<string, RawTokenValue>;
		measure?: Record<string, RawTokenValue>;
		size?: Record<string, LengthScaleInput>;
		space?: LengthScaleInput;
	};
	effect?: {
		blur?: Record<string, RawTokenValue>;
		opacity?: Record<string, RawTokenValue>;
		shadow?: Record<string, RawTokenValue | ShadowCompositeInput>;
	};
	focus?: {
		ring?: Record<string, RawTokenValue | BorderCompositeInput>;
	};
	icon?: {
		asset?: Record<string, RawTokenValue>;
		size?: LengthScaleInput;
		"stroke-width"?: LengthScaleInput;
	};
	layout?: {
		breakpoint?: Record<string, RawTokenValue>;
		container?: Record<string, RawTokenValue>;
		grid?: Record<string, RawTokenValue>;
		layer?: Record<string, RawTokenValue>;
	};
	motion?: {
		distance?: Record<string, RawTokenValue>;
		duration?: Record<string, RawTokenValue>;
		easing?: Record<string, RawTokenValue>;
		transition?: Record<string, RawTokenValue | TransitionCompositeInput>;
	};
	outline?: {
		offset?: LengthScaleInput;
		shorthand?: Record<string, BorderCompositeInput>;
		style?: Record<string, RawTokenValue>;
		width?: LengthScaleInput;
	};
	typography?: {
		family?: Record<TFontFamily, RawTokenValue>;
		"letter-spacing"?: Record<TLetterSpacing, RawTokenValue>;
		"line-height"?: Record<TLineHeight, RawTokenValue>;
		size?: ModularScaleInput<TFontSize>;
		"text-style"?: Record<
			string,
			TextStyleInput<
				TFontFamily,
				TLetterSpacing,
				TLineHeight,
				TFontSize,
				TFontWeight
			>
		>;
		weight?: Record<TFontWeight, RawTokenValue>;
	};
};

export type SemanticInput = {
	border?: TokenGroupInput;
	color?: TokenGroupInput;
	"data-visualization"?: TokenGroupInput;
	elevation?: TokenGroupInput;
	focus?: TokenGroupInput;
	icon?: TokenGroupInput;
	layout?: TokenGroupInput;
	outline?: TokenGroupInput;
	state?: TokenGroupInput;
	surface?: TokenGroupInput;
	text?: TokenGroupInput;
	tone?: TokenGroupInput;
	typography?: TokenGroupInput;
	readonly [category: string]: TokenGroupInput | undefined;
};

export type RecipeStyleDeclarationValue =
	| RecipeStyleDeclarationInput
	| TokenDeclarationValue;
export type RecipeStyleDeclarationInput = {
	readonly [property: string]: RecipeStyleDeclarationValue;
};
export type RecipeStyleDefinition = RecipeInput<
	string,
	string,
	RecipeStyleDeclarationInput
>;
export type RecipeStyleDefinitions = Record<string, RecipeStyleDefinition>;

type ResolvedRecipeStyleConditionMap<TConditions> =
	TConditions extends RecipeConditionMap<string, RecipeStyleDeclarationInput>
		? {
				readonly [K in keyof TConditions]: ResolvedTokenTree<
					Exclude<TConditions[K], undefined>
				>;
			}
		: never;

type ResolvedRecipeStyleSlotBase<
	TSlot extends RecipeSlotInput<string, RecipeStyleDeclarationInput>,
> = "base" extends keyof TSlot
	? TSlot extends {
			readonly base?: infer TBase;
		}
		? {
				readonly base?: ResolvedTokenTree<Exclude<TBase, undefined>>;
			}
		: never
	: {};

type ResolvedRecipeStyleSlotConditions<
	TSlot extends RecipeSlotInput<string, RecipeStyleDeclarationInput>,
> = "conditions" extends keyof TSlot
	? TSlot extends {
			readonly conditions?: infer TConditions;
		}
		? {
				readonly conditions?: ResolvedRecipeStyleConditionMap<
					Exclude<TConditions, undefined>
				>;
			}
		: never
	: {};

type ResolvedRecipeStyleSlotInput<
	TSlot extends RecipeSlotInput<string, RecipeStyleDeclarationInput>,
> = ResolvedRecipeStyleSlotBase<TSlot> &
	ResolvedRecipeStyleSlotConditions<TSlot>;

type ResolvedRecipeStyleSlotMapInput<
	TSlots extends
		| RecipeSlotMapInput<string, string, RecipeStyleDeclarationInput>
		| RecipeVariantSlotMapInput<
				string,
				string,
				RecipeStyleDeclarationInput
		  >,
> = {
	readonly [K in keyof TSlots]: Exclude<
		TSlots[K],
		undefined
	> extends RecipeSlotInput<string, RecipeStyleDeclarationInput>
		? ResolvedRecipeStyleSlotInput<Exclude<TSlots[K], undefined>>
		: never;
};

type ResolvedRecipeStyleVariantsInput<
	TVariants extends RecipeVariantsInput<
		string,
		string,
		RecipeStyleDeclarationInput
	>,
> = {
	readonly [TGroup in keyof TVariants]: {
		readonly [TValue in keyof TVariants[TGroup]]: TVariants[TGroup][TValue] extends RecipeVariantSlotMapInput<
			string,
			string,
			RecipeStyleDeclarationInput
		>
			? ResolvedRecipeStyleSlotMapInput<TVariants[TGroup][TValue]>
			: never;
	};
};

type ResolvedRecipeStyleOptionalVariants<TVariants> =
	Exclude<TVariants, undefined> extends RecipeVariantsInput<
		string,
		string,
		RecipeStyleDeclarationInput
	>
		? ResolvedRecipeStyleVariantsInput<Exclude<TVariants, undefined>>
		: never;

type ResolvedRecipeStyleCompoundVariantInput<
	TCompoundVariant extends RecipeCompoundVariantInput<
		string,
		string,
		RecipeStyleDeclarationInput
	>,
> = {
	readonly slots: ResolvedRecipeStyleSlotMapInput<TCompoundVariant["slots"]>;
	readonly variants: TCompoundVariant["variants"];
};

type ResolvedRecipeStyleCompoundVariantsInput<TCompoundVariants> =
	TCompoundVariants extends readonly RecipeCompoundVariantInput<
		string,
		string,
		RecipeStyleDeclarationInput
	>[]
		? {
				readonly [K in keyof TCompoundVariants]: TCompoundVariants[K] extends RecipeCompoundVariantInput<
					string,
					string,
					RecipeStyleDeclarationInput
				>
					? ResolvedRecipeStyleCompoundVariantInput<
							TCompoundVariants[K]
						>
					: never;
			}
		: never;

export type ResolvedRecipeStyleDefinition<
	TDefinition extends RecipeStyleDefinition,
> = ("compoundVariants" extends keyof TDefinition
	? TDefinition extends {
			readonly compoundVariants?: infer TCompoundVariants;
		}
		? {
				readonly compoundVariants?: ResolvedRecipeStyleCompoundVariantsInput<
					Exclude<TCompoundVariants, undefined>
				>;
			}
		: never
	: {}) &
	("defaultVariants" extends keyof TDefinition
		? TDefinition extends {
				readonly defaultVariants?: infer TDefaultVariants;
			}
			? {
					readonly defaultVariants?: TDefaultVariants;
				}
			: never
		: {}) & {
		readonly slots: ResolvedRecipeStyleSlotMapInput<TDefinition["slots"]>;
	} & ("variants" extends keyof TDefinition
		? TDefinition extends {
				readonly variants?: infer TVariants;
			}
			? {
					readonly variants?: ResolvedRecipeStyleOptionalVariants<TVariants>;
				}
			: never
		: {});

export type ResolvedRecipeStyleDefinitions<
	TDefinitions extends RecipeStyleDefinitions,
> = {
	readonly [K in keyof TDefinitions]: ResolvedRecipeStyleDefinition<
		TDefinitions[K]
	>;
};

export type SystemInput<
	TModes extends NonEmptyReadonlyArray<string> =
		NonEmptyReadonlyArray<string>,
	TColorSource extends string = string,
	TFontFamily extends string = string,
	TLetterSpacing extends string = string,
	TLineHeight extends string = string,
	TFontSize extends string = string,
	TFontWeight extends string = string,
> = {
	css: {
		variablePrefix: string;
	};
	modes: TModes;
	primitive?: PrimitiveInput<
		TModes[number],
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	>;
	semantic?: Partial<Record<TModes[number], SemanticInput>>;
};

export type ResolvedTokenTree<T> = T extends TokenSpec
	? string | number
	: {
			[K in keyof T as undefined extends T[K]
				? never
				: K]: ResolvedTokenTree<Exclude<T[K], undefined>>;
		};

export type PrimitiveTokens = {
	border?: {
		radius?: Record<string, string>;
		shorthand?: Record<string, string | number>;
		style?: Record<string, string | number>;
		width?: Record<string, string>;
	};
	color?: {
		palette?: Record<string, Palette>;
		source: Record<string, string>;
	};
	content?: {
		"aspect-ratio"?: Record<string, string | number>;
		measure?: Record<string, string | number>;
	};
	"data-visualization"?: {
		color?: Record<string, string | number>;
		shape?: Record<string, string | number>;
		stroke?: Record<string, string | number>;
	};
	dimension?: {
		"aspect-ratio"?: Record<string, string | number>;
		measure?: Record<string, string | number>;
		size?: Record<string, Record<string, string>>;
		space?: Record<string, string>;
	};
	effect?: {
		blur?: Record<string, string | number>;
		opacity?: Record<string, string | number>;
		shadow?: Record<string, string | number>;
	};
	focus?: {
		ring?: Record<string, string | number>;
	};
	icon?: {
		asset?: Record<string, string | number>;
		size?: Record<string, string>;
		"stroke-width"?: Record<string, string>;
	};
	layout?: {
		breakpoint?: Record<string, string | number>;
		container?: Record<string, string | number>;
		grid?: Record<string, string | number>;
		layer?: Record<string, string | number>;
	};
	motion?: {
		distance?: Record<string, string | number>;
		duration?: Record<string, string | number>;
		easing?: Record<string, string | number>;
		transition?: Record<string, string | number>;
	};
	outline?: {
		offset?: Record<string, string>;
		shorthand?: Record<string, string | number>;
		style?: Record<string, string | number>;
		width?: Record<string, string>;
	};
	typography?: {
		family?: Record<string, string | number>;
		"letter-spacing"?: Record<string, string | number>;
		"line-height"?: Record<string, string | number>;
		size?: Record<string, string>;
		"text-style"?: Record<string, PrimitiveTextStyleTokens>;
		weight?: Record<string, string | number>;
	};
};

export type SystemTokens = {
	primitive?: PrimitiveTokens;
	semantic?: ResolvedTokenTree<SemanticInput>;
};

export type PrimitiveTokensForInput<T> = T extends undefined
	? never
	: T extends LengthScaleInput<infer TKey>
		? Record<TKey, string>
		: T extends ModularScaleInput<infer TKey>
			? Record<TKey, string>
			: T extends PaletteInput
				? Palette
				: T extends TextStyleInput
					? PrimitiveTextStyleTokens
					: T extends
								| RawTokenValue
								| BorderCompositeInput
								| ShadowCompositeInput
								| TransitionCompositeInput
						? string | number
						: T extends object
							? {
									[K in keyof T as undefined extends T[K]
										? never
										: K]: PrimitiveTokensForInput<
										Exclude<T[K], undefined>
									>;
								}
							: never;

export type SystemTokensForInput<TInput extends SystemInput> = {
	[K in keyof {
		primitive: TInput["primitive"];
		semantic: TInput["semantic"];
	} as undefined extends {
		primitive: TInput["primitive"];
		semantic: TInput["semantic"];
	}[K]
		? never
		: K]: K extends "primitive"
		? PrimitiveTokensForInput<Exclude<TInput["primitive"], undefined>>
		: K extends "semantic"
			? ResolvedTokenTree<
					Exclude<TInput["semantic"], undefined>[keyof Exclude<
						TInput["semantic"],
						undefined
					>]
				>
			: never;
};

export type SystemMetadataForInput<TInput extends SystemInput> =
	Exclude<TInput["primitive"], undefined> extends {
		layout?: {
			breakpoint?: infer TBreakpoint;
		};
	}
		? TBreakpoint extends Record<string, RawTokenValue>
			? GeneratedSystemMetadata<keyof TBreakpoint & string>
			: GeneratedSystemMetadata<never>
		: GeneratedSystemMetadata<never>;

export type AnyGeneratedSystemMetadata =
	| GeneratedSystemMetadata<never>
	| GeneratedSystemMetadata<string>;

export type GeneratedSystem<
	TMode extends string = string,
	TTokenReferences = TokenReferences<SystemTokens>,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = {
	metadata: TMetadata;
	modes: Record<TMode, GeneratedSystemMode>;
	variablePrefix: string;
	token: TTokenReferences;
};

export type GeneratedSystemMode = {
	variables: CSSVariableMap;
};

export type GeneratedSystemMetadata<TBreakpoint extends string = never> = [
	TBreakpoint,
] extends [never]
	? {
			breakpoint?: undefined;
		}
	: {
			breakpoint: Record<TBreakpoint, string | number>;
		};

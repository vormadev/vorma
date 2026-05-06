import { create_primitive_tokens } from "./create_primitive.ts";
import {
	createTokenReferences,
	createTokenVariables,
	type TokenReferences,
} from "./css.ts";
import { compact_object, merge_token_shapes } from "./object.ts";
import type { RecipeStyle } from "./recipe/style.ts";
import type {
	RecipeCompoundVariantInput,
	RecipeInput,
	RecipeSlotInput,
	RecipeSlotMapInput,
	RecipeStateMap,
	RecipeVariantSlotMapInput,
	RecipeVariantsInput,
} from "./recipe/types.ts";
import { resolve_raw_record, resolve_token_tree } from "./resolve_token.ts";
import {
	token_groups,
	type AnyGeneratedSystemMetadata,
	type GeneratedSystem,
	type GeneratedSystemMode,
	type ModeName,
	type NonEmptyReadonlyArray,
	type RecipeStyleDeclarationInput,
	type RecipeStyleDefinitions,
	type ResolvedRecipeStyleDefinitions,
	type SystemInput,
	type SystemMetadataForInput,
	type SystemTokens,
	type SystemTokensForInput,
} from "./types.ts";

/////////////////////////////////////////////////////////////////////
/////// System Compilation
/////////////////////////////////////////////////////////////////////

function create_tokens<
	TModes extends NonEmptyReadonlyArray<ModeName>,
	TColorSource extends string,
>(
	input: SystemInput<TModes, TColorSource>,
	mode: TModes[number],
): SystemTokens {
	return compact_object({
		primitive: create_primitive_tokens(input, mode),
		semantic: input.semantic?.[mode]
			? resolve_token_tree(input.semantic[mode], input.css.variablePrefix)
			: undefined,
	});
}

function create_mode<
	TModes extends NonEmptyReadonlyArray<ModeName>,
	TColorSource extends string,
>(
	input: SystemInput<TModes, TColorSource>,
	mode: TModes[number],
): GeneratedSystemMode {
	const tokens = create_tokens(input, mode);

	return {
		variables: createTokenVariables(input.css.variablePrefix, tokens),
	};
}

function create_token_reference_source<
	TModes extends NonEmptyReadonlyArray<ModeName>,
	TColorSource extends string,
>(input: SystemInput<TModes, TColorSource>): SystemTokens {
	return input.modes.reduce<unknown>((tokens, mode) => {
		return merge_token_shapes(tokens, create_tokens(input, mode));
	}, undefined) as SystemTokens;
}

function create_metadata<TInput extends SystemInput>(
	input: TInput,
): SystemMetadataForInput<TInput> {
	return compact_object({
		breakpoint: resolve_raw_record(input.primitive?.layout?.breakpoint),
	}) as SystemMetadataForInput<TInput>;
}

export type SingleRecipeStyleInput<TRecipes extends object> =
	keyof TRecipes extends infer TKey
		? [TKey] extends [never]
			? never
			: TKey extends keyof TRecipes
				? Pick<TRecipes, TKey> &
						Partial<Record<Exclude<keyof TRecipes, TKey>, never>>
				: never
		: never;

export function createSystem<
	const TModes extends NonEmptyReadonlyArray<string>,
	const TColorSource extends string,
	const TFontFamily extends string,
	const TLetterSpacing extends string,
	const TLineHeight extends string,
	const TFontSize extends string,
	const TFontWeight extends string,
	const TInput extends SystemInput<
		TModes,
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	>,
>(
	input: SystemInput<
		TModes,
		TColorSource,
		TFontFamily,
		TLetterSpacing,
		TLineHeight,
		TFontSize,
		TFontWeight
	> &
		TInput,
): GeneratedSystem<
	TModes[number],
	TokenReferences<SystemTokensForInput<TInput>>,
	SystemMetadataForInput<TInput>
> {
	const token_reference_source = create_token_reference_source(input);
	const generated_modes = Object.fromEntries(
		input.modes.map((mode) => {
			return [mode, create_mode(input, mode)];
		}),
	) as Record<TModes[number], GeneratedSystemMode>;

	return {
		metadata: create_metadata(input) as SystemMetadataForInput<TInput>,
		modes: generated_modes,
		variablePrefix: input.css.variablePrefix,
		token: createTokenReferences(
			input.css.variablePrefix,
			token_reference_source,
		) as TokenReferences<SystemTokensForInput<TInput>>,
	};
}

export type RecipeStyleReferences<TRecipes extends RecipeStyleDefinitions> = {
	[token_groups.recipe]: ResolvedRecipeStyleDefinitions<TRecipes>;
};

function resolve_recipe_style(
	style: RecipeStyleDeclarationInput | undefined,
	variable_prefix: string,
): RecipeStyle | undefined {
	return style
		? (resolve_token_tree(style, variable_prefix) as RecipeStyle)
		: undefined;
}

function resolve_recipe_state_map(
	states: RecipeStateMap<string, RecipeStyleDeclarationInput> | undefined,
	variable_prefix: string,
): RecipeStateMap<string, RecipeStyle> | undefined {
	if (!states) {
		return undefined;
	}

	return Object.fromEntries(
		Object.entries(states).map(([state_name, style]) => {
			return [state_name, resolve_recipe_style(style, variable_prefix)];
		}),
	) as RecipeStateMap<string, RecipeStyle>;
}

function resolve_recipe_slot_input(
	slot: RecipeSlotInput<string, RecipeStyleDeclarationInput>,
	variable_prefix: string,
): RecipeSlotInput<string, RecipeStyle> {
	return compact_object({
		base: resolve_recipe_style(slot.base, variable_prefix),
		states: resolve_recipe_state_map(slot.states, variable_prefix),
	});
}

function resolve_recipe_slot_map(
	slots:
		| RecipeSlotMapInput<string, string, RecipeStyleDeclarationInput>
		| RecipeVariantSlotMapInput<
				string,
				string,
				RecipeStyleDeclarationInput
		  >,
	variable_prefix: string,
): RecipeSlotMapInput<string, string, RecipeStyle> {
	return Object.fromEntries(
		Object.entries(slots).flatMap(([slot_name, slot]) => {
			if (!slot) {
				return [];
			}

			return [
				[slot_name, resolve_recipe_slot_input(slot, variable_prefix)],
			];
		}),
	) as RecipeSlotMapInput<string, string, RecipeStyle>;
}

function resolve_recipe_variants(
	variants:
		| RecipeVariantsInput<string, string, RecipeStyleDeclarationInput>
		| undefined,
	variable_prefix: string,
): RecipeVariantsInput<string, string, RecipeStyle> | undefined {
	if (!variants) {
		return undefined;
	}

	return Object.fromEntries(
		Object.entries(variants).map(([variant_name, variant_group]) => {
			return [
				variant_name,
				Object.fromEntries(
					Object.entries(variant_group).map(
						([variant_value, slots]) => {
							return [
								variant_value,
								resolve_recipe_slot_map(slots, variable_prefix),
							];
						},
					),
				),
			];
		}),
	) as RecipeVariantsInput<string, string, RecipeStyle>;
}

function resolve_recipe_compound_variants(
	compound_variants:
		| readonly RecipeCompoundVariantInput<
				string,
				string,
				RecipeStyleDeclarationInput
		  >[]
		| undefined,
	variable_prefix: string,
):
	| readonly RecipeCompoundVariantInput<string, string, RecipeStyle>[]
	| undefined {
	return compound_variants?.map((compound_variant) => {
		return {
			slots: resolve_recipe_slot_map(
				compound_variant.slots,
				variable_prefix,
			),
			variants: compound_variant.variants,
		};
	});
}

function resolve_recipe_definition(
	definition: RecipeInput<string, string, RecipeStyleDeclarationInput>,
	variable_prefix: string,
): RecipeInput<string, string, RecipeStyle> {
	return compact_object({
		compoundVariants: resolve_recipe_compound_variants(
			definition.compoundVariants,
			variable_prefix,
		),
		defaultVariants: definition.defaultVariants,
		slots: resolve_recipe_slot_map(definition.slots, variable_prefix),
		variants: resolve_recipe_variants(definition.variants, variable_prefix),
	}) as RecipeInput<string, string, RecipeStyle>;
}

function resolve_recipe_definitions<TRecipes extends RecipeStyleDefinitions>(
	recipes: TRecipes,
	variable_prefix: string,
): ResolvedRecipeStyleDefinitions<TRecipes> {
	return Object.fromEntries(
		Object.entries(recipes).map(([recipe_name, definition]) => {
			return [
				recipe_name,
				resolve_recipe_definition(definition, variable_prefix),
			];
		}),
	) as ResolvedRecipeStyleDefinitions<TRecipes>;
}

export function createRecipeStyleSystem<
	const TSystem extends GeneratedSystem<
		string,
		unknown,
		AnyGeneratedSystemMetadata
	>,
	const TRecipes extends RecipeStyleDefinitions,
>(
	system: TSystem,
	recipes: TRecipes & SingleRecipeStyleInput<TRecipes>,
): GeneratedSystem<
	TSystem extends GeneratedSystem<
		infer TMode,
		unknown,
		AnyGeneratedSystemMetadata
	>
		? TMode
		: never,
	RecipeStyleReferences<TRecipes>,
	TSystem extends GeneratedSystem<string, unknown, infer TMetadata>
		? TMetadata
		: AnyGeneratedSystemMetadata
> {
	const tokens = {
		[token_groups.recipe]: resolve_recipe_definitions(
			recipes,
			system.variablePrefix,
		),
	} as RecipeStyleReferences<TRecipes>;
	const generated_modes = Object.fromEntries(
		Object.keys(system.modes).map((mode) => {
			return [mode, { variables: {} }];
		}),
	) as TSystem["modes"];

	return {
		metadata: system.metadata as TSystem extends GeneratedSystem<
			string,
			unknown,
			infer TMetadata
		>
			? TMetadata
			: AnyGeneratedSystemMetadata,
		modes: generated_modes,
		variablePrefix: system.variablePrefix,
		token: tokens,
	};
}

import type { RecipeStyle } from "./style.ts";
import { mergeRecipeStyles } from "./style.ts";
import type {
	CreatedRecipe,
	RecipeCompoundVariantInput,
	RecipeConditionMap,
	RecipeConditionName,
	RecipeInput,
	RecipeSlotInput,
	RecipeVariantGroupName,
	RecipeVariantPropsFor,
	RecipeVariantSelection,
	ResolvedRecipe,
	ResolvedRecipeSlot,
} from "./types.ts";

type AnyRecipeInput = RecipeInput<string, string, RecipeStyle>;
type AnyRecipeSlotInput = RecipeSlotInput<string, RecipeStyle>;
type AnyRecipeConditionMap = RecipeConditionMap<string, RecipeStyle>;
type AnyRecipeVariantSelection = Record<string, string>;
type AnyRecipeCompoundVariantInput = RecipeCompoundVariantInput<
	string,
	string,
	RecipeStyle
>;

/////////////////////////////////////////////////////////////////////
/////// Recipe Resolution
/////////////////////////////////////////////////////////////////////

function merge_variant_selection(
	default_variants: RecipeVariantSelection | undefined,
	variants: RecipeVariantSelection | undefined,
): AnyRecipeVariantSelection {
	const selection: Record<string, string> = {};

	for (const [key, value] of Object.entries(default_variants ?? {})) {
		if (value !== undefined) {
			selection[key] = value;
		}
	}

	for (const [key, value] of Object.entries(variants ?? {})) {
		if (value !== undefined) {
			selection[key] = value;
		}
	}

	return selection;
}

function matches_compound_variant(
	compound_variant: AnyRecipeCompoundVariantInput,
	selection: AnyRecipeVariantSelection,
): boolean {
	return Object.entries(compound_variant.variants).every(([key, value]) => {
		return selection[key] === value;
	});
}

function merge_recipe_condition_maps(
	condition_maps: readonly (AnyRecipeConditionMap | undefined)[],
): AnyRecipeConditionMap {
	const condition_names = new Set<string>();

	for (const conditions of condition_maps) {
		for (const condition_name of Object.keys(conditions ?? {})) {
			condition_names.add(condition_name);
		}
	}

	return Object.fromEntries(
		Array.from(condition_names).map((condition_name) => {
			const condition_styles = condition_maps.map((conditions) => {
				return conditions?.[condition_name];
			});

			return [condition_name, mergeRecipeStyles(...condition_styles)];
		}),
	);
}

function merge_recipe_slot_inputs(
	slot_inputs: readonly (AnyRecipeSlotInput | undefined)[],
): ResolvedRecipeSlot<string, RecipeStyle> {
	const base_styles = slot_inputs.map((slot_input) => {
		return slot_input?.base;
	});
	const condition_maps = slot_inputs.map((slot_input) => {
		return slot_input?.conditions;
	});

	return {
		base: mergeRecipeStyles(...base_styles),
		conditions: merge_recipe_condition_maps(condition_maps),
	};
}

function read_variant_slot_inputs(input: {
	recipe: AnyRecipeInput;
	selection: AnyRecipeVariantSelection;
	slot: string;
}): AnyRecipeSlotInput[] {
	return Object.entries(input.recipe.variants ?? {}).flatMap(
		([variant_name, variant_group]) => {
			const variant_value = input.selection[variant_name];
			if (variant_value === undefined) {
				return [];
			}

			const variant_slot = variant_group[variant_value]?.[input.slot];

			return variant_slot ? [variant_slot] : [];
		},
	);
}

function read_compound_variant_slot_inputs(input: {
	recipe: AnyRecipeInput;
	selection: AnyRecipeVariantSelection;
	slot: string;
}): AnyRecipeSlotInput[] {
	return (input.recipe.compoundVariants ?? []).flatMap((compound_variant) => {
		if (!matches_compound_variant(compound_variant, input.selection)) {
			return [];
		}

		const compound_slot = compound_variant.slots[input.slot];

		return compound_slot ? [compound_slot] : [];
	});
}

function resolve_recipe_slot(input: {
	recipe: AnyRecipeInput;
	selection: AnyRecipeVariantSelection;
	slot: string;
}): ResolvedRecipeSlot<string, RecipeStyle> {
	return merge_recipe_slot_inputs([
		input.recipe.slots[input.slot],
		...read_variant_slot_inputs(input),
		...read_compound_variant_slot_inputs(input),
	]);
}

export function resolveRecipe<
	const TRecipe extends AnyRecipeInput,
	const TGroups extends RecipeVariantGroupName<TRecipe>,
>(
	recipe: TRecipe,
	variants?: RecipeVariantPropsFor<TRecipe, TGroups>,
): ResolvedRecipe<TRecipe, RecipeConditionName<TRecipe>, RecipeStyle> {
	const selection = merge_variant_selection(
		recipe.defaultVariants,
		variants as AnyRecipeVariantSelection | undefined,
	);

	return {
		slots: Object.fromEntries(
			Object.keys(recipe.slots).map((slot) => {
				return [
					slot,
					resolve_recipe_slot({
						recipe,
						selection,
						slot,
					}),
				];
			}),
		),
	} as ResolvedRecipe<TRecipe, RecipeConditionName<TRecipe>, RecipeStyle>;
}

export function createRecipe<const TRecipe extends AnyRecipeInput>(
	recipe: TRecipe,
): CreatedRecipe<TRecipe> {
	return {
		input: recipe,
		resolve<const TGroups extends RecipeVariantGroupName<TRecipe>>(
			variants?: RecipeVariantPropsFor<TRecipe, TGroups>,
		): ResolvedRecipe<TRecipe, RecipeConditionName<TRecipe>, RecipeStyle> {
			return resolveRecipe(recipe, variants);
		},
	} as CreatedRecipe<TRecipe>;
}

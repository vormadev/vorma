import type { RecipeStyle } from "./style.ts";

export type RecipeStateMap<
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Partial<Record<TState, TStyle>>;

export type RecipeSlotInput<
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	base?: TStyle;
	states?: RecipeStateMap<TState, TStyle>;
};

export type RecipeSlotMapInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<TSlot, RecipeSlotInput<TState, TStyle>>;

export type RecipeVariantSlotMapInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Partial<Record<TSlot, RecipeSlotInput<TState, TStyle>>>;

export type RecipeVariantGroupInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<string, RecipeVariantSlotMapInput<TSlot, TState, TStyle>>;

export type RecipeVariantsInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<string, RecipeVariantGroupInput<TSlot, TState, TStyle>>;

export type RecipeVariantValueMap = Record<string, string>;

export type RecipeVariantGroupsFor<
	TSlot extends string,
	TState extends string,
	TStyle extends RecipeStyle,
	TVariantValues extends RecipeVariantValueMap,
> = {
	readonly [K in keyof TVariantValues & string]: Record<
		TVariantValues[K],
		RecipeVariantSlotMapInput<TSlot, TState, TStyle>
	>;
};

export type RecipeWithVariantGroups<
	TSlot extends string,
	TState extends string,
	TStyle extends RecipeStyle,
	TVariantValues extends RecipeVariantValueMap,
> = RecipeInput<
	TSlot,
	TState,
	TStyle,
	RecipeVariantGroupsFor<TSlot, TState, TStyle, TVariantValues>
>;

export type RecipeVariantSelection<
	TVariants extends RecipeVariantsInput = RecipeVariantsInput,
> = Partial<{
	readonly [K in keyof TVariants & string]: keyof TVariants[K] & string;
}>;

export type RecipeCompoundVariantInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
	TVariants extends RecipeVariantsInput<TSlot, TState, TStyle> =
		RecipeVariantsInput<TSlot, TState, TStyle>,
> = {
	slots: RecipeVariantSlotMapInput<TSlot, TState, TStyle>;
	variants: RecipeVariantSelection<TVariants>;
};

export type RecipeInput<
	TSlot extends string = string,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
	TVariants extends RecipeVariantsInput<TSlot, TState, TStyle> =
		RecipeVariantsInput<TSlot, TState, TStyle>,
> = {
	compoundVariants?: readonly RecipeCompoundVariantInput<
		TSlot,
		TState,
		TStyle,
		TVariants
	>[];
	defaultVariants?: RecipeVariantSelection<TVariants>;
	slots: RecipeSlotMapInput<TSlot, TState, TStyle>;
	variants?: TVariants;
};

export type RecipeSlotName<TRecipe extends RecipeInput> =
	keyof TRecipe["slots"] & string;

export type RecipeVariantGroups<TRecipe extends RecipeInput> =
	TRecipe extends RecipeInput<string, string, RecipeStyle, infer TVariants>
		? TVariants
		: never;

export type RecipeVariantGroupName<TRecipe extends RecipeInput> =
	keyof RecipeVariantGroups<TRecipe> & string;

export type RecipeVariantValue<
	TRecipe extends RecipeInput,
	TGroup extends RecipeVariantGroupName<TRecipe>,
> = keyof RecipeVariantGroups<TRecipe>[TGroup] & string;

export type RecipeVariantValues<
	TRecipe extends RecipeInput,
	TGroups extends RecipeVariantGroupName<TRecipe> =
		RecipeVariantGroupName<TRecipe>,
> = {
	readonly [K in TGroups]: RecipeVariantValue<TRecipe, K>;
};

export type RecipeVariantProps<TRecipe extends RecipeInput> = Partial<{
	readonly [K in RecipeVariantGroupName<TRecipe>]: RecipeVariantValue<
		TRecipe,
		K
	>;
}>;

export type RecipeVariantPropsFor<
	TRecipe extends RecipeInput,
	TGroups extends RecipeVariantGroupName<TRecipe>,
> = Pick<RecipeVariantProps<TRecipe>, TGroups>;

export type ResolvedRecipeSlot<
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	base: TStyle;
	states: RecipeStateMap<TState, TStyle>;
};

export type ResolvedRecipe<
	TRecipe extends RecipeInput = RecipeInput,
	TState extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	slots: {
		readonly [K in RecipeSlotName<TRecipe>]: ResolvedRecipeSlot<
			TState,
			TStyle
		>;
	};
};

export type CreatedRecipe<TRecipe extends RecipeInput> = {
	input: TRecipe;
	resolve<const TGroups extends RecipeVariantGroupName<TRecipe>>(
		variants?: RecipeVariantPropsFor<TRecipe, TGroups>,
	): ResolvedRecipe<TRecipe, string, RecipeStyle>;
};

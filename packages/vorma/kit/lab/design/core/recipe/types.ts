import type { RecipeStyle } from "./style.ts";

export type RecipeConditionMap<
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Partial<Record<TCondition, TStyle>>;

export type RecipeSlotInput<
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	base?: TStyle;
	conditions?: RecipeConditionMap<TCondition, TStyle>;
};

export type RecipeSlotMapInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<TSlot, RecipeSlotInput<TCondition, TStyle>>;

export type RecipeVariantSlotMapInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Partial<Record<TSlot, RecipeSlotInput<TCondition, TStyle>>>;

export type RecipeVariantGroupInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<string, RecipeVariantSlotMapInput<TSlot, TCondition, TStyle>>;

export type RecipeVariantsInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = Record<string, RecipeVariantGroupInput<TSlot, TCondition, TStyle>>;

export type RecipeVariantValueMap = Record<string, string>;

export type RecipeVariantGroupsFor<
	TSlot extends string,
	TCondition extends string,
	TStyle extends RecipeStyle,
	TVariantValues extends RecipeVariantValueMap,
> = {
	readonly [K in keyof TVariantValues & string]: Record<
		TVariantValues[K],
		RecipeVariantSlotMapInput<TSlot, TCondition, TStyle>
	>;
};

export type RecipeWithVariantGroups<
	TSlot extends string,
	TCondition extends string,
	TStyle extends RecipeStyle,
	TVariantValues extends RecipeVariantValueMap,
> = RecipeInput<
	TSlot,
	TCondition,
	TStyle,
	RecipeVariantGroupsFor<TSlot, TCondition, TStyle, TVariantValues>
>;

export type RecipeVariantSelection<
	TVariants extends RecipeVariantsInput = RecipeVariantsInput,
> = Partial<{
	readonly [K in keyof TVariants & string]: keyof TVariants[K] & string;
}>;

export type RecipeCompoundVariantInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
	TVariants extends RecipeVariantsInput<TSlot, TCondition, TStyle> =
		RecipeVariantsInput<TSlot, TCondition, TStyle>,
> = {
	slots: RecipeVariantSlotMapInput<TSlot, TCondition, TStyle>;
	variants: RecipeVariantSelection<TVariants>;
};

export type RecipeInput<
	TSlot extends string = string,
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
	TVariants extends RecipeVariantsInput<TSlot, TCondition, TStyle> =
		RecipeVariantsInput<TSlot, TCondition, TStyle>,
> = {
	compoundVariants?: readonly RecipeCompoundVariantInput<
		TSlot,
		TCondition,
		TStyle,
		TVariants
	>[];
	defaultVariants?: RecipeVariantSelection<TVariants>;
	slots: RecipeSlotMapInput<TSlot, TCondition, TStyle>;
	variants?: TVariants;
};

export type RecipeConditionName<TRecipe extends RecipeInput> =
	TRecipe extends RecipeInput<string, infer TCondition, RecipeStyle>
		? TCondition
		: never;

export type RecipeSlotName<TRecipe extends RecipeInput> = keyof TRecipe["slots"] & string;

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
	TGroups extends RecipeVariantGroupName<TRecipe> = RecipeVariantGroupName<TRecipe>,
> = {
	readonly [K in TGroups]: RecipeVariantValue<TRecipe, K>;
};

export type RecipeVariantProps<TRecipe extends RecipeInput> = Partial<{
	readonly [K in RecipeVariantGroupName<TRecipe>]: RecipeVariantValue<TRecipe, K>;
}>;

export type RecipeVariantPropsFor<
	TRecipe extends RecipeInput,
	TGroups extends RecipeVariantGroupName<TRecipe>,
> = Pick<RecipeVariantProps<TRecipe>, TGroups>;

export type ResolvedRecipeSlot<
	TCondition extends string = string,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	base: TStyle;
	conditions: RecipeConditionMap<TCondition, TStyle>;
};

export type ResolvedRecipe<
	TRecipe extends RecipeInput = RecipeInput,
	TCondition extends string = RecipeConditionName<TRecipe>,
	TStyle extends RecipeStyle = RecipeStyle,
> = {
	slots: {
		readonly [K in RecipeSlotName<TRecipe>]: ResolvedRecipeSlot<TCondition, TStyle>;
	};
};

export type CreatedRecipe<TRecipe extends RecipeInput> = {
	input: TRecipe;
	resolve<const TGroups extends RecipeVariantGroupName<TRecipe>>(
		variants?: RecipeVariantPropsFor<TRecipe, TGroups>,
	): ResolvedRecipe<TRecipe, RecipeConditionName<TRecipe>, RecipeStyle>;
};

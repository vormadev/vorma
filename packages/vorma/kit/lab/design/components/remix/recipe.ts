import { css, type MixInput } from "remix/ui";
import {
	mergeRecipeStyles,
	type RecipeConditionMap as RecipeConditionStyleMap,
	type RecipeStyle,
	type ResolvedRecipeSlot,
} from "../../core/core.ts";

export type RecipeConditionSelectorMap<TCondition extends string = string> = Partial<
	Readonly<Record<TCondition, string>>
>;

export function mergeRecipeConditionSelectors<TCondition extends string>(
	...maps: readonly (RecipeConditionSelectorMap<TCondition> | undefined)[]
): RecipeConditionSelectorMap<TCondition> {
	return Object.assign({}, ...maps);
}

export type RecipeStyleInput<TCondition extends string = string> = {
	conditions?: RecipeConditionSelectorMap;
	slot: ResolvedRecipeSlot<TCondition, RecipeStyle>;
	style?: RecipeStyle;
};

function condition_styles_to_selectors<TCondition extends string>(
	condition_styles: RecipeConditionStyleMap<TCondition, RecipeStyle>,
	condition_selectors: RecipeConditionSelectorMap | undefined,
): RecipeStyle {
	const selector_styles: Record<string, RecipeStyle> = {};

	for (const [condition_name, condition_style] of Object.entries(condition_styles) as [
		TCondition,
		RecipeStyle | undefined,
	][]) {
		if (!condition_style) {
			continue;
		}

		const selector = condition_selectors?.[condition_name];
		if (!selector) {
			throw new Error(
				`Missing Remix recipe condition selector for "${condition_name}"`,
			);
		}

		selector_styles[selector] = mergeRecipeStyles(
			selector_styles[selector],
			condition_style,
		);
	}

	return selector_styles;
}

export function createRecipeStyle<TCondition extends string>(
	input: RecipeStyleInput<TCondition>,
): RecipeStyle {
	return mergeRecipeStyles(
		input.slot.base,
		condition_styles_to_selectors(input.slot.conditions, input.conditions),
		input.style,
	);
}

export function createRecipeMix<
	TElement extends Element = Element,
	TCondition extends string = string,
>(input: RecipeStyleInput<TCondition>): MixInput<TElement> {
	return css<TElement>(createRecipeStyle(input) as Parameters<typeof css>[0]);
}

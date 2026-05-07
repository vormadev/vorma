import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	checkableInitialChecked,
	checkableState,
	checkableStateAttribute,
	checkableStateFromValue,
	checkableStateMixin,
	type CheckableChecked,
} from "./checkable-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type CheckboxRecipeCondition =
	| CommonRecipeCondition
	| "checked"
	| "indeterminate"
	| "unchecked";

export type CheckboxChecked = CheckableChecked;

export type CheckboxRecipeInput<
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		size: TSize;
		variant: TVariant;
	}
>;

export type CheckboxRecipeVariant<TRecipe extends CheckboxRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type CheckboxRecipeSize<TRecipe extends CheckboxRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type CheckboxRecipeSelection<TRecipe extends CheckboxRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "variant">;

export type CheckboxStyleSystem<
	TMode extends string = string,
	TRecipe extends CheckboxRecipeInput = CheckboxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			checkbox: TRecipe;
		};
	},
	TMetadata
>;

export type CheckboxProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"input">, "checked" | "defaultChecked" | "style" | "type"> &
	Partial<CheckboxRecipeSelection<CheckboxRecipeInput<TVariant, TSize>>> &
	ResponsiveProps<
		CheckboxRecipeSelection<CheckboxRecipeInput<TVariant, TSize>>,
		TBreakpoint
	> & {
		checked?: CheckboxChecked;
		defaultChecked?: CheckboxChecked;
		style?: never;
	};

const checkbox_scope = "checkbox";
const checkbox_type = "checkbox";

const checkbox_conditions =
	mergeRecipeConditionSelectors<CheckboxRecipeCondition>(commonConditions, {
		checked: `&[${checkableStateAttribute}='${checkableState.checked}']`,
		indeterminate: `&[${checkableStateAttribute}='${checkableState.indeterminate}']`,
		unchecked: `&[${checkableStateAttribute}='${checkableState.unchecked}']`,
	} satisfies RecipeConditionSelectorMap<CheckboxRecipeCondition>);

export function createCheckbox<
	TMode extends string,
	TRecipe extends CheckboxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: CheckboxStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	CheckboxProps<
		CheckboxRecipeVariant<TRecipe>,
		CheckboxRecipeSize<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSelection = CheckboxRecipeSelection<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.checkbox);

	function resolve_root_slot(
		props: Partial<TSelection>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(props).slots.root;
	}

	return () => {
		return (
			props: CheckboxProps<
				CheckboxRecipeVariant<TRecipe>,
				CheckboxRecipeSize<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				at,
				checked,
				defaultChecked,
				mix,
				size,
				variant,
				...host_props
			} = props;
			const selection = {
				size,
				variant,
			} satisfies TSelection;
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					root: "input",
				},
				targets: {
					root: {
						host: "root",
						conditions: checkbox_conditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: selection,
				styleSystem: style_system,
			});

			return createElement(
				"input",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(checkbox_scope, "root"),
					mix: [
						checkableStateMixin({
							checked,
							defaultChecked,
						}),
						parts.hosts.root.mix,
					],
					props: {
						...host_props,
						[checkableStateAttribute]: checkableStateFromValue(
							checked ?? defaultChecked,
						),
						checked:
							checked === undefined
								? undefined
								: checkableInitialChecked(checked),
						defaultChecked:
							checked === undefined
								? checkableInitialChecked(defaultChecked)
								: undefined,
						mix,
						type: checkbox_type,
					},
				}),
			);
		};
	};
}

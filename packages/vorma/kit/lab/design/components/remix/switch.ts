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
	type CheckableCheckedChangeDetails,
} from "./checkable-state.ts";
import { componentDataAttribute, dataFlag } from "./component-state.ts";
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
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type SwitchRecipeCondition = CommonRecipeCondition | "checked" | "unchecked";

export type SwitchCheckedChangeDetails = CheckableCheckedChangeDetails;

export type SwitchCheckedChangeHandler = (
	checked: boolean,
	details?: SwitchCheckedChangeDetails,
) => void;

export type SwitchRecipeInput<
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

export type SwitchRecipeVariant<TRecipe extends SwitchRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type SwitchRecipeSize<TRecipe extends SwitchRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type SwitchRecipeSelection<TRecipe extends SwitchRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "variant">;

export type SwitchStyleSystem<
	TMode extends string = string,
	TRecipe extends SwitchRecipeInput = SwitchRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			switch: TRecipe;
		};
	},
	TMetadata
>;

export type SwitchProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"input">, "checked" | "defaultChecked" | "role" | "style" | "type"> &
	Partial<SwitchRecipeSelection<SwitchRecipeInput<TVariant, TSize>>> &
	ResponsiveProps<
		SwitchRecipeSelection<SwitchRecipeInput<TVariant, TSize>>,
		TBreakpoint
	> & {
		checked?: boolean;
		defaultChecked?: boolean;
		onCheckedChange?: SwitchCheckedChangeHandler;
		style?: never;
	};

const switch_scope = "switch";
const switch_role = "switch";
const switch_type = "checkbox";

const switch_conditions = mergeRecipeConditionSelectors<SwitchRecipeCondition>(
	commonConditions,
	{
		checked: `&[${checkableStateAttribute}='${checkableState.checked}']`,
		unchecked: `&[${checkableStateAttribute}='${checkableState.unchecked}']`,
	} satisfies RecipeConditionSelectorMap<SwitchRecipeCondition>,
);

export function createSwitch<
	TMode extends string,
	TRecipe extends SwitchRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SwitchStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	SwitchProps<
		SwitchRecipeVariant<TRecipe>,
		SwitchRecipeSize<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSelection = SwitchRecipeSelection<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.switch);

	function resolve_root_slot(
		props: Partial<TSelection>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(props).slots.root;
	}

	return () => {
		return (
			props: SwitchProps<
				SwitchRecipeVariant<TRecipe>,
				SwitchRecipeSize<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				at,
				checked,
				defaultChecked,
				disabled,
				mix,
				onCheckedChange,
				readOnly,
				required,
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
						conditions: switch_conditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: selection,
				styleSystem: style_system,
			});

			return createElement(
				"input",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(switch_scope, "root"),
					mix: [
						checkableStateMixin({
							allowIndeterminate: false,
							checked,
							defaultChecked,
							disabled,
							onCheckedChange: onCheckedChange
								? (next_checked, details) => {
										onCheckedChange(next_checked === true, details);
									}
								: undefined,
							readOnly,
						}),
						parts.hosts.root.mix,
					],
					props: {
						...host_props,
						[componentDataAttribute.disabled]: dataFlag(disabled === true),
						[componentDataAttribute.required]: dataFlag(required === true),
						[componentDataAttribute.readOnly]: dataFlag(readOnly === true),
						[checkableStateAttribute]: checkableStateFromValue(
							checked ?? defaultChecked,
							false,
						),
						checked:
							checked === undefined
								? undefined
								: checkableInitialChecked(checked),
						defaultChecked:
							checked === undefined
								? checkableInitialChecked(defaultChecked)
								: undefined,
						disabled,
						mix,
						readOnly,
						required,
						role: switch_role,
						type: switch_type,
					},
				}),
			);
		};
	};
}

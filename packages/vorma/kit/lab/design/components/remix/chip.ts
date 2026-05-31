import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type ChipRecipeInput<
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		selected: "false" | "true";
		size: TSize;
		variant: TVariant;
	}
>;

export type ChipRecipeVariant<TRecipe extends ChipRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type ChipRecipeSize<TRecipe extends ChipRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type ChipRecipeSelection<TRecipe extends ChipRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"selected" | "size" | "variant"
>;

export type ChipStyleSystem<
	TMode extends string = string,
	TRecipe extends ChipRecipeInput = ChipRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			chip: TRecipe;
		};
	},
	TMetadata
>;

export type ChipStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	selected?: boolean;
	size?: TSize;
	variant?: TVariant;
};

export type ChipProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	ChipStyleProps<TVariant, TSize> &
	ResponsiveProps<ChipStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
	};

const button_type_default = "button";
const chip_scope = "chip";

export function createChip<
	TMode extends string,
	TRecipe extends ChipRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ChipStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	ChipProps<
		ChipRecipeVariant<TRecipe>,
		ChipRecipeSize<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSelection = ChipRecipeSelection<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.chip);

	function selection(
		input: ChipStyleProps<ChipRecipeVariant<TRecipe>, ChipRecipeSize<TRecipe>>,
	): TSelection {
		return {
			selected: input.selected ? "true" : "false",
			size: input.size,
			variant: input.variant,
		} satisfies TSelection;
	}

	function resolve_root_slot(
		props: ChipStyleProps<ChipRecipeVariant<TRecipe>, ChipRecipeSize<TRecipe>>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(selection(props)).slots.root;
	}

	return () => {
		return (
			props: ChipProps<
				ChipRecipeVariant<TRecipe>,
				ChipRecipeSize<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				at,
				children,
				mix,
				selected = false,
				size,
				type = button_type_default,
				variant,
				...host_props
			} = props;
			const style_props = { selected, size, variant };
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: style_props,
				styleSystem: style_system,
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(chip_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						"aria-pressed": selected,
						"data-selected": selected || undefined,
						mix,
						type,
					},
				}),
				children,
			);
		};
	};
}

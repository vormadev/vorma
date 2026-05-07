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
import { type RecipeConditionSelectorMap } from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type SelectableTileRecipeCondition =
	| "active"
	| "disabled"
	| "focusVisible"
	| "hover"
	| "reducedMotion";

export type SelectableTileRecipeInput<
	TVariant extends string = string,
	TDensity extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		density: TDensity;
		selected: "false" | "true";
		variant: TVariant;
	}
>;

export type SelectableTileRecipeVariant<
	TRecipe extends SelectableTileRecipeInput,
> = RecipeVariantValue<TRecipe, "variant">;
export type SelectableTileRecipeDensity<
	TRecipe extends SelectableTileRecipeInput,
> = RecipeVariantValue<TRecipe, "density">;
export type SelectableTileRecipeSelection<
	TRecipe extends SelectableTileRecipeInput,
> = RecipeVariantPropsFor<TRecipe, "density" | "selected" | "variant">;

export type SelectableTileStyleSystem<
	TMode extends string = string,
	TRecipe extends SelectableTileRecipeInput = SelectableTileRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{ recipe: { selectableTile: TRecipe } },
	TMetadata
>;

export type SelectableTileStyleProps<
	TVariant extends string = string,
	TDensity extends string = string,
> = {
	density?: TDensity;
	selected?: boolean;
	variant?: TVariant;
};

export type SelectableTileProps<
	TVariant extends string = string,
	TDensity extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	SelectableTileStyleProps<TVariant, TDensity> &
	ResponsiveProps<
		SelectableTileStyleProps<TVariant, TDensity>,
		TBreakpoint
	> & {
		style?: never;
	};

export type SelectableTileOptions<
	TVariant extends string,
	TDensity extends string,
> = {
	defaultDensity?: NoInfer<TDensity>;
	defaultVariant?: NoInfer<TVariant>;
};

const selectable_tile_conditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectableTileRecipeCondition>;

const selectable_tile_scope = "selectableTile";

export function createSelectableTile<
	TMode extends string,
	TRecipe extends SelectableTileRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SelectableTileStyleSystem<TMode, TRecipe, TMetadata>,
	options: SelectableTileOptions<
		SelectableTileRecipeVariant<TRecipe>,
		SelectableTileRecipeDensity<TRecipe>
	> = {},
): RemixComponent<
	SelectableTileProps<
		SelectableTileRecipeVariant<TRecipe>,
		SelectableTileRecipeDensity<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TVariant = SelectableTileRecipeVariant<TRecipe>;
	type TDensity = SelectableTileRecipeDensity<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const selectable_tile_recipe = createRecipe(
		style_system.token.recipe.selectableTile,
	);

	function resolve_slot(
		props: Partial<SelectableTileStyleProps<TVariant, TDensity>>,
	): ReturnType<typeof selectable_tile_recipe.resolve>["slots"]["root"] {
		return selectable_tile_recipe.resolve({
			density: props.density,
			selected: props.selected ? "true" : "false",
			variant: props.variant,
		} satisfies SelectableTileRecipeSelection<TRecipe>).slots.root;
	}

	return () => {
		return (
			props: SelectableTileProps<TVariant, TDensity, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				density = options.defaultDensity,
				mix,
				selected = false,
				type = "button",
				variant = options.defaultVariant,
				...selectable_tile_props
			} = props;
			const selected_props = { density, selected, variant };
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: selectable_tile_conditions,
						resolveSlot: resolve_slot,
					},
				},
				props: selected_props,
				styleSystem: style_system,
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						selectable_tile_scope,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: {
						...selectable_tile_props,
						"aria-pressed":
							selectable_tile_props["aria-pressed"] ?? selected,
						mix,
						type,
					},
				}),
				children,
			);
		};
	};
}

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

export type GridAlign = "center" | "end" | "start" | "stretch";
export type GridJustify = "center" | "end" | "start" | "stretch";

export type GridRecipeInput<
	TColumns extends string = string,
	TGap extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		columns: TColumns;
		gap: TGap;
		layout: TLayout;
	}
>;

export type GridRecipeColumns<TRecipe extends GridRecipeInput> = RecipeVariantValue<
	TRecipe,
	"columns"
>;

export type GridRecipeGap<TRecipe extends GridRecipeInput> = RecipeVariantValue<
	TRecipe,
	"gap"
>;

export type GridRecipeLayout<TRecipe extends GridRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type GridRecipeSelection<TRecipe extends GridRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"columns" | "gap" | "layout"
>;

export type GridStyleSystem<
	TMode extends string = string,
	TRecipe extends GridRecipeInput = GridRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			grid: TRecipe;
		};
	},
	TMetadata
>;

export type GridStyleProps<
	TColumns extends string = string,
	TGap extends string = string,
	TLayout extends string = string,
> = Partial<GridRecipeSelection<GridRecipeInput<TColumns, TGap, TLayout>>> & {
	align?: GridAlign;
	justify?: GridJustify;
};

export type GridProps<
	TColumns extends string = string,
	TGap extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<keyof HTMLElementTagNameMap>, "style"> &
	GridStyleProps<TColumns, TGap, TLayout> &
	ResponsiveProps<GridStyleProps<TColumns, TGap, TLayout>, TBreakpoint> & {
		as?: keyof HTMLElementTagNameMap;
		style?: never;
	};

const align_map = {
	center: "center",
	end: "end",
	start: "start",
	stretch: "stretch",
} as const;
const grid_scope = "grid";

export function createGrid<
	TMode extends string,
	TRecipe extends GridRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: GridStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	GridProps<
		GridRecipeColumns<TRecipe>,
		GridRecipeGap<TRecipe>,
		GridRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TStyleProps = GridStyleProps<
		GridRecipeColumns<TRecipe>,
		GridRecipeGap<TRecipe>,
		GridRecipeLayout<TRecipe>
	>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.grid);

	function deterministic_style(props: Partial<TStyleProps>): ComponentStyle {
		return {
			alignItems: props.align ? align_map[props.align] : undefined,
			justifyItems: props.justify ? align_map[props.justify] : undefined,
		};
	}

	function resolve_root_slot(
		props: Partial<TStyleProps>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			columns: props.columns,
			gap: props.gap,
			layout: props.layout,
		} satisfies GridRecipeSelection<TRecipe>).slots.root;
	}

	return () => {
		return (
			props: GridProps<
				GridRecipeColumns<TRecipe>,
				GridRecipeGap<TRecipe>,
				GridRecipeLayout<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				align,
				as = "div",
				at,
				children,
				columns,
				gap,
				justify,
				layout,
				mix,
				...host_props
			} = props;
			const style_props = { align, columns, gap, justify, layout };
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
						resolveStyle: deterministic_style,
					},
				},
				props: style_props,
				styleSystem: style_system,
			});
			return createElement(
				as,
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(grid_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

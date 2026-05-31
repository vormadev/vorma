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

export type StackAlign = "center" | "end" | "start" | "stretch";
export type StackDirection = "column" | "column-reverse" | "row" | "row-reverse";
export type StackJustify = "between" | "center" | "end" | "start";

export type StackRecipeInput<
	TGap extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		gap: TGap;
		layout: TLayout;
	}
>;

export type StackRecipeGap<TRecipe extends StackRecipeInput> = RecipeVariantValue<
	TRecipe,
	"gap"
>;

export type StackRecipeLayout<TRecipe extends StackRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type StackRecipeSelection<TRecipe extends StackRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "gap" | "layout">;

export type StackStyleSystem<
	TMode extends string = string,
	TRecipe extends StackRecipeInput = StackRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			stack: TRecipe;
		};
	},
	TMetadata
>;

export type StackStyleProps<
	TGap extends string = string,
	TLayout extends string = string,
> = Partial<StackRecipeSelection<StackRecipeInput<TGap, TLayout>>> & {
	align?: StackAlign;
	direction?: StackDirection;
	justify?: StackJustify;
	wrap?: boolean;
};

export type StackProps<
	TGap extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<keyof HTMLElementTagNameMap>, "style"> &
	StackStyleProps<TGap, TLayout> &
	ResponsiveProps<StackStyleProps<TGap, TLayout>, TBreakpoint> & {
		as?: keyof HTMLElementTagNameMap;
		style?: never;
	};

export type StackOptions = {
	defaultAlign?: StackAlign;
	defaultDirection?: StackDirection;
	defaultJustify?: StackJustify;
	defaultWrap?: boolean;
};

const align_map = {
	center: "center",
	end: "flex-end",
	start: "flex-start",
	stretch: "stretch",
} as const;

const justify_map = {
	between: "space-between",
	center: "center",
	end: "flex-end",
	start: "flex-start",
} as const;
const stack_scope = "stack";

export function createStack<
	TMode extends string,
	TRecipe extends StackRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: StackStyleSystem<TMode, TRecipe, TMetadata>,
	options: StackOptions = {},
): RemixComponent<
	StackProps<
		StackRecipeGap<TRecipe>,
		StackRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TStyleProps = StackStyleProps<
		StackRecipeGap<TRecipe>,
		StackRecipeLayout<TRecipe>
	>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.stack);

	function deterministic_style(props: Partial<TStyleProps>): ComponentStyle {
		const align = props.align ?? options.defaultAlign;
		const direction = props.direction ?? options.defaultDirection;
		const justify = props.justify ?? options.defaultJustify;
		const wrap = props.wrap ?? options.defaultWrap;

		return {
			alignItems: align ? align_map[align] : undefined,
			flexDirection: direction,
			flexWrap: wrap === undefined ? undefined : wrap ? "wrap" : "nowrap",
			justifyContent: justify ? justify_map[justify] : undefined,
		};
	}

	function resolve_root_slot(
		props: Partial<TStyleProps>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			gap: props.gap,
			layout: props.layout,
		} satisfies StackRecipeSelection<TRecipe>).slots.root;
	}

	return () => {
		return (
			props: StackProps<
				StackRecipeGap<TRecipe>,
				StackRecipeLayout<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				align,
				as = "div",
				at,
				children,
				direction,
				gap,
				justify,
				layout,
				mix,
				wrap,
				...host_props
			} = props;
			const style_props = {
				align,
				direction,
				gap,
				justify,
				layout,
				wrap,
			};
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
					attrs: createComponentAnatomyAttrs(stack_scope, "root"),
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

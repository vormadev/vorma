import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeStyle,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type IconElement = "div" | "span";
export type IconRotate = string;

export type IconRecipeInput<
	TSize extends string = string,
	TTone extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	"root",
	never,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		tone: TTone;
	}
>;

export type IconRecipeSize<TRecipe extends IconRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;
export type IconRecipeTone<TRecipe extends IconRecipeInput> = RecipeVariantValue<
	TRecipe,
	"tone"
>;
export type IconRecipeLayout<TRecipe extends IconRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;
export type IconRecipeSelection<TRecipe extends IconRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"layout" | "size" | "tone"
>;

export type IconStyleSystem<
	TMode extends string = string,
	TRecipe extends IconRecipeInput = IconRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { icon: TRecipe } }, TMetadata>;

export type IconStyleProps<
	TSize extends string = string,
	TTone extends string = string,
	TLayout extends string = string,
> = {
	layout?: TLayout;
	rotate?: IconRotate;
	size?: TSize;
	tone?: TTone;
};

export type IconProps<
	TSize extends string = string,
	TTone extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<IconElement>, "style"> &
	IconStyleProps<TSize, TTone, TLayout> &
	ResponsiveProps<IconStyleProps<TSize, TTone, TLayout>, TBreakpoint> & {
		as?: IconElement;
		decorative?: boolean;
		style?: never;
	};

const icon_scope = "icon";

export function createIcon<
	TMode extends string,
	TRecipe extends IconRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: IconStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	IconProps<
		IconRecipeSize<TRecipe>,
		IconRecipeTone<TRecipe>,
		IconRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSize = IconRecipeSize<TRecipe>;
	type TTone = IconRecipeTone<TRecipe>;
	type TLayout = IconRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	const icon_recipe = createRecipe(style_system.token.recipe.icon);

	function resolve_root_slot(
		props: Partial<IconStyleProps<TSize, TTone, TLayout>>,
	): ReturnType<typeof icon_recipe.resolve>["slots"]["root"] {
		return icon_recipe.resolve({
			layout: props.layout,
			size: props.size,
			tone: props.tone,
		} satisfies IconRecipeSelection<TRecipe>).slots.root;
	}

	function deterministic_style(
		props: Partial<IconStyleProps<TSize, TTone, TLayout>>,
	): RecipeStyle | undefined {
		return props.rotate ? { rotate: props.rotate } : undefined;
	}

	return () => {
		return (props: IconProps<TSize, TTone, TLayout, TBreakpoint>): RemixNode => {
			const {
				as = "span",
				at,
				children,
				decorative,
				layout,
				mix,
				rotate,
				size,
				tone,
				...icon_props
			} = props;
			const has_accessible_name =
				icon_props["aria-label"] !== undefined ||
				icon_props["aria-labelledby"] !== undefined;
			const is_decorative = decorative ?? !has_accessible_name;
			const style_props = {
				layout,
				rotate,
				size,
				tone,
			};
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
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
					attrs: createComponentAnatomyAttrs(icon_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...icon_props,
						"aria-hidden":
							icon_props["aria-hidden"] ??
							(is_decorative ? true : undefined),
						mix,
					},
				}),
				children,
			);
		};
	};
}

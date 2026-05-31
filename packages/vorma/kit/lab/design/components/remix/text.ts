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

export type TextAlign = "center" | "end" | "justify" | "start";
export type TextRecipeSlot = "root";
export type TextRecipeCondition = string;

export type TextRecipeInput<
	TVariant extends string = string,
	TTone extends string = string,
> = RecipeWithVariantGroups<
	TextRecipeSlot,
	string,
	ComponentStyle,
	{
		tone: TTone;
		variant: TVariant;
	}
>;

export type TextRecipeVariant<TRecipe extends TextRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type TextRecipeTone<TRecipe extends TextRecipeInput> = RecipeVariantValue<
	TRecipe,
	"tone"
>;

export type TextRecipeSelection<TRecipe extends TextRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"tone" | "variant"
>;

export type TextStyleSystem<
	TMode extends string = string,
	TRecipe extends TextRecipeInput = TextRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			text: TRecipe;
		};
	},
	TMetadata
>;

type TextStyleProps<TVariant extends string, TTone extends string> = Partial<
	TextRecipeSelection<TextRecipeInput<TVariant, TTone>>
> & {
	align?: TextAlign;
};

export type TextProps<
	TVariant extends string = string,
	TTone extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<keyof HTMLElementTagNameMap>, "style"> &
	TextStyleProps<TVariant, TTone> &
	ResponsiveProps<TextStyleProps<TVariant, TTone>, TBreakpoint> & {
		as?: keyof HTMLElementTagNameMap;
		style?: never;
	};

const text_scope = "text";

export function createText<
	TMode extends string,
	TRecipe extends TextRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TextStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	TextProps<
		TextRecipeVariant<TRecipe>,
		TextRecipeTone<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TStyleProps = TextStyleProps<
		TextRecipeVariant<TRecipe>,
		TextRecipeTone<TRecipe>
	>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.text);

	function resolve_root_slot(
		props: Partial<TStyleProps>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			tone: props.tone,
			variant: props.variant,
		} satisfies Partial<TextRecipeSelection<TRecipe>>).slots.root;
	}

	function deterministic_style(
		props: Partial<TStyleProps>,
	): ComponentStyle | undefined {
		return props.align ? { textAlign: props.align } : undefined;
	}

	return () => {
		return (
			props: TextProps<
				TextRecipeVariant<TRecipe>,
				TextRecipeTone<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				align,
				as = "p",
				at,
				children,
				mix,
				tone,
				variant,
				...host_props
			} = props;
			const style_props = {
				align,
				tone,
				variant,
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
					attrs: createComponentAnatomyAttrs(text_scope, "root"),
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

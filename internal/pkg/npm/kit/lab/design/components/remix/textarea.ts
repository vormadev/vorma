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
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type TextareaRecipeInput<
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

export type TextareaRecipeVariant<TRecipe extends TextareaRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type TextareaRecipeSize<TRecipe extends TextareaRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type TextareaRecipeSelection<TRecipe extends TextareaRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "variant">;

export type TextareaStyleSystem<
	TMode extends string = string,
	TRecipe extends TextareaRecipeInput = TextareaRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			textarea: TRecipe;
		};
	},
	TMetadata
>;

export type TextareaProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"textarea">, "style"> &
	Partial<TextareaRecipeSelection<TextareaRecipeInput<TVariant, TSize>>> &
	ResponsiveProps<
		TextareaRecipeSelection<TextareaRecipeInput<TVariant, TSize>>,
		TBreakpoint
	> & {
		style?: never;
	};

const textarea_scope = "textarea";

export function createTextarea<
	TMode extends string,
	TRecipe extends TextareaRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TextareaStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	TextareaProps<
		TextareaRecipeVariant<TRecipe>,
		TextareaRecipeSize<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSelection = TextareaRecipeSelection<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.textarea);

	function resolve_root_slot(
		props: Partial<TSelection>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(props).slots.root;
	}

	return () => {
		return (
			props: TextareaProps<
				TextareaRecipeVariant<TRecipe>,
				TextareaRecipeSize<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const { at, mix, size, variant, ...host_props } = props;
			const selection = {
				size,
				variant,
			} satisfies TSelection;
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					root: "textarea",
				},
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: selection,
				styleSystem: style_system,
			});

			return createElement(
				"textarea",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(textarea_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						mix,
					},
				}),
			);
		};
	};
}

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

export type FormRecipeInput<TLayout extends string = string> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		layout: TLayout;
	}
>;

export type FormRecipeLayout<TRecipe extends FormRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type FormRecipeSelection<TRecipe extends FormRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"layout"
>;

export type FormStyleSystem<
	TMode extends string = string,
	TRecipe extends FormRecipeInput = FormRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { form: TRecipe } }, TMetadata>;

export type FormStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type FormProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"form">, "style"> &
	FormStyleProps<TLayout> &
	ResponsiveProps<FormStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

const form_scope = "form";

export function createForm<
	TMode extends string,
	TRecipe extends FormRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FormStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	FormProps<FormRecipeLayout<TRecipe>, BreakpointForStyleSystem<typeof style_system>>
> {
	type TLayout = FormRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.form);

	function resolve_root_slot(
		props: Partial<FormStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies FormRecipeSelection<TRecipe>).slots.root;
	}

	return () => {
		return (props: FormProps<TLayout, TBreakpoint>): RemixNode => {
			const { at, children, layout, mix, ...form_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"form",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(form_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...form_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

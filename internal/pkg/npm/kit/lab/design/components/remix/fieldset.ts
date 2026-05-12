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

export type FieldsetRecipeInput<TLayout extends string = string> =
	RecipeWithVariantGroups<
		"legend" | "root",
		string,
		ComponentStyle,
		{
			layout: TLayout;
		}
	>;

export type FieldsetRecipeLayout<TRecipe extends FieldsetRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type FieldsetRecipeSelection<TRecipe extends FieldsetRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout">;

export type FieldsetStyleSystem<
	TMode extends string = string,
	TRecipe extends FieldsetRecipeInput = FieldsetRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { fieldset: TRecipe } }, TMetadata>;

export type FieldsetRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type FieldsetRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"fieldset">, "style"> &
	FieldsetRootStyleProps<TLayout> &
	ResponsiveProps<FieldsetRootStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type FieldsetLegendProps = Omit<Props<"legend">, "style"> & {
	style?: never;
};

export type FieldsetComponents<TLayout extends string = string> = {
	Legend: RemixComponent<FieldsetLegendProps>;
	Root: RemixComponent<FieldsetRootProps<TLayout>>;
};

const fieldset_scope = "fieldset";

export function createFieldset<
	TMode extends string,
	TRecipe extends FieldsetRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldsetStyleSystem<TMode, TRecipe, TMetadata>,
): FieldsetComponents<FieldsetRecipeLayout<TRecipe>> {
	type TLayout = FieldsetRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.fieldset);

	function resolve_slot(
		slot: "legend" | "root",
		props: Partial<FieldsetRootStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies FieldsetRecipeSelection<TRecipe>).slots[slot];
	}

	function Root(): (
		props: FieldsetRootProps<TLayout, TBreakpoint>,
	) => RemixNode {
		return (props: FieldsetRootProps<TLayout, TBreakpoint>): RemixNode => {
			const { at, children, layout, mix, ...root_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: (current_props) => {
							return resolve_slot("root", current_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"fieldset",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(fieldset_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...root_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Legend(): (props: FieldsetLegendProps) => RemixNode {
		return (props: FieldsetLegendProps): RemixNode => {
			const { children, mix, ...legend_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					legend: {
						host: "legend",
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot("legend", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"legend",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						fieldset_scope,
						"legend",
					),
					mix: parts.hosts.legend.mix,
					props: {
						...legend_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	return {
		Legend,
		Root,
	};
}

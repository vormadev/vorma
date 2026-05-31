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
	type ComponentSlotProps,
} from "./component-style.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type StatRecipeInput<TValueTone extends string = string> = RecipeWithVariantGroups<
	"root" | "title" | "value",
	never,
	ComponentStyle,
	{
		valueTone: TValueTone;
	}
>;

export type StatRecipeValueTone<TRecipe extends StatRecipeInput> = RecipeVariantValue<
	TRecipe,
	"valueTone"
>;
export type StatRecipeSelection<TRecipe extends StatRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"valueTone"
>;

export type StatStyleSystem<
	TMode extends string = string,
	TRecipe extends StatRecipeInput = StatRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { stat: TRecipe } }, TMetadata>;

export type StatProps<TValueTone extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	style?: never;
	title: RemixNode;
	titleProps?: ComponentSlotProps<"div">;
	value: RemixNode;
	valueProps?: ComponentSlotProps<"div">;
	valueTone?: TValueTone;
};

const stat_scope = "stat";

export function createStat<
	TMode extends string,
	TRecipe extends StatRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: StatStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<StatProps<StatRecipeValueTone<TRecipe>>> {
	type TValueTone = StatRecipeValueTone<TRecipe>;
	const stat_recipe = createRecipe(style_system.token.recipe.stat);

	return () => {
		return (props: StatProps<TValueTone>): RemixNode => {
			const { title, titleProps, value, valueProps, valueTone, ...stat_props } =
				props;
			const { children: _title_children, ...title_props } = titleProps ?? {};
			const { children: _value_children, ...value_props } = valueProps ?? {};
			const resolved = stat_recipe.resolve({
				valueTone,
			} satisfies StatRecipeSelection<TRecipe>);
			const parts = createComponentStyleTargets({
				targets: {
					root: {
						host: "root",
						resolveSlot: () => {
							return resolved.slots.root;
						},
					},
					title: {
						host: "title",
						resolveSlot: () => {
							return resolved.slots.title;
						},
					},
					value: {
						host: "value",
						resolveSlot: () => {
							return resolved.slots.value;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(stat_scope, "root"),
					mix: parts.hosts.root.mix,
					props: stat_props,
				}),
				createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(stat_scope, "title"),
						mix: parts.hosts.title.mix,
						props: title_props,
					}),
					title,
				),
				createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(stat_scope, "value"),
						mix: parts.hosts.value.mix,
						props: value_props,
					}),
					value,
				),
			);
		};
	};
}

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
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type DescriptionListRecipeInput<
	TTone extends string = string,
	TDescriptionTone extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	"description" | "item" | "root" | "term",
	never,
	ComponentStyle,
	{
		descriptionTone: TDescriptionTone;
		layout: TLayout;
		tone: TTone;
	}
>;

export type DescriptionListRecipeTone<
	TRecipe extends DescriptionListRecipeInput,
> = RecipeVariantValue<TRecipe, "tone">;
export type DescriptionListRecipeDescriptionTone<
	TRecipe extends DescriptionListRecipeInput,
> = RecipeVariantValue<TRecipe, "descriptionTone">;
export type DescriptionListLayout<TRecipe extends DescriptionListRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type DescriptionListRecipeSelection<
	TRecipe extends DescriptionListRecipeInput,
> = RecipeVariantPropsFor<TRecipe, "descriptionTone" | "layout" | "tone">;

export type DescriptionListStyleSystem<
	TMode extends string = string,
	TRecipe extends DescriptionListRecipeInput = DescriptionListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{ recipe: { descriptionList: TRecipe } },
	TMetadata
>;

export type DescriptionListProps = Omit<Props<"dl">, "style"> & {
	style?: never;
};

export type DescriptionListItemStyleProps<
	TDescriptionTone extends string = string,
	TTone extends string = string,
	TLayout extends string = string,
> = {
	descriptionTone?: TDescriptionTone;
	layout?: TLayout;
	tone?: TTone;
};

export type DescriptionListItemProps<
	TDescriptionTone extends string = string,
	TTone extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	DescriptionListItemStyleProps<TDescriptionTone, TTone, TLayout> &
	ResponsiveProps<
		DescriptionListItemStyleProps<TDescriptionTone, TTone, TLayout>,
		TBreakpoint
	> & {
		descriptionProps?: ComponentSlotProps<"dd">;
		style?: never;
		term: RemixNode;
		termProps?: ComponentSlotProps<"dt">;
	};

const layout_style: Record<
	string,
	{
		description?: ComponentStyle;
		item?: ComponentStyle;
	}
> = {
	row: {},
	stacked: {
		description: {
			textAlign: "left",
		},
		item: {
			alignItems: "flex-start",
			flexDirection: "column",
		},
	},
};
const description_list_scope = "descriptionList";

export function createDescriptionList<
	TMode extends string,
	TRecipe extends DescriptionListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: DescriptionListStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<DescriptionListProps> {
	const description_list_recipe = createRecipe(
		style_system.token.recipe.descriptionList,
	);

	return () => {
		return (props: DescriptionListProps): RemixNode => {
			const { children, ...listProps } = props;
			const resolved = description_list_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					root: {
						host: "root",
						resolveSlot: () => {
							return resolved.slots.root;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"dl",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						description_list_scope,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: listProps,
				}),
				children,
			);
		};
	};
}

export function createDescriptionListItem<
	TMode extends string,
	TRecipe extends DescriptionListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: DescriptionListStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	DescriptionListItemProps<
		DescriptionListRecipeDescriptionTone<TRecipe>,
		DescriptionListRecipeTone<TRecipe>,
		DescriptionListLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TDescriptionTone = DescriptionListRecipeDescriptionTone<TRecipe>;
	type TTone = DescriptionListRecipeTone<TRecipe>;
	type TLayout = DescriptionListLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	const description_list_recipe = createRecipe(
		style_system.token.recipe.descriptionList,
	);

	function resolve(
		props: Partial<
			DescriptionListItemStyleProps<TDescriptionTone, TTone, TLayout>
		>,
	): ReturnType<typeof description_list_recipe.resolve> {
		return description_list_recipe.resolve({
			descriptionTone: props.descriptionTone,
			layout: props.layout,
			tone: props.tone,
		} satisfies DescriptionListRecipeSelection<TRecipe>);
	}

	function item_style(
		props: Partial<
			DescriptionListItemStyleProps<TDescriptionTone, TTone, TLayout>
		>,
	): ComponentStyle | undefined {
		const layout = props.layout ? layout_style[props.layout] : undefined;
		return layout?.item;
	}

	function description_style(
		props: Partial<
			DescriptionListItemStyleProps<TDescriptionTone, TTone, TLayout>
		>,
	): ComponentStyle | undefined {
		const layout = props.layout ? layout_style[props.layout] : undefined;
		return layout?.description;
	}

	return () => {
		return (
			props: DescriptionListItemProps<
				TDescriptionTone,
				TTone,
				TLayout,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				at,
				children,
				descriptionProps,
				descriptionTone,
				layout,
				term,
				termProps,
				tone,
				...itemProps
			} = props;
			const { children: _description_children, ...description_props } =
				descriptionProps ?? {};
			const { children: _term_children, ...term_props } = termProps ?? {};
			const style_props = { descriptionTone, layout, tone };
			const parts = createComponentStyleTargets({
				at,
				targets: {
					description: {
						host: "description",
						resolveSlot: (current_style_props) => {
							return resolve(current_style_props).slots
								.description;
						},
						resolveStyle: description_style,
					},
					item: {
						host: "item",
						resolveSlot: (current_style_props) => {
							return resolve(current_style_props).slots.item;
						},
						resolveStyle: item_style,
					},
					term: {
						host: "term",
						resolveSlot: (current_style_props) => {
							return resolve(current_style_props).slots.term;
						},
					},
				},
				props: style_props,
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						description_list_scope,
						"item",
					),
					mix: parts.hosts.item.mix,
					props: itemProps,
				}),
				createElement(
					"dt",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							description_list_scope,
							"term",
						),
						mix: parts.hosts.term.mix,
						props: term_props,
					}),
					term,
				),
				createElement(
					"dd",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							description_list_scope,
							"description",
						),
						mix: parts.hosts.description.mix,
						props: description_props,
					}),
					children,
				),
			);
		};
	};
}

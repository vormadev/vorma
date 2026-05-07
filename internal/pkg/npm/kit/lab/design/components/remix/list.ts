import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type ListElement = "ol" | "ul";

export type ListRecipeInput = RecipeWithVariantGroups<
	"item" | "root",
	never,
	ComponentStyle,
	Record<string, never>
>;

export type ListStyleSystem<
	TMode extends string = string,
	TRecipe extends ListRecipeInput = ListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { list: TRecipe } }, TMetadata>;

export type ListProps = Omit<Props<ListElement>, "style"> & {
	as?: ListElement;
	style?: never;
};

export type ListItemProps = Omit<Props<"li">, "style"> & {
	style?: never;
};

const list_scope = "list";

export function createList<
	TMode extends string,
	TRecipe extends ListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ListStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<ListProps> {
	const list_recipe = createRecipe(style_system.token.recipe.list);

	return () => {
		return (props: ListProps): RemixNode => {
			const { as = "ul", children, mix, ...listProps } = props;
			const resolved = list_recipe.resolve();
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
				as,
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(list_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...listProps,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createListItem<
	TMode extends string,
	TRecipe extends ListRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ListStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<ListItemProps> {
	const list_recipe = createRecipe(style_system.token.recipe.list);

	return () => {
		return (props: ListItemProps): RemixNode => {
			const { children, mix, ...itemProps } = props;
			const resolved = list_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					item: {
						host: "item",
						resolveSlot: () => {
							return resolved.slots.item;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"li",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(list_scope, "item"),
					mix: parts.hosts.item.mix,
					props: {
						...itemProps,
						mix,
					},
				}),
				children,
			);
		};
	};
}

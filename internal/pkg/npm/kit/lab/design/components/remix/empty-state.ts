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
	type ComponentSlotProps,
} from "./component-style.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type EmptyStateRecipeInput = RecipeWithVariantGroups<
	"body" | "indicator" | "root" | "title",
	never,
	ComponentStyle,
	Record<string, never>
>;

export type EmptyStateStyleSystem<
	TMode extends string = string,
	TRecipe extends EmptyStateRecipeInput = EmptyStateRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { emptyState: TRecipe } }, TMetadata>;

export type EmptyStateProps = Omit<Props<"div">, "style"> & {
	bodyProps?: ComponentSlotProps<"div">;
	indicator?: RemixNode;
	indicatorProps?: ComponentSlotProps<"div">;
	style?: never;
	title: RemixNode;
	titleProps?: ComponentSlotProps<"div">;
};

const empty_state_scope = "emptyState";

export function createEmptyState<
	TMode extends string,
	TRecipe extends EmptyStateRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: EmptyStateStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<EmptyStateProps> {
	const empty_state_recipe = createRecipe(
		style_system.token.recipe.emptyState,
	);

	return () => {
		return (props: EmptyStateProps): RemixNode => {
			const {
				bodyProps,
				children,
				indicator,
				indicatorProps,
				title,
				titleProps,
				...empty_state_props
			} = props;
			const { children: _body_children, ...body_props } = bodyProps ?? {};
			const { children: _indicator_children, ...indicator_props } =
				indicatorProps ?? {};
			const { children: _title_children, ...title_props } =
				titleProps ?? {};
			const resolved = empty_state_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					body: {
						host: "body",
						resolveSlot: () => {
							return resolved.slots.body;
						},
					},
					indicator: {
						host: "indicator",
						resolveSlot: () => {
							return resolved.slots.indicator;
						},
					},
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
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						empty_state_scope,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: empty_state_props,
				}),
				indicator === undefined
					? null
					: createElement(
							"div",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									empty_state_scope,
									"indicator",
								),
								mix: parts.hosts.indicator.mix,
								props: {
									"aria-hidden":
										indicator_props["aria-hidden"] ??
										"true",
									...indicator_props,
								},
							}),
							indicator,
						),
				createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							empty_state_scope,
							"title",
						),
						mix: parts.hosts.title.mix,
						props: title_props,
					}),
					title,
				),
				children === undefined
					? null
					: createElement(
							"div",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									empty_state_scope,
									"body",
								),
								mix: parts.hosts.body.mix,
								props: body_props,
							}),
							children,
						),
			);
		};
	};
}

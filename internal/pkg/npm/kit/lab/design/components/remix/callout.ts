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
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type CalloutRecipeInput<TTone extends string = string> =
	RecipeWithVariantGroups<
		"content" | "icon" | "root",
		never,
		ComponentStyle,
		{
			tone: TTone;
		}
	>;

export type CalloutRecipeTone<TRecipe extends CalloutRecipeInput> =
	RecipeVariantValue<TRecipe, "tone">;

export type CalloutRecipeSelection<TRecipe extends CalloutRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "tone">;

export type CalloutStyleSystem<
	TMode extends string = string,
	TRecipe extends CalloutRecipeInput = CalloutRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { callout: TRecipe } }, TMetadata>;

export type CalloutProps<TTone extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	contentProps?: ComponentSlotProps<"div">;
	icon?: RemixNode;
	iconProps?: ComponentSlotProps<"span">;
	style?: never;
	tone?: TTone;
};

const callout_scope = "callout";

export function createCallout<
	TMode extends string,
	TRecipe extends CalloutRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: CalloutStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<CalloutProps<CalloutRecipeTone<TRecipe>>> {
	type TTone = CalloutRecipeTone<TRecipe>;
	const callout_recipe = createRecipe(style_system.token.recipe.callout);

	return () => {
		return (props: CalloutProps<TTone>): RemixNode => {
			const {
				children,
				contentProps,
				icon,
				iconProps,
				tone,
				...callout_props
			} = props;
			const { children: _content_children, ...content_props } =
				contentProps ?? {};
			const { children: _icon_children, ...icon_props } = iconProps ?? {};
			const resolved = callout_recipe.resolve({
				tone,
			} satisfies CalloutRecipeSelection<TRecipe>);
			const parts = createComponentStyleTargets({
				targets: {
					content: {
						host: "content",
						resolveSlot: () => {
							return resolved.slots.content;
						},
					},
					icon: {
						host: "icon",
						resolveSlot: () => {
							return resolved.slots.icon;
						},
					},
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
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(callout_scope, "root"),
					mix: parts.hosts.root.mix,
					props: callout_props,
				}),
				icon === undefined
					? null
					: createElement(
							"span",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									callout_scope,
									"icon",
								),
								mix: parts.hosts.icon.mix,
								props: {
									"aria-hidden":
										icon_props["aria-hidden"] ?? "true",
									...icon_props,
								},
							}),
							icon,
						),
				createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							callout_scope,
							"content",
						),
						mix: parts.hosts.content.mix,
						props: content_props,
					}),
					children,
				),
			);
		};
	};
}

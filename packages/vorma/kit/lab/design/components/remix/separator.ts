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
import { commonConditions } from "./conditions.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export const separatorDecorativeRole = "presentation";
export const separatorHiddenAttribute = "aria-hidden";
export const separatorOrientation = {
	horizontal: "horizontal",
	vertical: "vertical",
} as const;
export const separatorOrientationAttribute = "data-orientation";
export const separatorRole = "separator";

export type SeparatorOrientation =
	(typeof separatorOrientation)[keyof typeof separatorOrientation];

export type SeparatorRecipeInput = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	Record<never, string>
>;

export type SeparatorStyleSystem<
	TMode extends string = string,
	TRecipe extends SeparatorRecipeInput = SeparatorRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			separator: TRecipe;
		};
	},
	TMetadata
>;

export type SeparatorProps = Omit<
	Props<"div">,
	"children" | "role" | "style" | "tabIndex"
> & {
	children?: never;
	decorative?: boolean;
	orientation?: SeparatorOrientation;
	style?: never;
};

const separator_scope = "separator";

export function createSeparator<
	TMode extends string,
	TRecipe extends SeparatorRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SeparatorStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<SeparatorProps> {
	const recipe = createRecipe(style_system.token.recipe.separator);

	return () => {
		return (props: SeparatorProps): RemixNode => {
			const {
				decorative = false,
				mix,
				orientation = separatorOrientation.horizontal,
				...host_props
			} = props;
			const resolved = recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
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
					attrs: createComponentAnatomyAttrs(separator_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						[separatorHiddenAttribute]: decorative ? "true" : undefined,
						[separatorOrientationAttribute]: orientation,
						mix,
						role: decorative ? separatorDecorativeRole : separatorRole,
					},
				}),
			);
		};
	};
}

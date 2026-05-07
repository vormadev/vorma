import {
	type AnyGeneratedSystemMetadata,
	type RecipeVariantValue,
} from "../../core/core.ts";
import type { BreakpointForStyleSystem } from "./responsive.ts";
import {
	createRootComponent,
	type RootComponent,
	type RootRecipeInput,
	type RootStyleSystem,
} from "./root.ts";

export type BadgeRecipeInput<
	TTone extends string = string,
	TSize extends string = string,
> = RootRecipeInput<{
	size: TSize;
	tone: TTone;
}>;

export type BadgeRecipeTone<TRecipe extends BadgeRecipeInput> =
	RecipeVariantValue<TRecipe, "tone">;

export type BadgeRecipeSize<TRecipe extends BadgeRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type BadgeStyleSystem<
	TMode extends string = string,
	TRecipe extends BadgeRecipeInput = BadgeRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = RootStyleSystem<TMode, TRecipe, TMetadata, "badge">;

export function createBadge<
	TMode extends string,
	TRecipe extends BadgeRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: BadgeStyleSystem<TMode, TRecipe, TMetadata>,
): RootComponent<
	"size" | "tone",
	TRecipe,
	BreakpointForStyleSystem<BadgeStyleSystem<TMode, TRecipe, TMetadata>>
> {
	return createRootComponent(style_system, {
		defaultElement: "span",
		recipeName: "badge",
		resolveProps: (props) => {
			return {
				size: props.size,
				tone: props.tone,
			};
		},
		variantProps: ["size", "tone"],
	});
}

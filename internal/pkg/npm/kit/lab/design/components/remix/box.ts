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

export type BoxRecipeInput<TLayout extends string = string> = RootRecipeInput<{
	layout: TLayout;
}>;

export type BoxRecipeLayout<TRecipe extends BoxRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type BoxStyleSystem<
	TMode extends string = string,
	TRecipe extends BoxRecipeInput = BoxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = RootStyleSystem<TMode, TRecipe, TMetadata, "box">;

export function createBox<
	TMode extends string,
	TRecipe extends BoxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: RootStyleSystem<TMode, TRecipe, TMetadata, "box">,
): RootComponent<
	"layout",
	TRecipe,
	BreakpointForStyleSystem<RootStyleSystem<TMode, TRecipe, TMetadata, "box">>
> {
	return createRootComponent(style_system, {
		defaultElement: "div",
		recipeName: "box",
		resolveProps: (props) => {
			return {
				layout: props.layout,
			};
		},
		variantProps: ["layout"],
	});
}

import {
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
} from "../../core/core.ts";
import type { BreakpointForStyleSystem } from "./responsive.ts";
import {
	createRootComponent,
	type RootComponent,
	type RootRecipeInput,
	type RootStyleSystem,
} from "./root.ts";

export type SurfaceRecipeInput<
	TVariant extends string = string,
	TDensity extends string = string,
	TLayout extends string = string,
> = RootRecipeInput<{
	density: TDensity;
	layout: TLayout;
	variant: TVariant;
}>;

export type SurfaceRecipeVariant<TRecipe extends SurfaceRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type SurfaceRecipeDensity<TRecipe extends SurfaceRecipeInput> = RecipeVariantValue<
	TRecipe,
	"density"
>;

export type SurfaceRecipeLayout<TRecipe extends SurfaceRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type SurfaceStyleSystem<
	TMode extends string = string,
	TRecipe extends SurfaceRecipeInput = SurfaceRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = RootStyleSystem<TMode, TRecipe, TMetadata, "surface">;

export type SurfaceRecipeSelection<TRecipe extends SurfaceRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "density" | "layout" | "variant">;

export function createSurface<
	TMode extends string,
	TRecipe extends SurfaceRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SurfaceStyleSystem<TMode, TRecipe, TMetadata>,
): RootComponent<
	"density" | "layout" | "variant",
	TRecipe,
	BreakpointForStyleSystem<SurfaceStyleSystem<TMode, TRecipe, TMetadata>>
> {
	return createRootComponent(style_system, {
		defaultElement: "div",
		recipeName: "surface",
		resolveProps: (props: Partial<SurfaceRecipeSelection<TRecipe>>) => {
			return {
				density: props.density,
				layout: props.layout,
				variant: props.variant,
			};
		},
		variantProps: ["density", "layout", "variant"],
	});
}

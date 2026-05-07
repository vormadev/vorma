import {
	type AnyGeneratedSystemMetadata,
	type RecipeVariantValue,
} from "../../core/core.ts";
import type { BreakpointForStyleSystem } from "./responsive.ts";
import {
	createRootComponent,
	type RootComponent,
	type RootComponentProps,
	type RootRecipeInput,
	type RootStyleSystem,
} from "./root.ts";

export const kbdDefaultElement = "kbd";
export const kbdScope = "kbd";

export type KbdRecipeInput<TSize extends string = string> = RootRecipeInput<{
	size: TSize;
}>;

export type KbdRecipeSize<TRecipe extends KbdRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type KbdStyleSystem<
	TMode extends string = string,
	TRecipe extends KbdRecipeInput = KbdRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = RootStyleSystem<TMode, TRecipe, TMetadata, typeof kbdScope>;

export type KbdProps<
	TRecipe extends KbdRecipeInput = KbdRecipeInput,
	TBreakpoint extends string = string,
> = RootComponentProps<"size", TRecipe, TBreakpoint>;

export function createKbd<
	TMode extends string,
	TRecipe extends KbdRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: KbdStyleSystem<TMode, TRecipe, TMetadata>,
): RootComponent<
	"size",
	TRecipe,
	BreakpointForStyleSystem<KbdStyleSystem<TMode, TRecipe, TMetadata>>
> {
	return createRootComponent(style_system, {
		defaultElement: kbdDefaultElement,
		recipeName: kbdScope,
		resolveProps: (props) => {
			return {
				size: props.size,
			};
		},
		variantProps: ["size"],
	});
}

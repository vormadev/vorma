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
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type SpinnerRecipeInput<
	TTone extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		size: TSize;
		tone: TTone;
	}
>;

export type SpinnerRecipeTone<TRecipe extends SpinnerRecipeInput> = RecipeVariantValue<
	TRecipe,
	"tone"
>;

export type SpinnerRecipeSize<TRecipe extends SpinnerRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type SpinnerRecipeSelection<TRecipe extends SpinnerRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "tone">;

export type SpinnerStyleSystem<
	TMode extends string = string,
	TRecipe extends SpinnerRecipeInput = SpinnerRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			spinner: TRecipe;
		};
	},
	TMetadata
>;

export type SpinnerProps<
	TTone extends string = string,
	TSize extends string = string,
> = Omit<Props<"span">, "style"> &
	Partial<SpinnerRecipeSelection<SpinnerRecipeInput<TTone, TSize>>> & {
		idle?: boolean;
		style?: never;
	};

const spinner_scope = "spinner";

export function createSpinner<
	TMode extends string,
	TRecipe extends SpinnerRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SpinnerStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<SpinnerProps<SpinnerRecipeTone<TRecipe>, SpinnerRecipeSize<TRecipe>>> {
	const recipe = createRecipe(style_system.token.recipe.spinner);

	return () => {
		return (
			props: SpinnerProps<SpinnerRecipeTone<TRecipe>, SpinnerRecipeSize<TRecipe>>,
		): RemixNode => {
			const { idle = false, mix, size, tone, ...host_props } = props;
			const resolved = recipe.resolve({
				size,
				tone,
			} satisfies SpinnerRecipeSelection<TRecipe>);
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
				"span",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(spinner_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						"data-idle": idle || undefined,
						mix,
					},
				}),
			);
		};
	};
}

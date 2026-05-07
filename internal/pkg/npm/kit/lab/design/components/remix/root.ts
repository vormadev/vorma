import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import { type RecipeConditionSelectorMap } from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type RootRecipeInput<TGroups extends Record<string, string>> =
	RecipeWithVariantGroups<"root", string, ComponentStyle, TGroups>;

export type RootStyleSystem<
	TMode extends string = string,
	TRecipe extends RootRecipeInput<Record<string, string>> = RootRecipeInput<
		Record<string, string>
	>,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
	TRecipeName extends string = string,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: Record<TRecipeName, TRecipe>;
	},
	TMetadata
>;

export type RootVariantProps<
	TRecipe extends RootRecipeInput<Record<string, string>>,
	TGroups extends string,
> = RecipeVariantPropsFor<TRecipe, TGroups>;

export type RootHostProps<TElement extends keyof HTMLElementTagNameMap> = Omit<
	Props<TElement>,
	"style"
> & {
	as?: TElement;
	style?: never;
};

export type RootComponentProps<
	TGroups extends string,
	TRecipe extends RootRecipeInput<Record<TGroups, string>>,
	TBreakpoint extends string,
> = RootHostProps<keyof HTMLElementTagNameMap> &
	Partial<RecipeVariantPropsFor<TRecipe, TGroups>> &
	ResponsiveProps<RecipeVariantPropsFor<TRecipe, TGroups>, TBreakpoint>;

export type RootComponent<
	TGroups extends string,
	TRecipe extends RootRecipeInput<Record<TGroups, string>>,
	TBreakpoint extends string,
> = RemixComponent<RootComponentProps<TGroups, TRecipe, TBreakpoint>>;

export function createRootComponent<
	TRecipeName extends string,
	TDefaultElement extends keyof HTMLElementTagNameMap,
	TGroups extends string,
	TMode extends string,
	TRecipe extends RootRecipeInput<Record<TGroups, string>>,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: RootStyleSystem<TMode, TRecipe, TMetadata, TRecipeName>,
	input: {
		conditions?: RecipeConditionSelectorMap;
		defaultElement: TDefaultElement;
		recipeName: TRecipeName;
		resolveProps: (
			props: Partial<RecipeVariantPropsFor<TRecipe, TGroups>>,
		) => RecipeVariantPropsFor<TRecipe, TGroups>;
		variantProps: readonly TGroups[];
	},
): RootComponent<
	TGroups,
	TRecipe,
	BreakpointForStyleSystem<typeof style_system>
> {
	type TProps = RecipeVariantPropsFor<TRecipe, TGroups>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	const recipe = createRecipe(style_system.token.recipe[input.recipeName]);
	const conditions = input.conditions ?? commonConditions;

	function resolve_root_slot(
		props: Partial<TProps>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(input.resolveProps(props)).slots.root;
	}

	return () => {
		return (
			props: RootHostProps<keyof HTMLElementTagNameMap> &
				Partial<TProps> &
				ResponsiveProps<TProps, TBreakpoint>,
		): RemixNode => {
			const { as, at, children, mix, ...host_props } =
				props as RootHostProps<keyof HTMLElementTagNameMap> &
					Partial<TProps> &
					ResponsiveProps<TProps, TBreakpoint>;
			const element_props = { ...host_props };
			for (const variant_prop of input.variantProps) {
				delete (element_props as Record<string, unknown>)[variant_prop];
			}
			const selected_props = input.resolveProps(
				host_props as Partial<TProps>,
			);
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: selected_props,
				styleSystem: style_system,
			});

			return createElement(
				as ?? input.defaultElement,
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						input.recipeName,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: {
						...element_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

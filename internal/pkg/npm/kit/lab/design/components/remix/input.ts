import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	componentDataAttribute,
	dataFlag,
	is_aria_invalid,
} from "./component-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type InputRecipeInput<
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"root",
	string,
	ComponentStyle,
	{
		size: TSize;
		variant: TVariant;
	}
>;

export type InputRecipeVariant<TRecipe extends InputRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type InputRecipeSize<TRecipe extends InputRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type InputRecipeSelection<TRecipe extends InputRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "variant">;

export type InputStyleSystem<
	TMode extends string = string,
	TRecipe extends InputRecipeInput = InputRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			input: TRecipe;
		};
	},
	TMetadata
>;

export type InputProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"input">, "size" | "style"> &
	Partial<InputRecipeSelection<InputRecipeInput<TVariant, TSize>>> &
	ResponsiveProps<
		InputRecipeSelection<InputRecipeInput<TVariant, TSize>>,
		TBreakpoint
	> & {
		htmlSize?: number;
		style?: never;
	};

export const inputAnatomy = {
	root: "root",
} as const;

const input_scope = "input";

export function createInput<
	TMode extends string,
	TRecipe extends InputRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: InputStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	InputProps<
		InputRecipeVariant<TRecipe>,
		InputRecipeSize<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TSelection = InputRecipeSelection<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.input);

	function resolve_root_slot(
		props: Partial<TSelection>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve(props).slots.root;
	}

	return () => {
		return (
			props: InputProps<
				InputRecipeVariant<TRecipe>,
				InputRecipeSize<TRecipe>,
				TBreakpoint
			>,
		): RemixNode => {
			const {
				at,
				"aria-invalid": aria_invalid,
				disabled,
				htmlSize,
				mix,
				readOnly,
				required,
				size,
				variant,
				...host_props
			} = props;
			const selection = {
				size,
				variant,
			} satisfies TSelection;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: selection,
				styleSystem: style_system,
			});

			return createElement(
				"input",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						input_scope,
						inputAnatomy.root,
					),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						"aria-invalid": aria_invalid,
						[componentDataAttribute.disabled]: dataFlag(
							disabled === true,
						),
						[componentDataAttribute.invalid]: dataFlag(
							is_aria_invalid(aria_invalid),
						),
						[componentDataAttribute.readOnly]: dataFlag(
							readOnly === true,
						),
						[componentDataAttribute.required]: dataFlag(
							required === true,
						),
						disabled,
						mix,
						readOnly,
						required,
						size: htmlSize,
					},
				}),
			);
		};
	};
}

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
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

const button_slot = {
	content: "content",
	loading_indicator: "loadingIndicator",
	loading_indicator_frame: "loadingIndicatorFrame",
	root: "root",
} as const;

export type ButtonRecipeSlot = (typeof button_slot)[keyof typeof button_slot];

export type ButtonRecipeCondition =
	| "active"
	| "disabled"
	| "focusVisible"
	| "hover"
	| "reducedMotion";

export type ButtonRecipeInput<
	TVariant extends string = string,
	TSize extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	ButtonRecipeSlot,
	string,
	ComponentStyle,
	{
		fluid: "true";
		layout: TLayout;
		loading: "true";
		size: TSize;
		variant: TVariant;
	}
>;

export type ButtonRecipeVariant<TRecipe extends ButtonRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type ButtonRecipeSize<TRecipe extends ButtonRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type ButtonRecipeLayout<TRecipe extends ButtonRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type ButtonRecipeSelection<TRecipe extends ButtonRecipeInput> =
	RecipeVariantPropsFor<
		TRecipe,
		"fluid" | "layout" | "loading" | "size" | "variant"
	>;

export type ButtonStyleSystem<
	TMode extends string = string,
	TRecipe extends ButtonRecipeInput = ButtonRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			button: TRecipe;
		};
	},
	TMetadata
>;

export type ButtonStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
	TLayout extends string = string,
> = {
	fluid?: boolean;
	layout?: TLayout;
	size?: TSize;
	variant?: TVariant;
};

type ButtonHostProps = Props<"button">;

export type ButtonProps<
	TVariant extends string = string,
	TSize extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<ButtonHostProps, "style"> &
	ButtonStyleProps<TVariant, TSize, TLayout> &
	ResponsiveProps<ButtonStyleProps<TVariant, TSize, TLayout>, TBreakpoint> & {
		loading?: boolean;
		loadingLabel?: string;
		style?: never;
	};

export type ButtonOptions<TVariant extends string, TSize extends string> = {
	conditions?: Partial<
		Readonly<Record<ButtonRecipeSlot, RecipeConditionSelectorMap<string>>>
	>;
	defaultSize?: NoInfer<TSize>;
	defaultVariant?: NoInfer<TVariant>;
};

const button_conditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<ButtonRecipeCondition>;

const loading_indicator_conditions = {
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<ButtonRecipeCondition>;

const button_scope = "button";
const loading_label_default = "Loading";
const button_type_default = "button";

export function createButton<
	TMode extends string,
	TRecipe extends ButtonRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ButtonStyleSystem<TMode, TRecipe, TMetadata>,
	options: ButtonOptions<
		ButtonRecipeVariant<TRecipe>,
		ButtonRecipeSize<TRecipe>
	> = {},
): RemixComponent<
	ButtonProps<
		ButtonRecipeVariant<TRecipe>,
		ButtonRecipeSize<TRecipe>,
		ButtonRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TVariant = ButtonRecipeVariant<TRecipe>;
	type TSize = ButtonRecipeSize<TRecipe>;
	type TLayout = ButtonRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TStyleProps = ButtonStyleProps<TVariant, TSize, TLayout> & {
		loading?: boolean;
	};

	const button_recipe = createRecipe(style_system.token.recipe.button);
	const resolve_slot = (
		slot: ButtonRecipeSlot,
		style_props: Partial<TStyleProps>,
	): ReturnType<typeof button_recipe.resolve>["slots"][ButtonRecipeSlot] => {
		const resolved = button_recipe.resolve({
			fluid: style_props.fluid ? "true" : undefined,
			layout: style_props.layout,
			loading: style_props.loading ? "true" : undefined,
			size: style_props.size,
			variant: style_props.variant,
		} satisfies ButtonRecipeSelection<TRecipe>);

		return resolved.slots[slot];
	};

	return () => {
		return (
			props: ButtonProps<TVariant, TSize, TLayout, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				disabled = false,
				fluid = false,
				layout,
				loading = false,
				loadingLabel: loading_label = loading_label_default,
				mix,
				size = options.defaultSize,
				type = button_type_default,
				variant = options.defaultVariant,
				...button_props
			} = props;
			const style_props = {
				fluid,
				layout,
				loading,
				size,
				variant,
			};
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[button_slot.content]: {
						host: button_slot.content,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[button_slot.content],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								button_slot.content,
								current_style_props,
							);
						},
					},
					[button_slot.loading_indicator]: {
						host: button_slot.loading_indicator,
						conditions: mergeRecipeConditionSelectors(
							loading_indicator_conditions,
							options.conditions?.[button_slot.loading_indicator],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								button_slot.loading_indicator,
								current_style_props,
							);
						},
					},
					[button_slot.loading_indicator_frame]: {
						host: button_slot.loading_indicator_frame,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[
								button_slot.loading_indicator_frame
							],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								button_slot.loading_indicator_frame,
								current_style_props,
							);
						},
					},
					[button_slot.root]: {
						host: button_slot.root,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[button_slot.root],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								button_slot.root,
								current_style_props,
							);
						},
					},
				},
				props: style_props,
				styleSystem: style_system,
			});
			const is_disabled = disabled || loading;

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						button_scope,
						button_slot.root,
					),
					mix: parts.hosts[button_slot.root].mix,
					props: {
						...button_props,
						"aria-busy": loading || undefined,
						"aria-label": loading
							? loading_label
							: props["aria-label"],
						disabled: is_disabled,
						mix,
						type,
					},
				}),
				createElement(
					"span",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							button_scope,
							button_slot.content,
						),
						mix: parts.hosts[button_slot.content].mix,
					}),
					children,
				),
				loading
					? createElement(
							"span",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									button_scope,
									button_slot.loading_indicator_frame,
								),
								mix: parts.hosts[
									button_slot.loading_indicator_frame
								].mix,
								props: {
									"aria-hidden": "true",
								},
							}),
							createElement(
								"span",
								createComponentSlotProps({
									attrs: createComponentAnatomyAttrs(
										button_scope,
										button_slot.loading_indicator,
									),
									mix: parts.hosts[
										button_slot.loading_indicator
									].mix,
								}),
							),
						)
					: null,
			);
		};
	};
}

import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import { componentDataAttribute, dataFlag } from "./component-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export const buttonAnatomy = {
	content: "content",
	loadingIndicator: "loadingIndicator",
	loadingIndicatorFrame: "loadingIndicatorFrame",
	root: "root",
} as const;

export type ButtonRecipeSlot = (typeof buttonAnatomy)[keyof typeof buttonAnatomy];

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

export type ButtonRecipeVariant<TRecipe extends ButtonRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type ButtonRecipeSize<TRecipe extends ButtonRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type ButtonRecipeLayout<TRecipe extends ButtonRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type ButtonRecipeSelection<TRecipe extends ButtonRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "fluid" | "layout" | "loading" | "size" | "variant">;

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
		loadingIndicator?: RemixNode;
		loadingLabel?: string;
		style?: never;
	};

export type ButtonOptions = {
	conditions?: Partial<
		Readonly<Record<ButtonRecipeSlot, RecipeConditionSelectorMap<string>>>
	>;
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
const button_type_default = "button";

export function createButton<
	TMode extends string,
	TRecipe extends ButtonRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ButtonStyleSystem<TMode, TRecipe, TMetadata>,
	options: ButtonOptions = {},
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
		return (props: ButtonProps<TVariant, TSize, TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				disabled = false,
				fluid = false,
				layout,
				loading = false,
				loadingIndicator: loading_indicator,
				loadingLabel: loading_label,
				mix,
				size,
				type = button_type_default,
				variant,
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
					[buttonAnatomy.content]: {
						host: buttonAnatomy.content,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[buttonAnatomy.content],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								buttonAnatomy.content,
								current_style_props,
							);
						},
					},
					[buttonAnatomy.loadingIndicator]: {
						host: buttonAnatomy.loadingIndicator,
						conditions: mergeRecipeConditionSelectors(
							loading_indicator_conditions,
							options.conditions?.[buttonAnatomy.loadingIndicator],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								buttonAnatomy.loadingIndicator,
								current_style_props,
							);
						},
					},
					[buttonAnatomy.loadingIndicatorFrame]: {
						host: buttonAnatomy.loadingIndicatorFrame,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[buttonAnatomy.loadingIndicatorFrame],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								buttonAnatomy.loadingIndicatorFrame,
								current_style_props,
							);
						},
					},
					[buttonAnatomy.root]: {
						host: buttonAnatomy.root,
						conditions: mergeRecipeConditionSelectors(
							button_conditions,
							options.conditions?.[buttonAnatomy.root],
						),
						resolveSlot: (current_style_props) => {
							return resolve_slot(buttonAnatomy.root, current_style_props);
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
					attrs: createComponentAnatomyAttrs(button_scope, buttonAnatomy.root),
					mix: parts.hosts[buttonAnatomy.root].mix,
					props: {
						...button_props,
						"aria-busy": loading || undefined,
						"aria-label": loading
							? (loading_label ?? props["aria-label"])
							: props["aria-label"],
						[componentDataAttribute.disabled]: dataFlag(is_disabled),
						[componentDataAttribute.loading]: dataFlag(loading),
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
							buttonAnatomy.content,
						),
						mix: parts.hosts[buttonAnatomy.content].mix,
					}),
					children,
				),
				loading
					? createElement(
							"span",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									button_scope,
									buttonAnatomy.loadingIndicatorFrame,
								),
								mix: parts.hosts[buttonAnatomy.loadingIndicatorFrame].mix,
								props: {
									"aria-hidden": "true",
								},
							}),
							createElement(
								"span",
								createComponentSlotProps({
									attrs: createComponentAnatomyAttrs(
										button_scope,
										buttonAnatomy.loadingIndicator,
									),
									mix: parts.hosts[buttonAnatomy.loadingIndicator].mix,
								}),
								loading_indicator,
							),
						)
					: null,
			);
		};
	};
}

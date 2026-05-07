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
	type ComponentSlotProps,
} from "./component-style.ts";
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

export type RangeFieldRecipeCondition = "disabled" | "focusVisible";

const range_field_slot = {
	input: "input",
	label: "label",
	progress: "progress",
	root: "root",
	thumb: "thumb",
	track: "track",
	value: "value",
} as const;

export type RangeFieldRecipeSlot =
	(typeof range_field_slot)[keyof typeof range_field_slot];

type RangeFieldHost = "input" | "label" | "root" | "value";

export type RangeFieldRecipeInput<TLayout extends string = string> =
	RecipeWithVariantGroups<
		RangeFieldRecipeSlot,
		string,
		ComponentStyle,
		{
			layout: TLayout;
		}
	>;

export type RangeFieldRecipeLayout<TRecipe extends RangeFieldRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;
export type RangeFieldRecipeSelection<TRecipe extends RangeFieldRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout">;

export type RangeFieldStyleSystem<
	TMode extends string = string,
	TRecipe extends RangeFieldRecipeInput = RangeFieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { rangeField: TRecipe } }, TMetadata>;

export type RangeFieldStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type RangeFieldProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"input">, "style" | "type"> &
	RangeFieldStyleProps<TLayout> &
	ResponsiveProps<RangeFieldStyleProps<TLayout>, TBreakpoint> & {
		displayValue?: RemixNode;
		label: RemixNode;
		labelProps?: ComponentSlotProps<"label">;
		rootProps?: ComponentSlotProps<"div">;
		style?: never;
		valueProps?: ComponentSlotProps<"span">;
	};

export type RangeFieldOptions<TLayout extends string> = {
	defaultLayout?: NoInfer<TLayout>;
};

const input_conditions = {
	disabled: "&:disabled",
	focusVisible: "&:focus-visible",
} as const satisfies RecipeConditionSelectorMap<RangeFieldRecipeCondition>;

const range_field_scope = "rangeField";
const range_field_progress_selectors = ["&::-moz-range-progress"] as const;
const range_field_thumb_selectors = [
	"&::-webkit-slider-thumb",
	"&::-moz-range-thumb",
] as const;
const range_field_track_selectors = [
	"&::-webkit-slider-runnable-track",
	"&::-moz-range-track",
] as const;

export function createRangeField<
	TMode extends string,
	TRecipe extends RangeFieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: RangeFieldStyleSystem<TMode, TRecipe, TMetadata>,
	options: RangeFieldOptions<RangeFieldRecipeLayout<TRecipe>> = {},
): RemixComponent<
	RangeFieldProps<
		RangeFieldRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TLayout = RangeFieldRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const range_field_recipe = createRecipe(
		style_system.token.recipe.rangeField,
	);

	function resolve_slot(
		slot: RangeFieldRecipeSlot,
		props: Partial<RangeFieldStyleProps<TLayout>>,
	): ReturnType<
		typeof range_field_recipe.resolve
	>["slots"][RangeFieldRecipeSlot] {
		return range_field_recipe.resolve({
			layout: props.layout,
		} satisfies RangeFieldRecipeSelection<TRecipe>).slots[slot];
	}

	return () => {
		return (props: RangeFieldProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				displayValue,
				id,
				label,
				labelProps,
				layout = options.defaultLayout,
				rootProps,
				valueProps,
				...input_props
			} = props;
			const { children: _label_children, ...label_props } =
				labelProps ?? {};
			const { children: _root_children, ...root_props } = rootProps ?? {};
			const { children: _value_children, ...value_props } =
				valueProps ?? {};
			const style_props = { layout };
			const targets = createComponentStyleTargets({
				at,
				targets: {
					input: {
						conditions: input_conditions,
						host: "input",
						resolveSlot: (current_style_props) => {
							return resolve_slot("input", current_style_props);
						},
					},
					label: {
						host: "label",
						resolveSlot: (current_style_props) => {
							return resolve_slot("label", current_style_props);
						},
					},
					progress: {
						conditions: input_conditions,
						host: "input",
						resolveSlot: (current_style_props) => {
							return resolve_slot(
								"progress",
								current_style_props,
							);
						},
						selectors: range_field_progress_selectors,
					},
					root: {
						host: "root",
						resolveSlot: (current_style_props) => {
							return resolve_slot("root", current_style_props);
						},
					},
					thumb: {
						conditions: input_conditions,
						host: "input",
						resolveSlot: (current_style_props) => {
							return resolve_slot("thumb", current_style_props);
						},
						selectors: range_field_thumb_selectors,
					},
					track: {
						conditions: input_conditions,
						host: "input",
						resolveSlot: (current_style_props) => {
							return resolve_slot("track", current_style_props);
						},
						selectors: range_field_track_selectors,
					},
					value: {
						host: "value",
						resolveSlot: (current_style_props) => {
							return resolve_slot("value", current_style_props);
						},
					},
				} satisfies Record<
					RangeFieldRecipeSlot,
					{
						conditions?: RecipeConditionSelectorMap<RangeFieldRecipeCondition>;
						host: RangeFieldHost;
						resolveSlot: (
							props: Partial<RangeFieldStyleProps<TLayout>>,
						) => ReturnType<
							typeof range_field_recipe.resolve
						>["slots"][RangeFieldRecipeSlot];
						selectors?: readonly string[];
					}
				>,
				props: style_props,
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						range_field_scope,
						"root",
					),
					mix: targets.hosts.root.mix,
					props: root_props,
				}),
				createElement(
					"label",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							range_field_scope,
							"label",
						),
						mix: targets.hosts.label.mix,
						props: {
							...label_props,
							htmlFor: label_props.htmlFor ?? id,
						},
					}),
					label,
				),
				createElement(
					"input",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							range_field_scope,
							"input",
						),
						mix: targets.hosts.input.mix,
						props: {
							...input_props,
							id,
							type: "range",
						},
					}),
				),
				displayValue === undefined
					? null
					: createElement(
							"span",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									range_field_scope,
									"value",
								),
								mix: targets.hosts.value.mix,
								props: value_props,
							}),
							displayValue,
						),
			);
		};
	};
}

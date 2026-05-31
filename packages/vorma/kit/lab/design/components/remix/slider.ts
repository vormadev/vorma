import {
	createElement,
	createMixin,
	type ElementProps,
	type Props,
	type RemixNode,
} from "remix/ui";
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
import { type RecipeConditionSelectorMap } from "./recipe.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type SliderRecipeCondition = "disabled" | "focusVisible";

const slider_slot = {
	progress: "progress",
	root: "root",
	thumb: "thumb",
	track: "track",
} as const;

export type SliderRecipeSlot = (typeof slider_slot)[keyof typeof slider_slot];

export type SliderStyleTarget =
	| SliderRecipeSlot
	| "thumbMoz"
	| "thumbWebkit"
	| "trackMoz"
	| "trackWebkit";

export type SliderTargetStyles = Partial<
	Readonly<Record<SliderStyleTarget, ComponentStyle>>
>;

type SliderHost = "root";

export type SliderRecipeInput<TLayout extends string = string> = RecipeWithVariantGroups<
	SliderRecipeSlot,
	string,
	ComponentStyle,
	{
		layout: TLayout;
	}
>;

export type SliderRecipeLayout<TRecipe extends SliderRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;
export type SliderRecipeSelection<TRecipe extends SliderRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout">;

export type SliderStyleSystem<
	TMode extends string = string,
	TRecipe extends SliderRecipeInput = SliderRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { slider: TRecipe } }, TMetadata>;

export type SliderStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type SliderProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"input">, "style" | "type"> &
	SliderStyleProps<TLayout> &
	ResponsiveProps<SliderStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type SliderOptions = {
	progressVariable?: string;
	targetStyles?: SliderTargetStyles;
};

const input_conditions = {
	disabled: "&:disabled",
	focusVisible: "&:focus-visible",
} as const satisfies RecipeConditionSelectorMap<SliderRecipeCondition>;

const slider_scope = "slider";
export const sliderProgressVariable = "--vorma-slider-progress";
const slider_progress_selectors = ["&::-moz-range-progress"] as const;
const slider_thumb_selectors = [
	"&::-webkit-slider-thumb",
	"&::-moz-range-thumb",
] as const;
const slider_thumb_moz_selectors = ["&::-moz-range-thumb"] as const;
const slider_thumb_webkit_selectors = ["&::-webkit-slider-thumb"] as const;
const slider_track_selectors = [
	"&::-webkit-slider-runnable-track",
	"&::-moz-range-track",
] as const;
const slider_track_moz_selectors = ["&::-moz-range-track"] as const;
const slider_track_webkit_selectors = ["&::-webkit-slider-runnable-track"] as const;
const slider_empty_slot = {
	base: {},
	conditions: {},
} as const;

function read_range_number(value: string, fallback: number): number {
	const numeric_value = Number(value);
	if (Number.isFinite(numeric_value)) {
		return numeric_value;
	}
	return fallback;
}

function update_slider_progress(node: HTMLInputElement, progress_variable: string): void {
	const min = read_range_number(node.min, 0);
	const max = read_range_number(node.max, 100);
	const fallback_value = min + (max - min) / 2;
	const value = read_range_number(node.value, fallback_value);
	const raw_progress = max <= min ? 0 : ((value - min) / (max - min)) * 100;
	const progress = Math.min(100, Math.max(0, raw_progress));

	node.style.setProperty(progress_variable, `${progress.toFixed(4)}%`);
}

const slider_progress_mixin = createMixin<
	HTMLInputElement,
	[progressVariable: string],
	ElementProps
>((handle) => {
	let current_node: HTMLInputElement | undefined;
	let current_progress_variable = sliderProgressVariable;

	function handle_input(event: Event): void {
		update_slider_progress(
			event.currentTarget as HTMLInputElement,
			current_progress_variable,
		);
	}

	handle.addEventListener("insert", (event) => {
		current_node = event.node;
		current_node.addEventListener("input", handle_input);
		update_slider_progress(current_node, current_progress_variable);
	});
	handle.addEventListener("commit", (event) => {
		update_slider_progress(event.node, current_progress_variable);
	});
	handle.addEventListener("remove", () => {
		current_node?.removeEventListener("input", handle_input);
		current_node = undefined;
	});

	return (progress_variable) => {
		current_progress_variable = progress_variable;
		if (current_node) {
			update_slider_progress(current_node, current_progress_variable);
		}
		return handle.element;
	};
});

export function createSlider<
	TMode extends string,
	TRecipe extends SliderRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SliderStyleSystem<TMode, TRecipe, TMetadata>,
	options: SliderOptions = {},
): RemixComponent<
	SliderProps<
		SliderRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TLayout = SliderRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const slider_recipe = createRecipe(style_system.token.recipe.slider);

	function resolve_slot(
		slot: SliderRecipeSlot,
		props: Partial<SliderStyleProps<TLayout>>,
	): ReturnType<typeof slider_recipe.resolve>["slots"][SliderRecipeSlot] {
		return slider_recipe.resolve({
			layout: props.layout,
		} satisfies SliderRecipeSelection<TRecipe>).slots[slot];
	}

	return () => {
		return (props: SliderProps<TLayout, TBreakpoint>): RemixNode => {
			const { at, layout, mix, ...input_props } = props;
			const progress_variable = options.progressVariable ?? sliderProgressVariable;
			const style_props = { layout };
			const targets = createComponentStyleTargets({
				at,
				hostElements: {
					root: "input",
				},
				targets: {
					root: {
						conditions: input_conditions,
						host: "root",
						resolveSlot: (current_style_props) => {
							return resolve_slot("root", current_style_props);
						},
						style: options.targetStyles?.root,
					},
					progress: {
						conditions: input_conditions,
						host: "root",
						resolveSlot: (current_style_props) => {
							return resolve_slot("progress", current_style_props);
						},
						selectors: slider_progress_selectors,
						style: options.targetStyles?.progress,
					},
					thumb: {
						conditions: input_conditions,
						host: "root",
						resolveSlot: (current_style_props) => {
							return resolve_slot("thumb", current_style_props);
						},
						selectors: slider_thumb_selectors,
						style: options.targetStyles?.thumb,
					},
					thumbMoz: {
						host: "root",
						resolveSlot: () => {
							return slider_empty_slot;
						},
						selectors: slider_thumb_moz_selectors,
						style: options.targetStyles?.thumbMoz,
					},
					thumbWebkit: {
						host: "root",
						resolveSlot: () => {
							return slider_empty_slot;
						},
						selectors: slider_thumb_webkit_selectors,
						style: options.targetStyles?.thumbWebkit,
					},
					track: {
						conditions: input_conditions,
						host: "root",
						resolveSlot: (current_style_props) => {
							return resolve_slot("track", current_style_props);
						},
						selectors: slider_track_selectors,
						style: options.targetStyles?.track,
					},
					trackMoz: {
						host: "root",
						resolveSlot: () => {
							return slider_empty_slot;
						},
						selectors: slider_track_moz_selectors,
						style: options.targetStyles?.trackMoz,
					},
					trackWebkit: {
						host: "root",
						resolveSlot: () => {
							return slider_empty_slot;
						},
						selectors: slider_track_webkit_selectors,
						style: options.targetStyles?.trackWebkit,
					},
				} satisfies Record<
					SliderStyleTarget,
					{
						conditions?: RecipeConditionSelectorMap<SliderRecipeCondition>;
						host: SliderHost;
						resolveSlot: (
							props: Partial<SliderStyleProps<TLayout>>,
						) => ReturnType<
							typeof slider_recipe.resolve
						>["slots"][SliderRecipeSlot];
						selectors?: readonly string[];
						style?: ComponentStyle;
					}
				>,
				props: style_props,
				styleSystem: style_system,
			});

			return createElement(
				"input",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(slider_scope, "root"),
					mix: [
						slider_progress_mixin(progress_variable),
						targets.hosts.root.mix,
					],
					props: {
						...input_props,
						mix,
						type: "range",
					},
				}),
			);
		};
	};
}

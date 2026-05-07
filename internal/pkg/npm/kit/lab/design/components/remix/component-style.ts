import { type MixInput, type Props } from "remix/ui";
import {
	mergeRecipeStyles,
	type RecipeStyle,
	type ResolvedRecipeSlot,
} from "../../core/core.ts";
import {
	createRecipeStyle,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import { createResponsiveRecipeStyle } from "./responsive.ts";
import { create_component_style_mix } from "./style.ts";
import type { ComponentStyleSystem } from "./types.ts";

const component_scope_attribute = "data-vorma-scope";
const component_part_attribute = "data-vorma-part";

type ComponentRecipeStyleMetadata = {
	breakpoint?: Record<string, string | number>;
};

export const componentAnatomyAttrs = {
	part: component_part_attribute,
	scope: component_scope_attribute,
} as const;

export type ComponentAnatomyAttrs = {
	readonly [component_part_attribute]: string;
	readonly [component_scope_attribute]: string;
};

export type ComponentSlotProps<TElement extends keyof HTMLElementTagNameMap> =
	Omit<Props<TElement>, "style"> & {
		style?: never;
	};

type ComponentSlotOwnProps = {
	mix?: unknown;
};

type ComponentSlotInputProps<TProps extends object> = TProps &
	ComponentSlotOwnProps;

export function createComponentSlotProps<TProps extends object = {}>(input: {
	attrs: ComponentAnatomyAttrs;
	mix?: unknown;
	props?: ComponentSlotInputProps<TProps>;
}): Omit<ComponentSlotInputProps<TProps>, "mix"> &
	ComponentAnatomyAttrs & { mix: unknown[] } {
	const { mix, ...props } = input.props ?? {};
	const system_mix = Array.isArray(input.mix) ? input.mix : [input.mix];
	const consumer_mix = Array.isArray(mix) ? mix : [mix];

	return {
		...input.attrs,
		...props,
		mix: [...system_mix, ...consumer_mix],
	} as Omit<ComponentSlotInputProps<TProps>, "mix"> &
		ComponentAnatomyAttrs & { mix: unknown[] };
}

export type ComponentStyleTargetRecipeInput<
	TProps extends object,
	TCondition extends string,
> = {
	conditions?: RecipeConditionSelectorMap;
	resolveSlot: (
		props: Partial<TProps>,
	) => ResolvedRecipeSlot<TCondition, RecipeStyle>;
	resolveStyle?: (props: Partial<TProps>) => RecipeStyle | undefined;
	style?: RecipeStyle;
};

export type ComponentStyleTargetInput<
	TProps extends object,
	THost extends string,
	TCondition extends string,
> = ComponentStyleTargetRecipeInput<TProps, TCondition> & {
	host: THost;
	selectors?: readonly string[];
};

export type ComponentStyleTargetsInput<
	TProps extends object,
	THost extends string,
	TTarget extends string,
	TCondition extends string,
	TMode extends string,
	TToken,
	TMetadata extends ComponentRecipeStyleMetadata,
> = {
	at?: Partial<Record<string, Partial<TProps>>>;
	props: Partial<TProps>;
	styleSystem: ComponentStyleSystem<TMode, TToken, TMetadata>;
	targets: Record<
		TTarget,
		ComponentStyleTargetInput<TProps, THost, TCondition>
	>;
};

export type ComponentStyleTargetOutput = {
	mix: MixInput<Element>;
	style: RecipeStyle;
};

export type ComponentStyleTargetsOutput<
	THost extends string,
	TTarget extends string,
> = {
	readonly hosts: {
		readonly [K in THost]: ComponentStyleTargetOutput;
	};
	readonly targets: {
		readonly [K in TTarget]: ComponentStyleTargetOutput;
	};
};

export function createComponentAnatomyAttrs(
	scope: string,
	part: string,
): ComponentAnatomyAttrs {
	return {
		[component_part_attribute]: part,
		[component_scope_attribute]: scope,
	};
}

function is_recipe_style(value: unknown): value is RecipeStyle {
	return value !== null && typeof value === "object" && !Array.isArray(value);
}

function merge_style_value(
	style: Record<string, unknown>,
	key: string,
	value: unknown,
): void {
	const current = style[key];
	if (is_recipe_style(current) && is_recipe_style(value)) {
		style[key] = mergeRecipeStyles(current, value);
		return;
	}
	style[key] = value;
}

function is_style_at_rule(key: string): boolean {
	return key.startsWith("@");
}

function is_style_selector(key: string): boolean {
	return key.startsWith("&");
}

function combine_style_selectors(
	condition_selector: string,
	target_selector: string,
): string {
	if (target_selector.includes("&")) {
		return target_selector.replaceAll("&", condition_selector);
	}
	return `${condition_selector} ${target_selector}`;
}

function scope_style_to_selectors(
	style: RecipeStyle,
	selectors: readonly string[],
): RecipeStyle {
	const scoped: Record<string, unknown> = {};

	for (const [key, value] of Object.entries(style)) {
		if (is_style_at_rule(key) && is_recipe_style(value)) {
			merge_style_value(
				scoped,
				key,
				scope_style_to_selectors(value, selectors),
			);
			continue;
		}

		if (is_style_selector(key) && is_recipe_style(value)) {
			for (const selector of selectors) {
				merge_style_value(
					scoped,
					combine_style_selectors(key, selector),
					value,
				);
			}
			continue;
		}

		for (const selector of selectors) {
			const selector_style = scoped[selector];
			const next_selector_style: Record<string, unknown> =
				is_recipe_style(selector_style) ? { ...selector_style } : {};
			merge_style_value(next_selector_style, key, value);
			merge_style_value(scoped, selector, next_selector_style);
		}
	}

	return scoped;
}

function create_component_style_target_slot<
	TProps extends object,
	TCondition extends string,
>(
	target: ComponentStyleTargetRecipeInput<TProps, TCondition>,
	props: Partial<TProps>,
): ResolvedRecipeSlot<TCondition, RecipeStyle> {
	const slot = target.resolveSlot(props);

	return {
		base: mergeRecipeStyles(slot.base, target.resolveStyle?.(props)),
		conditions: slot.conditions,
	};
}

function create_component_style_target_recipe_style<
	TProps extends object,
	TCondition extends string,
	TMode extends string,
	TToken,
	TMetadata extends ComponentRecipeStyleMetadata,
>(
	input: {
		at?: Partial<Record<string, Partial<TProps>>>;
		props: Partial<TProps>;
		styleSystem: ComponentStyleSystem<TMode, TToken, TMetadata>;
	},
	target: ComponentStyleTargetRecipeInput<TProps, TCondition>,
): RecipeStyle {
	return createRecipeStyle({
		conditions: target.conditions,
		slot: create_component_style_target_slot(target, input.props),
		style: mergeRecipeStyles(
			target.style,
			createResponsiveRecipeStyle({
				at: input.at,
				conditions: target.conditions,
				resolve: (props) => {
					return create_component_style_target_slot(target, {
						...input.props,
						...props,
					});
				},
				styleSystem: input.styleSystem,
			}),
		),
	});
}

function create_component_style_output(
	style: RecipeStyle,
): ComponentStyleTargetOutput {
	return {
		mix: create_component_style_mix(style),
		style,
	};
}

function create_component_style_target_style<
	TProps extends object,
	THost extends string,
	TTarget extends string,
	TCondition extends string,
	TMode extends string,
	TToken,
	TMetadata extends ComponentRecipeStyleMetadata,
>(
	input: ComponentStyleTargetsInput<
		TProps,
		THost,
		TTarget,
		TCondition,
		TMode,
		TToken,
		TMetadata
	>,
	target: ComponentStyleTargetInput<TProps, THost, TCondition>,
): RecipeStyle {
	const style = create_component_style_target_recipe_style(input, target);
	if (target.selectors === undefined || target.selectors.length === 0) {
		return style;
	}
	return scope_style_to_selectors(style, target.selectors);
}

export function createComponentStyleTargets<
	TProps extends object,
	THost extends string,
	TTarget extends string,
	TCondition extends string,
	TMode extends string,
	TToken,
	TMetadata extends ComponentRecipeStyleMetadata,
>(
	input: ComponentStyleTargetsInput<
		TProps,
		THost,
		TTarget,
		TCondition,
		TMode,
		TToken,
		TMetadata
	>,
): ComponentStyleTargetsOutput<THost, TTarget> {
	const host_styles: Partial<Record<THost, RecipeStyle>> = {};
	const target_outputs: Partial<Record<TTarget, ComponentStyleTargetOutput>> =
		{};

	for (const [target_name, target] of Object.entries(input.targets) as [
		TTarget,
		ComponentStyleTargetInput<TProps, THost, TCondition>,
	][]) {
		const target_style = create_component_style_target_style(input, target);
		target_outputs[target_name] =
			create_component_style_output(target_style);
		host_styles[target.host] = mergeRecipeStyles(
			host_styles[target.host],
			target_style,
		);
	}

	return {
		hosts: Object.fromEntries(
			Object.entries(host_styles).map(([host, style]) => {
				return [
					host,
					create_component_style_output(style as RecipeStyle),
				];
			}),
		) as ComponentStyleTargetsOutput<THost, TTarget>["hosts"],
		targets: target_outputs as ComponentStyleTargetsOutput<
			THost,
			TTarget
		>["targets"],
	};
}

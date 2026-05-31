import { css, type MixInput, type Props } from "remix/ui";
import {
	mergeRecipeStyles,
	type RecipeStyle,
	type ResolvedRecipeSlot,
} from "../../core/core.ts";
import { createRecipeStyle, type RecipeConditionSelectorMap } from "./recipe.ts";
import { createResponsiveRecipeStyle } from "./responsive.ts";
import type { ComponentStyleSystem } from "./types.ts";

const component_scope_attribute = "data-vorma-scope";
const component_part_attribute = "data-vorma-part";

type ComponentRecipeStyleMetadata = {
	breakpoint?: Record<string, string | number>;
};

type ComponentStyleHostElementName = keyof HTMLElementTagNameMap;

export type ComponentStyleHostElementMap<THost extends string> = Partial<
	Readonly<Record<THost, ComponentStyleHostElementName>>
>;

type ComponentStyleHostElement<
	THost extends string,
	THostElements extends ComponentStyleHostElementMap<THost>,
	TKey extends THost,
> = TKey extends keyof THostElements
	? THostElements[TKey] extends ComponentStyleHostElementName
		? HTMLElementTagNameMap[THostElements[TKey]]
		: Element
	: Element;

export const componentAnatomyAttrs = {
	part: component_part_attribute,
	scope: component_scope_attribute,
} as const;

export type ComponentAnatomyAttrs = {
	readonly [component_part_attribute]: string;
	readonly [component_scope_attribute]: string;
};

export type ComponentSlotProps<TElement extends keyof HTMLElementTagNameMap> = Omit<
	Props<TElement>,
	"style"
> & {
	style?: never;
};

type ComponentSlotOwnProps = {
	mix?: unknown;
};

type ComponentSlotInputProps<TProps extends object> = TProps & ComponentSlotOwnProps;

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
	resolveSlot: (props: Partial<TProps>) => ResolvedRecipeSlot<TCondition, RecipeStyle>;
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
	THostElements extends ComponentStyleHostElementMap<THost> = {},
> = {
	at?: Partial<Record<string, Partial<TProps>>>;
	hostElements?: THostElements;
	props: Partial<TProps>;
	styleSystem: ComponentStyleSystem<TMode, TToken, TMetadata>;
	targets: Record<TTarget, ComponentStyleTargetInput<TProps, THost, TCondition>>;
};

export type ComponentStyleTargetOutput<TElement extends Element = Element> = {
	mix: MixInput<TElement>;
	style: RecipeStyle;
};

export type ComponentStyleTargetsOutput<
	THost extends string,
	TTarget extends string,
	THostElements extends ComponentStyleHostElementMap<THost> = {},
> = {
	readonly hosts: {
		readonly [K in THost]: ComponentStyleTargetOutput<
			ComponentStyleHostElement<THost, THostElements, K>
		>;
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
		style[key] = merge_component_styles(current, value);
		return;
	}
	style[key] = value;
}

function merge_component_styles(
	...styles: ReadonlyArray<RecipeStyle | undefined>
): RecipeStyle {
	const merged: Record<string, unknown> = {};
	for (const style of styles) {
		if (style === undefined) {
			continue;
		}
		for (const [key, value] of Object.entries(style)) {
			merge_style_value(merged, key, value);
		}
	}
	return merged;
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
			merge_style_value(scoped, key, scope_style_to_selectors(value, selectors));
			continue;
		}

		if (is_style_selector(key) && is_recipe_style(value)) {
			for (const selector of selectors) {
				merge_style_value(scoped, combine_style_selectors(key, selector), value);
			}
			continue;
		}

		for (const selector of selectors) {
			const selector_style = scoped[selector];
			const next_selector_style: Record<string, unknown> = is_recipe_style(
				selector_style,
			)
				? { ...selector_style }
				: {};
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

function create_component_style_output<TElement extends Element = Element>(
	style: RecipeStyle,
): ComponentStyleTargetOutput<TElement> {
	return {
		mix: css<TElement>(style as Parameters<typeof css>[0]),
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
	THostElements extends ComponentStyleHostElementMap<THost>,
>(
	input: ComponentStyleTargetsInput<
		TProps,
		THost,
		TTarget,
		TCondition,
		TMode,
		TToken,
		TMetadata,
		THostElements
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
	THostElements extends ComponentStyleHostElementMap<THost> = {},
>(
	input: ComponentStyleTargetsInput<
		TProps,
		THost,
		TTarget,
		TCondition,
		TMode,
		TToken,
		TMetadata,
		THostElements
	>,
): ComponentStyleTargetsOutput<THost, TTarget, THostElements> {
	const host_styles: Partial<Record<THost, RecipeStyle>> = {};
	const target_outputs: Partial<Record<TTarget, ComponentStyleTargetOutput>> = {};

	for (const [target_name, target] of Object.entries(input.targets) as [
		TTarget,
		ComponentStyleTargetInput<TProps, THost, TCondition>,
	][]) {
		const target_style = create_component_style_target_style(input, target);
		target_outputs[target_name] = create_component_style_output(target_style);
		host_styles[target.host] = merge_component_styles(
			host_styles[target.host],
			target_style,
		);
	}

	return {
		hosts: Object.fromEntries(
			Object.entries(host_styles).map(([host, style]) => {
				return [host, create_component_style_output(style as RecipeStyle)];
			}),
		) as unknown as ComponentStyleTargetsOutput<
			THost,
			TTarget,
			THostElements
		>["hosts"],
		targets: target_outputs as ComponentStyleTargetsOutput<
			THost,
			TTarget,
			THostElements
		>["targets"],
	};
}

import {
	createElement,
	on,
	type Handle,
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
import {
	componentStateAttribute,
	openState,
	openStateFromBoolean,
} from "./component-state.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
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

export type AccordionRecipeCondition =
	| CommonRecipeCondition
	| "closed"
	| "open";

export type AccordionRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"item" | "panel" | "root" | "trigger",
	string,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type AccordionRecipeLayout<TRecipe extends AccordionRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type AccordionRecipeVariant<TRecipe extends AccordionRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type AccordionRecipeSize<TRecipe extends AccordionRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type AccordionRecipeSelection<TRecipe extends AccordionRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout" | "size" | "variant">;

export type AccordionStyleSystem<
	TMode extends string = string,
	TRecipe extends AccordionRecipeInput = AccordionRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { accordion: TRecipe } }, TMetadata>;

export type AccordionValue = string | readonly string[] | null;

export type AccordionValueChangeDetails = {
	event?: Event;
};

export type AccordionValueChangeHandler = (
	value: AccordionValue,
	details?: AccordionValueChangeDetails,
) => void;

export type AccordionRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type AccordionItemStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type AccordionRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	AccordionRootStyleProps<TLayout> &
	ResponsiveProps<AccordionRootStyleProps<TLayout>, TBreakpoint> & {
		defaultValue?: AccordionValue;
		multiple?: boolean;
		onValueChange?: AccordionValueChangeHandler;
		style?: never;
		value?: AccordionValue;
	};

export type AccordionItemProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	AccordionItemStyleProps<TVariant, TSize> &
	ResponsiveProps<AccordionItemStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
		value: string;
	};

export type AccordionTriggerProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type AccordionPanelProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type AccordionComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	Item: RemixComponent<
		AccordionItemProps<TVariant, TSize>,
		AccordionItemContext
	>;
	Panel: RemixComponent<AccordionPanelProps>;
	Root: RemixComponent<AccordionRootProps<TLayout>, AccordionRootContext>;
	Trigger: RemixComponent<AccordionTriggerProps>;
};

type AccordionRootContext = {
	get_multiple: () => boolean;
	is_open: (value: string) => boolean;
	toggle_value: (
		value: string,
		details?: AccordionValueChangeDetails,
	) => void;
};

type AccordionItemContext = {
	get_at: () =>
		| Partial<Record<string, Partial<AccordionItemStyleProps>>>
		| undefined;
	get_open: () => boolean;
	get_panel_id: () => string;
	get_style_props: () => Partial<AccordionItemStyleProps>;
	get_trigger_id: () => string;
	get_value: () => string;
	toggle: (details?: AccordionValueChangeDetails) => void;
};

const accordion_scope = "accordion";
const button_type_default = "button";
const click_event = "click";

const accordion_conditions =
	mergeRecipeConditionSelectors<AccordionRecipeCondition>(
		commonConditions,
		{
			closed: `&[${componentStateAttribute}='${openState.closed}']`,
			open: `&[${componentStateAttribute}='${openState.open}']`,
		} satisfies RecipeConditionSelectorMap<AccordionRecipeCondition>,
	);

function normalize_accordion_values(value: AccordionValue): readonly string[] {
	if (value === null) {
		return [];
	}
	if (typeof value === "string") {
		return [value];
	}
	return value;
}

function format_accordion_value(
	values: readonly string[],
	multiple: boolean,
): AccordionValue {
	return multiple ? values : (values[0] ?? null);
}

function toggle_accordion_value(
	values: readonly string[],
	value: string,
	multiple: boolean,
): readonly string[] {
	if (values.includes(value)) {
		return values.filter((current_value) => {
			return current_value !== value;
		});
	}
	if (multiple) {
		return [...values, value];
	}
	return [value];
}

export function createAccordion<
	TMode extends string,
	TRecipe extends AccordionRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: AccordionStyleSystem<TMode, TRecipe, TMetadata>,
): AccordionComponents<
	AccordionRecipeLayout<TRecipe>,
	AccordionRecipeVariant<TRecipe>,
	AccordionRecipeSize<TRecipe>
> {
	type TLayout = AccordionRecipeLayout<TRecipe>;
	type TVariant = AccordionRecipeVariant<TRecipe>;
	type TSize = AccordionRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = AccordionRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.accordion);

	function resolve_slot(
		slot: "item" | "panel" | "root" | "trigger",
		props: Partial<AccordionRootStyleProps<TLayout>> &
			Partial<AccordionItemStyleProps<TVariant, TSize>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(
		handle: Handle<AccordionRootProps<TLayout, TBreakpoint>, AccordionRootContext>,
	): (props: AccordionRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_values = normalize_accordion_values(
			handle.props.defaultValue ?? null,
		);

		function get_multiple(): boolean {
			return handle.props.multiple === true;
		}

		function get_values(): readonly string[] {
			return normalize_accordion_values(handle.props.value ?? local_values);
		}

		const context: AccordionRootContext = {
			get_multiple,
			is_open: (value) => {
				return get_values().includes(value);
			},
			toggle_value: (value, details) => {
				const next_values = toggle_accordion_value(
					get_values(),
					value,
					get_multiple(),
				);
				if (handle.props.value === undefined) {
					local_values = [...next_values];
				}
				handle.props.onValueChange?.(
					format_accordion_value(next_values, get_multiple()),
					details,
				);
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: AccordionRootProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultValue: _default_value,
				layout,
				mix,
				multiple: _multiple,
				onValueChange: _on_value_change,
				value: _value,
				...root_props
			} = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: accordion_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("root", current_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						accordion_scope,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: {
						...root_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Item(
		handle: Handle<AccordionItemProps<TVariant, TSize, TBreakpoint>, AccordionItemContext>,
	): (props: AccordionItemProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const root_context = handle.context.get(Root);
		const context: AccordionItemContext = {
			get_at: () => {
				return handle.props.at;
			},
			get_open: () => {
				return root_context.is_open(handle.props.value);
			},
			get_panel_id: () => {
				return `${handle.id}-panel`;
			},
			get_style_props: () => {
				return {
					size: handle.props.size,
					variant: handle.props.variant,
				};
			},
			get_trigger_id: () => {
				return `${handle.id}-trigger`;
			},
			get_value: () => {
				return handle.props.value;
			},
			toggle: (details) => {
				root_context.toggle_value(handle.props.value, details);
			},
		};
		handle.context.set(context);

		return (props: AccordionItemProps<TVariant, TSize, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				mix,
				size,
				value: _value,
				variant,
				...item_props
			} = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				targets: {
					item: {
						host: "item",
						conditions: accordion_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("item", current_props);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						accordion_scope,
						"item",
					),
					mix: parts.hosts.item.mix,
					props: {
						...item_props,
						[componentStateAttribute]:
							openStateFromBoolean(open),
						mix,
					},
				}),
				children,
			);
		};
	}

	function Trigger(
		handle: Handle<AccordionTriggerProps>,
	): (props: AccordionTriggerProps) => RemixNode {
		const item_context = handle.context.get(Item);

		return (props: AccordionTriggerProps): RemixNode => {
			const {
				children,
				mix,
				type = button_type_default,
				...trigger_props
			} = props;
			const open = item_context.get_open();
			const parts = createComponentStyleTargets({
				at: item_context.get_at(),
				targets: {
					trigger: {
						host: "trigger",
						conditions: accordion_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("trigger", current_props);
						},
					},
				},
				props: item_context.get_style_props(),
				styleSystem: style_system,
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						accordion_scope,
						"trigger",
					),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								item_context.toggle({ event });
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-controls": item_context.get_panel_id(),
						"aria-expanded": open,
						[componentStateAttribute]: openStateFromBoolean(open),
						id: trigger_props.id ?? item_context.get_trigger_id(),
						mix,
						type,
					},
				}),
				children,
			);
		};
	}

	function Panel(
		handle: Handle<AccordionPanelProps>,
	): (props: AccordionPanelProps) => RemixNode {
		const item_context = handle.context.get(Item);

		return (props: AccordionPanelProps): RemixNode => {
			const { children, mix, ...panel_props } = props;
			const open = item_context.get_open();
			const parts = createComponentStyleTargets({
				at: item_context.get_at(),
				targets: {
					panel: {
						host: "panel",
						conditions: accordion_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("panel", current_props);
						},
					},
				},
				props: item_context.get_style_props(),
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						accordion_scope,
						"panel",
					),
					mix: parts.hosts.panel.mix,
					props: {
						...panel_props,
						"aria-labelledby": item_context.get_trigger_id(),
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						id: panel_props.id ?? item_context.get_panel_id(),
						mix,
						role: panel_props.role ?? "region",
					},
				}),
				children,
			);
		};
	}

	return {
		Item,
		Panel,
		Root,
		Trigger,
	};
}

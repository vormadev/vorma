import { createElement, on, type Handle, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	componentStateAttribute,
	selectionState,
	selectionStateFromBoolean,
} from "./component-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type TabsRecipeCondition = CommonRecipeCondition | "selected" | "unselected";

export type TabsRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"list" | "panel" | "root" | "trigger",
	string,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type TabsRecipeLayout<TRecipe extends TabsRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;

export type TabsRecipeVariant<TRecipe extends TabsRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type TabsRecipeSize<TRecipe extends TabsRecipeInput> = RecipeVariantValue<
	TRecipe,
	"size"
>;

export type TabsRecipeSelection<TRecipe extends TabsRecipeInput> = RecipeVariantPropsFor<
	TRecipe,
	"layout" | "size" | "variant"
>;

export type TabsStyleSystem<
	TMode extends string = string,
	TRecipe extends TabsRecipeInput = TabsRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { tabs: TRecipe } }, TMetadata>;

export type TabsValueChangeDetails = {
	event?: Event;
};

export type TabsValueChangeHandler<TValue extends string = string> = (
	value: TValue,
	details?: TabsValueChangeDetails,
) => void;

export type TabsRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type TabsTriggerStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type TabsRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"div">, "style"> &
	TabsRootStyleProps<TLayout> &
	ResponsiveProps<TabsRootStyleProps<TLayout>, TBreakpoint> & {
		defaultValue?: TValue | null;
		onValueChange?: TabsValueChangeHandler<TValue>;
		style?: never;
		value?: TValue | null;
	};

export type TabsListProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type TabsTriggerProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"button">, "style" | "value"> &
	TabsTriggerStyleProps<TVariant, TSize> &
	ResponsiveProps<TabsTriggerStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
		value: TValue;
	};

export type TabsPanelProps<TValue extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	style?: never;
	value: TValue;
};

export type TabsComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	List: RemixComponent<TabsListProps>;
	Panel: RemixComponent<TabsPanelProps>;
	Root: RemixComponent<TabsRootProps<TLayout>, TabsRootContext>;
	Trigger: RemixComponent<TabsTriggerProps<TVariant, TSize>>;
};

type TabsRootContext = {
	get_panel_id: (value: string) => string;
	get_trigger_id: (value: string) => string;
	get_value: () => string | null;
	set_value: (value: string, details?: TabsValueChangeDetails) => void;
};

const tabs_scope = "tabs";
const button_type_default = "button";
const click_event = "click";

const tabs_conditions = mergeRecipeConditionSelectors<TabsRecipeCondition>(
	commonConditions,
	{
		selected: `&[${componentStateAttribute}='${selectionState.selected}']`,
		unselected: `&[${componentStateAttribute}='${selectionState.unselected}']`,
	} satisfies RecipeConditionSelectorMap<TabsRecipeCondition>,
);

function value_id_segment(value: string): string {
	return value.replaceAll(/[^A-Za-z0-9_-]/g, "-");
}

export function createTabs<
	TMode extends string,
	TRecipe extends TabsRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TabsStyleSystem<TMode, TRecipe, TMetadata>,
): TabsComponents<
	TabsRecipeLayout<TRecipe>,
	TabsRecipeVariant<TRecipe>,
	TabsRecipeSize<TRecipe>
> {
	type TLayout = TabsRecipeLayout<TRecipe>;
	type TVariant = TabsRecipeVariant<TRecipe>;
	type TSize = TabsRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = TabsRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.tabs);

	function resolve_slot(
		slot: "list" | "panel" | "root" | "trigger",
		props: Partial<TabsRootStyleProps<TLayout>> &
			Partial<TabsTriggerStyleProps<TVariant, TSize>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(
		handle: Handle<TabsRootProps<TLayout, TBreakpoint>, TabsRootContext>,
	): (props: TabsRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_value = handle.props.defaultValue ?? null;

		function get_value(): string | null {
			return handle.props.value ?? local_value;
		}

		const context: TabsRootContext = {
			get_panel_id: (value) => {
				return `${handle.id}-${value_id_segment(value)}-panel`;
			},
			get_trigger_id: (value) => {
				return `${handle.id}-${value_id_segment(value)}-trigger`;
			},
			get_value,
			set_value: (value, details) => {
				if (get_value() === value) {
					return;
				}
				if (handle.props.value === undefined) {
					local_value = value;
				}
				handle.props.onValueChange?.(value, details);
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: TabsRootProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultValue: _default_value,
				layout,
				mix,
				onValueChange: _on_value_change,
				value: _value,
				...root_props
			} = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: tabs_conditions,
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
					attrs: createComponentAnatomyAttrs(tabs_scope, "root"),
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

	function List(): (props: TabsListProps) => RemixNode {
		return (props: TabsListProps): RemixNode => {
			const { children, mix, ...list_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					list: {
						host: "list",
						conditions: tabs_conditions,
						resolveSlot: () => {
							return resolve_slot("list", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(tabs_scope, "list"),
					mix: parts.hosts.list.mix,
					props: {
						...list_props,
						mix,
						role: list_props.role ?? "tablist",
					},
				}),
				children,
			);
		};
	}

	function Trigger(
		handle: Handle<TabsTriggerProps<TVariant, TSize, TBreakpoint>>,
	): (props: TabsTriggerProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (props: TabsTriggerProps<TVariant, TSize, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				mix,
				size,
				type = button_type_default,
				value,
				variant,
				...trigger_props
			} = props;
			const selected = context.get_value() === value;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					trigger: {
						host: "trigger",
						conditions: tabs_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("trigger", current_props);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(tabs_scope, "trigger"),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								context.set_value(value, { event });
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-controls": context.get_panel_id(value),
						"aria-selected": selected,
						[componentStateAttribute]: selectionStateFromBoolean(selected),
						id: trigger_props.id ?? context.get_trigger_id(value),
						mix,
						role: trigger_props.role ?? "tab",
						tabIndex: trigger_props.tabIndex ?? (selected ? 0 : -1),
						type,
					},
				}),
				children,
			);
		};
	}

	function Panel(handle: Handle<TabsPanelProps>): (props: TabsPanelProps) => RemixNode {
		const context = handle.context.get(Root);

		return (props: TabsPanelProps): RemixNode => {
			const { children, mix, value, ...panel_props } = props;
			const selected = context.get_value() === value;
			const parts = createComponentStyleTargets({
				targets: {
					panel: {
						host: "panel",
						conditions: tabs_conditions,
						resolveSlot: () => {
							return resolve_slot("panel", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(tabs_scope, "panel"),
					mix: parts.hosts.panel.mix,
					props: {
						...panel_props,
						"aria-labelledby": context.get_trigger_id(value),
						[componentStateAttribute]: selectionStateFromBoolean(selected),
						hidden: !selected,
						id: panel_props.id ?? context.get_panel_id(value),
						mix,
						role: panel_props.role ?? "tabpanel",
					},
				}),
				children,
			);
		};
	}

	return {
		List,
		Panel,
		Root,
		Trigger,
	};
}

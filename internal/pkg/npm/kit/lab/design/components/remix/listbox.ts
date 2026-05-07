import {
	createElement,
	on,
	ref,
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
	get_collection_navigation_item,
	get_first_collection_item,
	get_last_collection_item,
} from "./composite-navigation.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
import { create_controllable_state } from "./controllable-state.ts";
import {
	create_group_label_ref_mix,
	create_group_label_relationship,
	type GroupLabelRelationship,
} from "./group-label.ts";
import {
	create_ordered_collection,
	type OrderedCollectionItem,
} from "./ordered-collection.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import { create_typeahead } from "./typeahead.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

const listbox_slot = {
	group: "group",
	groupLabel: "groupLabel",
	option: "option",
	optionIndicator: "optionIndicator",
	optionText: "optionText",
	root: "root",
} as const;

export type ListboxRecipeSlot =
	(typeof listbox_slot)[keyof typeof listbox_slot];

export type ListboxRecipeCondition = CommonRecipeCondition | "highlighted";

export type ListboxRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	ListboxRecipeSlot,
	string,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type ListboxRecipeLayout<TRecipe extends ListboxRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type ListboxRecipeVariant<TRecipe extends ListboxRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type ListboxRecipeSize<TRecipe extends ListboxRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type ListboxRecipeSelection<TRecipe extends ListboxRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout" | "size" | "variant">;

export type ListboxStyleSystem<
	TMode extends string = string,
	TRecipe extends ListboxRecipeInput = ListboxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			listbox: TRecipe;
		};
	},
	TMetadata
>;

export const listboxValueChangeReason = {
	keyboard: "keyboard",
	pointer: "pointer",
} as const;

export const listboxHighlightChangeReason = {
	focus: "focus",
	keyboard: "keyboard",
	pointer: "pointer",
	typeahead: "typeahead",
} as const;

export type ListboxValueChangeReason =
	(typeof listboxValueChangeReason)[keyof typeof listboxValueChangeReason];

export type ListboxHighlightChangeReason =
	(typeof listboxHighlightChangeReason)[keyof typeof listboxHighlightChangeReason];

export type ListboxValueChangeDetails = {
	event?: Event;
	reason: ListboxValueChangeReason;
};

export type ListboxHighlightChangeDetails = {
	event?: Event;
	reason: ListboxHighlightChangeReason;
};

export type ListboxValueChangeHandler<TValue extends string = string> = (
	value: TValue | null,
	details?: ListboxValueChangeDetails,
) => void;

export type ListboxHighlightChangeHandler<TValue extends string = string> = (
	value: TValue | null,
	details?: ListboxHighlightChangeDetails,
) => void;

export type ListboxValuesChangeHandler<TValue extends string = string> = (
	values: readonly TValue[],
	details?: ListboxValueChangeDetails,
) => void;

export type ListboxSelectionMode = "multiple" | "single";

export type ListboxRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type ListboxOrientation = "horizontal" | "vertical";

export type ListboxRootSharedProps<TValue extends string = string> = {
	defaultHighlightedValue?: TValue | null;
	disabled?: boolean;
	highlightedValue?: TValue | null;
	invalid?: boolean;
	loopFocus?: boolean;
	onHighlightChange?: ListboxHighlightChangeHandler<TValue>;
	orientation?: ListboxOrientation;
	required?: boolean;
	style?: never;
	typeahead?: boolean;
};

export type ListboxRootSingleSelectionProps<TValue extends string = string> = {
	defaultValue?: TValue | null;
	defaultValues?: never;
	onValueChange?: ListboxValueChangeHandler<TValue>;
	onValuesChange?: never;
	selectionFollowsFocus?: boolean;
	selectionMode?: "single";
	value?: TValue | null;
	values?: never;
};

export type ListboxRootMultipleSelectionProps<TValue extends string = string> =
	{
		defaultValue?: never;
		defaultValues?: readonly TValue[];
		onValueChange?: never;
		onValuesChange?: ListboxValuesChangeHandler<TValue>;
		selectionFollowsFocus?: never;
		selectionMode: "multiple";
		value?: never;
		values?: readonly TValue[];
	};

export type ListboxRootSelectionProps<TValue extends string = string> =
	| ListboxRootSingleSelectionProps<TValue>
	| ListboxRootMultipleSelectionProps<TValue>;

export type ListboxOptionStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type ListboxRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"div">, "style"> &
	ListboxRootStyleProps<TLayout> &
	ResponsiveProps<ListboxRootStyleProps<TLayout>, TBreakpoint> &
	ListboxRootSharedProps<TValue> &
	ListboxRootSelectionProps<TValue>;

export type ListboxOptionProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"div">, "style"> &
	ListboxOptionStyleProps<TVariant, TSize> &
	ResponsiveProps<ListboxOptionStyleProps<TVariant, TSize>, TBreakpoint> & {
		disabled?: boolean;
		style?: never;
		textValue?: string;
		value: TValue;
	};

export type ListboxGroupProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type ListboxGroupLabelProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type ListboxOptionTextProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type ListboxOptionIndicatorProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type ListboxComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	Group: RemixComponent<ListboxGroupProps, ListboxGroupRuntimeContext>;
	GroupLabel: RemixComponent<ListboxGroupLabelProps>;
	Option: RemixComponent<ListboxOptionProps<TVariant, TSize>>;
	OptionIndicator: RemixComponent<ListboxOptionIndicatorProps>;
	OptionText: RemixComponent<ListboxOptionTextProps>;
	Root: RemixComponent<ListboxRootProps<TLayout>, ListboxRuntimeContext>;
};

export type ListboxOptions = {
	conditions?: Partial<
		Readonly<Record<ListboxRecipeSlot, RecipeConditionSelectorMap<string>>>
	>;
};

type ListboxRuntimeContext = {
	get_disabled: () => boolean;
	get_highlighted_id: () => string | undefined;
	get_highlighted_value: () => string | null;
	get_invalid: () => boolean;
	get_required: () => boolean;
	get_value: () => string | null;
	handle_focus: (event: FocusEvent) => void;
	handle_keydown: (event: KeyboardEvent) => void;
	highlight_value: (
		value: string | null,
		details: ListboxHighlightChangeDetails,
	) => void;
	register_option: (option: RegisteredListboxOption) => void;
	select_value: (
		value: string | null,
		details: ListboxValueChangeDetails,
	) => void;
	value_selected: (value: string) => boolean;
	unregister_option: (id: string) => void;
};

type ListboxGroupRuntimeContext = GroupLabelRelationship;

type RegisteredListboxOption = OrderedCollectionItem<string>;

const option_conditions = mergeRecipeConditionSelectors<ListboxRecipeCondition>(
	commonConditions,
	{
		highlighted: "&[data-highlighted]",
	} satisfies RecipeConditionSelectorMap<ListboxRecipeCondition>,
);

const listbox_scope = "listbox";
const listbox_page_jump_size = 10;
const listbox_typeahead_reset_ms = 700;
const multiple_selection_mode = "multiple";
const single_selection_mode = "single";
const vertical_orientation = "vertical";
const focus_event = "focus";
const click_event = "click";
const keydown_event = "keydown";
const pointermove_event = "pointermove";

function infer_text_value(children: RemixNode): string {
	if (typeof children === "string" || typeof children === "number") {
		return String(children);
	}
	return "";
}

function is_listbox_keyboard_selection(event: KeyboardEvent): boolean {
	return (
		event.key === "Enter" || event.key === " " || event.key === "Spacebar"
	);
}

function is_printable_key_event(event: KeyboardEvent): boolean {
	return (
		event.key.length === 1 &&
		!event.altKey &&
		!event.ctrlKey &&
		!event.metaKey
	);
}

function coerce_listbox_value(value: string | null | undefined): string | null {
	return value ?? null;
}

function coerce_listbox_values(
	values: readonly string[] | undefined,
): readonly string[] {
	return values ?? [];
}

function add_value(
	values: readonly string[],
	value: string,
): readonly string[] {
	if (values.includes(value)) {
		return values;
	}
	return [...values, value];
}

function remove_value(
	values: readonly string[],
	value: string,
): readonly string[] {
	return values.filter((current_value) => {
		return current_value !== value;
	});
}

function toggle_value(
	values: readonly string[],
	value: string,
): readonly string[] {
	if (values.includes(value)) {
		return remove_value(values, value);
	}
	return add_value(values, value);
}

function is_same_values(
	left: readonly string[],
	right: readonly string[],
): boolean {
	return (
		left.length === right.length &&
		left.every((left_value, index) => {
			return left_value === right[index];
		})
	);
}

function is_next_navigation_key(
	key: string,
	orientation: ListboxOrientation,
): boolean {
	return (
		key === "ArrowDown" ||
		(orientation === "horizontal" && key === "ArrowRight")
	);
}

function is_previous_navigation_key(
	key: string,
	orientation: ListboxOrientation,
): boolean {
	return (
		key === "ArrowUp" ||
		(orientation === "horizontal" && key === "ArrowLeft")
	);
}

export function createListbox<
	TMode extends string,
	TRecipe extends ListboxRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ListboxStyleSystem<TMode, TRecipe, TMetadata>,
	options: ListboxOptions = {},
): ListboxComponents<
	ListboxRecipeLayout<TRecipe>,
	ListboxRecipeVariant<TRecipe>,
	ListboxRecipeSize<TRecipe>
> {
	type TLayout = ListboxRecipeLayout<TRecipe>;
	type TVariant = ListboxRecipeVariant<TRecipe>;
	type TSize = ListboxRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	const listbox_recipe = createRecipe(style_system.token.recipe.listbox);

	function create_resolve_slot<TProps extends object>(
		slot: ListboxRecipeSlot,
		selection: (props: Partial<TProps>) => ListboxRecipeSelection<TRecipe>,
	): (
		props: Partial<TProps>,
	) => ReturnType<typeof listbox_recipe.resolve>["slots"][typeof slot] {
		return (props) => {
			return listbox_recipe.resolve(selection(props)).slots[slot];
		};
	}

	function Root(
		handle: Handle<
			ListboxRootProps<TLayout, TBreakpoint>,
			ListboxRuntimeContext
		>,
	): (props: ListboxRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_value = handle.props.defaultValue ?? null;
		let local_values = [...(handle.props.defaultValues ?? [])];
		let local_highlighted_value =
			handle.props.defaultHighlightedValue ?? null;
		const option_collection =
			create_ordered_collection<RegisteredListboxOption>();
		const option_typeahead = create_typeahead<RegisteredListboxOption>({
			timeoutMs: listbox_typeahead_reset_ms,
		});
		const value_state = create_controllable_state<
			string | null,
			ListboxValueChangeDetails
		>({
			getControlled: () => {
				if (handle.props.value === undefined) {
					return undefined;
				}
				return coerce_listbox_value(handle.props.value);
			},
			getLocal: () => {
				return local_value;
			},
			getOnChange: () => {
				return handle.props.onValueChange;
			},
			setLocal: (value) => {
				local_value = value;
			},
		});
		const values_state = create_controllable_state<
			readonly string[],
			ListboxValueChangeDetails
		>({
			equals: is_same_values,
			getControlled: () => {
				if (handle.props.values === undefined) {
					return undefined;
				}
				return coerce_listbox_values(handle.props.values);
			},
			getLocal: () => {
				return local_values;
			},
			getOnChange: () => {
				return handle.props.onValuesChange;
			},
			setLocal: (values) => {
				local_values = [...values];
			},
		});
		const highlighted_state = create_controllable_state<
			string | null,
			ListboxHighlightChangeDetails
		>({
			getControlled: () => {
				if (handle.props.highlightedValue === undefined) {
					return undefined;
				}
				return coerce_listbox_value(handle.props.highlightedValue);
			},
			getLocal: () => {
				return local_highlighted_value;
			},
			getOnChange: () => {
				return handle.props.onHighlightChange;
			},
			setLocal: (value) => {
				local_highlighted_value = value;
			},
		});

		function get_value(): string | null {
			return value_state.get();
		}

		function get_values(): readonly string[] {
			return values_state.get();
		}

		function get_selection_mode(): ListboxSelectionMode {
			return handle.props.selectionMode ?? single_selection_mode;
		}

		function get_navigation_fallback_value(): string | null {
			if (get_selection_mode() === multiple_selection_mode) {
				return get_values()[0] ?? null;
			}
			return get_value();
		}

		function get_highlighted_value(): string | null {
			return highlighted_state.get();
		}

		function get_enabled_options(): RegisteredListboxOption[] {
			return option_collection.getEnabledItems();
		}

		function find_option(
			value: string | null,
		): RegisteredListboxOption | undefined {
			if (value === null) {
				return undefined;
			}
			return option_collection.findByValue(value);
		}

		function scroll_highlighted_option_into_view(): void {
			const highlighted_option = find_option(get_highlighted_value());
			if (
				highlighted_option &&
				typeof highlighted_option.node.scrollIntoView === "function"
			) {
				highlighted_option.node.scrollIntoView({ block: "nearest" });
			}
		}

		function schedule_highlight_scroll(): void {
			handle.queueTask((signal) => {
				if (signal.aborted) {
					return;
				}
				scroll_highlighted_option_into_view();
			});
		}

		function value_selected(value: string): boolean {
			if (get_selection_mode() === multiple_selection_mode) {
				return get_values().includes(value);
			}
			return get_value() === value;
		}

		function select_value(
			value: string | null,
			details: ListboxValueChangeDetails,
		): void {
			if (handle.props.disabled) {
				return;
			}
			const option = find_option(value);
			if (option?.disabled) {
				return;
			}
			if (get_selection_mode() === multiple_selection_mode) {
				if (value === null) {
					return;
				}
				values_state.set(toggle_value(get_values(), value), details);
				if (handle.props.highlightedValue === undefined) {
					local_highlighted_value = value;
				}
				void handle.update();
				return;
			}
			value_state.set(value, details);
			if (handle.props.highlightedValue === undefined) {
				local_highlighted_value = value;
			}
			void handle.update();
		}

		function set_highlighted_value(
			value: string | null,
			details: ListboxHighlightChangeDetails,
		): void {
			const changed = highlighted_state.set(value, details);
			if (!changed) {
				return;
			}

			schedule_highlight_scroll();
			if (
				get_selection_mode() === single_selection_mode &&
				handle.props.selectionFollowsFocus &&
				value !== null
			) {
				value_state.set(value, {
					event: details.event,
					reason:
						details.reason === listboxHighlightChangeReason.pointer
							? listboxValueChangeReason.pointer
							: listboxValueChangeReason.keyboard,
				});
			}
			void handle.update();
		}

		function highlight_option(
			option: RegisteredListboxOption | undefined,
			details: ListboxHighlightChangeDetails,
		): void {
			set_highlighted_value(option?.value ?? null, details);
		}

		function move_highlight(
			offset: number,
			details: ListboxHighlightChangeDetails,
			clamp = false,
		): void {
			const enabled_options = get_enabled_options();
			if (enabled_options.length === 0) {
				highlight_option(undefined, details);
				return;
			}

			const next_option = get_collection_navigation_item({
				clamp,
				currentValue: get_highlighted_value(),
				fallbackValue: get_navigation_fallback_value(),
				items: enabled_options,
				loop: handle.props.loopFocus,
				offset,
			});
			if (next_option === undefined) {
				return;
			}
			highlight_option(next_option, details);
		}

		function ensure_highlight(
			details: ListboxHighlightChangeDetails,
		): void {
			if (get_highlighted_value() !== null) {
				return;
			}
			highlight_option(
				find_option(get_navigation_fallback_value()) ??
					get_first_collection_item(get_enabled_options()),
				details,
			);
		}

		function search(text: string, event: KeyboardEvent): void {
			if (handle.props.typeahead === false) {
				return;
			}

			const match = option_typeahead.search({
				currentValue: get_highlighted_value(),
				items: get_enabled_options(),
				key: text,
			});

			highlight_option(match, {
				event,
				reason: listboxHighlightChangeReason.typeahead,
			});
		}

		function handle_keyboard(event: KeyboardEvent): void {
			if (handle.props.disabled) {
				return;
			}

			const keyboard_highlight_details: ListboxHighlightChangeDetails = {
				event,
				reason: listboxHighlightChangeReason.keyboard,
			};
			const orientation =
				handle.props.orientation ?? vertical_orientation;

			if (is_next_navigation_key(event.key, orientation)) {
				event.preventDefault();
				move_highlight(1, keyboard_highlight_details);
				return;
			}

			if (is_previous_navigation_key(event.key, orientation)) {
				event.preventDefault();
				move_highlight(-1, keyboard_highlight_details);
				return;
			}

			if (event.key === "Home") {
				event.preventDefault();
				highlight_option(
					get_first_collection_item(get_enabled_options()),
					keyboard_highlight_details,
				);
				return;
			}

			if (event.key === "End") {
				event.preventDefault();
				highlight_option(
					get_last_collection_item(get_enabled_options()),
					keyboard_highlight_details,
				);
				return;
			}

			if (event.key === "PageDown") {
				event.preventDefault();
				move_highlight(
					listbox_page_jump_size,
					keyboard_highlight_details,
					true,
				);
				return;
			}

			if (event.key === "PageUp") {
				event.preventDefault();
				move_highlight(
					-listbox_page_jump_size,
					keyboard_highlight_details,
					true,
				);
				return;
			}

			if (is_listbox_keyboard_selection(event)) {
				event.preventDefault();
				select_value(get_highlighted_value(), {
					event,
					reason: listboxValueChangeReason.keyboard,
				});
				return;
			}

			if (is_printable_key_event(event)) {
				event.preventDefault();
				search(event.key, event);
			}
		}

		const context: ListboxRuntimeContext = {
			get_disabled: () => {
				return handle.props.disabled === true;
			},
			get_highlighted_id: () => {
				return find_option(get_highlighted_value())?.id;
			},
			get_highlighted_value,
			get_invalid: () => {
				return handle.props.invalid === true;
			},
			get_required: () => {
				return handle.props.required === true;
			},
			get_value,
			handle_focus: (event) => {
				if (handle.props.disabled) {
					return;
				}
				ensure_highlight({
					event,
					reason: listboxHighlightChangeReason.focus,
				});
			},
			handle_keydown: handle_keyboard,
			highlight_value: (value, details) => {
				if (handle.props.disabled) {
					return;
				}
				highlight_option(find_option(value), details);
			},
			register_option: (option) => {
				const changed = option_collection.register(option);
				if (changed) {
					void handle.update();
				}
			},
			select_value,
			value_selected,
			unregister_option: (id) => {
				const option = option_collection.unregister(id);
				void handle.update();
				if (option?.value === get_highlighted_value()) {
					set_highlighted_value(null, {
						reason: listboxHighlightChangeReason.focus,
					});
				}
			},
		};
		handle.context.set(context);

		const resolve_slot = create_resolve_slot<
			ListboxRootStyleProps<TLayout>
		>(listbox_slot.root, (props) => {
			return {
				layout: props.layout,
			} satisfies ListboxRecipeSelection<TRecipe>;
		});

		return (props: ListboxRootProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultHighlightedValue: _default_highlighted_value,
				defaultValue: _default_value,
				defaultValues: _default_values,
				disabled,
				highlightedValue: _highlighted_value,
				invalid,
				layout,
				loopFocus: _loop_focus,
				mix,
				onHighlightChange: _on_highlight_change,
				onValueChange: _on_value_change,
				onValuesChange: _on_values_change,
				orientation = vertical_orientation,
				required,
				selectionFollowsFocus: _selection_follows_focus,
				selectionMode = single_selection_mode,
				typeahead: _typeahead,
				value: _value,
				values: _values,
				...root_props
			} = props;
			const highlighted_id = context.get_highlighted_id();
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[listbox_slot.root]: {
						host: listbox_slot.root,
						conditions: mergeRecipeConditionSelectors(
							commonConditions,
							options.conditions?.[listbox_slot.root],
						),
						resolveSlot: resolve_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						listbox_scope,
						listbox_slot.root,
					),
					mix: [
						on<HTMLElement, typeof focus_event>(
							focus_event,
							(event) => {
								context.handle_focus(event);
							},
						),
						on<HTMLElement, typeof keydown_event>(
							keydown_event,
							(event) => {
								context.handle_keydown(event);
							},
						),
						parts.hosts[listbox_slot.root].mix,
					],
					props: {
						...root_props,
						"aria-activedescendant": highlighted_id,
						"aria-disabled": disabled ? "true" : undefined,
						"aria-invalid": invalid ? "true" : undefined,
						"aria-multiselectable":
							selectionMode === multiple_selection_mode
								? "true"
								: undefined,
						"aria-orientation": orientation,
						"aria-required": required ? "true" : undefined,
						"data-disabled": disabled ? "" : undefined,
						"data-invalid": invalid ? "" : undefined,
						"data-orientation": orientation,
						"data-required": required ? "" : undefined,
						mix,
						role: root_props.role ?? "listbox",
						tabIndex: disabled
							? undefined
							: (root_props.tabIndex ?? 0),
					},
				}),
				children,
			);
		};
	}

	function Option(
		handle: Handle<ListboxOptionProps<TVariant, TSize, TBreakpoint>>,
	): (props: ListboxOptionProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		let option_node: HTMLElement | null = null;
		let option_id: string | null = null;
		const resolve_slot = create_resolve_slot<
			ListboxOptionStyleProps<TVariant, TSize>
		>(listbox_slot.option, (props) => {
			return {
				size: props.size,
				variant: props.variant,
			} satisfies ListboxRecipeSelection<TRecipe>;
		});

		return (
			props: ListboxOptionProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				disabled = false,
				mix,
				size,
				textValue,
				value,
				variant,
				...option_props
			} = props;
			const selected = context.value_selected(value);
			const highlighted = context.get_highlighted_value() === value;
			const option_text = textValue ?? infer_text_value(children);
			const current_option_id = `${handle.id}-option`;
			option_id = current_option_id;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[listbox_slot.option]: {
						host: listbox_slot.option,
						conditions: mergeRecipeConditionSelectors(
							option_conditions,
							options.conditions?.[listbox_slot.option],
						),
						resolveSlot: resolve_slot,
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			function register_option(node: HTMLElement): void {
				context.register_option({
					disabled,
					id: current_option_id,
					node,
					text: option_text || node.textContent || "",
					value,
				});
			}

			if (option_node) {
				handle.queueTask((signal) => {
					if (signal.aborted || !option_node) {
						return;
					}
					register_option(option_node);
				});
			}

			const option_ref_mix = ref<HTMLElement>((node, signal) => {
				option_node = node;
				register_option(node);
				signal.addEventListener("abort", () => {
					if (option_id) {
						context.unregister_option(option_id);
					}
					option_node = null;
				});
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						listbox_scope,
						listbox_slot.option,
					),
					mix: [
						option_ref_mix,
						on<HTMLElement, typeof click_event>(
							click_event,
							(event) => {
								if (disabled) {
									return;
								}
								context.select_value(value, {
									event,
									reason: listboxValueChangeReason.pointer,
								});
							},
						),
						on<HTMLElement, typeof pointermove_event>(
							pointermove_event,
							(event) => {
								if (disabled) {
									return;
								}
								context.highlight_value(value, {
									event,
									reason: listboxHighlightChangeReason.pointer,
								});
							},
						),
						parts.hosts[listbox_slot.option].mix,
					],
					props: {
						...option_props,
						"aria-disabled": disabled ? "true" : undefined,
						"aria-selected": selected ? "true" : "false",
						"data-disabled": disabled ? "" : undefined,
						"data-highlighted": highlighted ? "" : undefined,
						"data-selected": selected ? "true" : undefined,
						id: current_option_id,
						mix,
						role: "option",
					},
				}),
				children ?? option_text,
			);
		};
	}

	function create_static_span_component<TProps extends { mix?: unknown }>(
		slot: ListboxRecipeSlot,
	): RemixComponent<TProps> {
		return () => {
			return (props: TProps): RemixNode => {
				const { children, mix, ...rest_props } = props as TProps & {
					children?: RemixNode;
					mix?: unknown;
				};
				const parts = createComponentStyleTargets({
					targets: {
						[slot]: {
							host: slot,
							resolveSlot: () => {
								return listbox_recipe.resolve().slots[slot];
							},
						},
					},
					props: {},
					styleSystem: style_system,
				});

				return createElement(
					"span",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(listbox_scope, slot),
						mix: parts.hosts[slot].mix,
						props: {
							...rest_props,
							mix,
						},
					}),
					children,
				);
			};
		};
	}

	function Group(
		handle: Handle<ListboxGroupProps, ListboxGroupRuntimeContext>,
	): (props: ListboxGroupProps) => RemixNode {
		const group_context = create_group_label_relationship({
			label_id: `${handle.id}-label`,
			on_change: () => {
				void handle.update();
			},
		});
		handle.context.set(group_context);

		return (props: ListboxGroupProps): RemixNode => {
			const { children, mix, ...group_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[listbox_slot.group]: {
						host: listbox_slot.group,
						resolveSlot: () => {
							return listbox_recipe.resolve().slots.group;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						listbox_scope,
						listbox_slot.group,
					),
					mix: parts.hosts[listbox_slot.group].mix,
					props: {
						...group_props,
						"aria-labelledby": group_context.get_labelled_by(),
						mix,
						role: "group",
					},
				}),
				children,
			);
		};
	}

	function GroupLabel(
		handle: Handle<ListboxGroupLabelProps>,
	): (props: ListboxGroupLabelProps) => RemixNode {
		const group_context = handle.context.get(Group);

		return (props: ListboxGroupLabelProps): RemixNode => {
			const { children, mix, ...group_label_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[listbox_slot.groupLabel]: {
						host: listbox_slot.groupLabel,
						resolveSlot: () => {
							return listbox_recipe.resolve().slots.groupLabel;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			const group_label_ref_mix =
				create_group_label_ref_mix(group_context);

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						listbox_scope,
						listbox_slot.groupLabel,
					),
					mix: [
						group_label_ref_mix,
						parts.hosts[listbox_slot.groupLabel].mix,
					],
					props: {
						...group_label_props,
						id: group_context.get_label_id(),
						mix,
					},
				}),
				children,
			);
		};
	}

	return {
		Group,
		GroupLabel,
		Option,
		OptionIndicator:
			create_static_span_component<ListboxOptionIndicatorProps>(
				listbox_slot.optionIndicator,
			),
		OptionText: create_static_span_component<ListboxOptionTextProps>(
			listbox_slot.optionText,
		),
		Root,
	};
}

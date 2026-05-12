import {
	Fragment,
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
	checkableStateFromValue,
	type CheckableStateName,
} from "./checkable-state.ts";
import {
	ariaBoolean,
	ariaTrue,
	componentDataAttribute,
	componentStateAttribute,
	dataFlag,
	openStateFromBoolean,
} from "./component-state.ts";
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
import { create_controllable_state } from "./controllable-state.ts";
import { createNativeSelectFormMirror } from "./form-mirror.ts";
import {
	create_group_label_ref_mix,
	create_group_label_relationship,
	type GroupLabelRelationship,
} from "./group-label.ts";
import {
	create_ordered_collection,
	type OrderedCollectionItem,
} from "./ordered-collection.ts";
import { popoverScrollLockGuard } from "./popover-scroll-lock.ts";
import {
	createPopupRefMix,
	createPopupRelationship,
	type PopupRelationship,
} from "./popup-behavior.ts";
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

const select_slot = {
	group: "group",
	groupLabel: "groupLabel",
	icon: "icon",
	list: "list",
	option: "option",
	optionIndicator: "optionIndicator",
	optionText: "optionText",
	popup: "popup",
	separator: "separator",
	trigger: "trigger",
	value: "value",
} as const;

export type SelectRecipeSlot = (typeof select_slot)[keyof typeof select_slot];

export type SelectRecipeCondition =
	| "active"
	| "disabled"
	| "focusVisible"
	| "highlighted"
	| "hover"
	| "open"
	| "placeholder"
	| "reducedMotion"
	| "selected";

export type SelectRecipeInput<
	TVariant extends string = string,
	TSize extends string = string,
	TPopupLayout extends string = string,
> = RecipeWithVariantGroups<
	SelectRecipeSlot,
	string,
	ComponentStyle,
	{
		popupLayout: TPopupLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type SelectRecipeVariant<TRecipe extends SelectRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;
export type SelectRecipeSize<TRecipe extends SelectRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;
export type SelectRecipePopupLayout<TRecipe extends SelectRecipeInput> =
	RecipeVariantValue<TRecipe, "popupLayout">;
export type SelectRecipeSelection<TRecipe extends SelectRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "popupLayout" | "size" | "variant">;

export type SelectStyleSystem<
	TMode extends string = string,
	TRecipe extends SelectRecipeInput = SelectRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { select: TRecipe } }, TMetadata>;

export const selectOpenChangeReason = {
	escape: "escape",
	interactOutside: "interactOutside",
	option: "option",
	tab: "tab",
	trigger: "trigger",
} as const;

export const selectValueChangeReason = {
	keyboard: "keyboard",
	pointer: "pointer",
} as const;

export const selectHighlightChangeReason = {
	keyboard: "keyboard",
	open: "open",
	pointer: "pointer",
	typeahead: "typeahead",
} as const;

export type SelectOpenChangeReason =
	(typeof selectOpenChangeReason)[keyof typeof selectOpenChangeReason];

export type SelectValueChangeReason =
	(typeof selectValueChangeReason)[keyof typeof selectValueChangeReason];

export type SelectHighlightChangeReason =
	(typeof selectHighlightChangeReason)[keyof typeof selectHighlightChangeReason];

export type SelectOpenChangeDetails = {
	event?: Event;
	reason: SelectOpenChangeReason;
};

export type SelectValueChangeDetails = {
	event?: Event;
	reason: SelectValueChangeReason;
};

export type SelectHighlightChangeDetails = {
	event?: Event;
	reason: SelectHighlightChangeReason;
};

export type SelectValueChangeHandler<TValue extends string = string> = (
	value: TValue | null,
	details?: SelectValueChangeDetails,
) => void;

export type SelectOpenChangeHandler = (
	open: boolean,
	details?: SelectOpenChangeDetails,
) => void;

export type SelectHighlightChangeHandler<TValue extends string = string> = (
	value: TValue | null,
	details?: SelectHighlightChangeDetails,
) => void;

export type SelectProps<TValue extends string = string> = {
	autoComplete?: string;
	children?: RemixNode;
	defaultOpen?: boolean;
	defaultHighlightedValue?: TValue | null;
	defaultValue?: TValue | null;
	disabled?: boolean;
	form?: string;
	highlightedValue?: TValue | null;
	invalid?: boolean;
	loopFocus?: boolean;
	name?: string;
	onHighlightChange?: SelectHighlightChangeHandler<TValue>;
	onOpenChange?: SelectOpenChangeHandler;
	onOpenChangeComplete?: SelectOpenChangeHandler;
	onValueChange?: SelectValueChangeHandler<TValue>;
	open?: boolean;
	placeholder?: string;
	readOnly?: boolean;
	required?: boolean;
	typeahead?: boolean;
	value?: TValue | null;
};

export type SelectTriggerStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type SelectTriggerProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	SelectTriggerStyleProps<TVariant, TSize> &
	ResponsiveProps<SelectTriggerStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
	};

export type SelectPopupStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type SelectPopupProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	SelectPopupStyleProps<TLayout> &
	ResponsiveProps<SelectPopupStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type SelectListProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = SelectPopupProps<TLayout, TBreakpoint>;

export type SelectValueProps = Omit<Props<"span">, "style"> & {
	placeholder?: RemixNode;
	style?: never;
};

export type SelectIconProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type SelectGroupProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type SelectGroupLabelProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type SelectSeparatorProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type SelectOptionProps<TValue extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	disabled?: boolean;
	style?: never;
	textValue?: string;
	value: TValue;
};

export type SelectOptionTextProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type SelectOptionIndicatorProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type SelectComponents<
	TVariant extends string = string,
	TSize extends string = string,
	TPopupLayout extends string = string,
> = {
	Group: RemixComponent<SelectGroupProps, SelectGroupRuntimeContext>;
	GroupLabel: RemixComponent<SelectGroupLabelProps>;
	Icon: RemixComponent<SelectIconProps>;
	List: RemixComponent<SelectListProps<TPopupLayout>>;
	Option: RemixComponent<SelectOptionProps>;
	OptionIndicator: RemixComponent<SelectOptionIndicatorProps>;
	OptionText: RemixComponent<SelectOptionTextProps>;
	Popup: RemixComponent<SelectPopupProps<TPopupLayout>>;
	Root: RemixComponent<SelectProps, SelectRuntimeContext>;
	Separator: RemixComponent<SelectSeparatorProps>;
	Trigger: RemixComponent<SelectTriggerProps<TVariant, TSize>>;
	Value: RemixComponent<SelectValueProps>;
};

export type SelectOptions = {
	conditions?: Partial<
		Readonly<Record<SelectRecipeSlot, RecipeConditionSelectorMap<string>>>
	>;
};

type SelectRuntimeContext = {
	close: (details: SelectOpenChangeDetails) => void;
	get_disabled: () => boolean;
	get_highlighted_id: () => string | undefined;
	get_highlighted_value: () => string | null;
	get_invalid: () => boolean;
	get_list_id: () => string;
	get_open: () => boolean;
	get_placeholder: () => string | undefined;
	get_popup_relationship: () => PopupRelationship;
	get_popup_id: () => string;
	get_read_only: () => boolean;
	get_required: () => boolean;
	get_selected_text: () => string | undefined;
	get_trigger_id: () => string;
	get_value: () => string | null;
	handle_keydown: (event: KeyboardEvent) => void;
	highlight_by_offset: (
		offset: number,
		details: SelectHighlightChangeDetails,
	) => void;
	highlight_first: () => void;
	highlight_last: () => void;
	highlight_value: (
		value: string | null,
		details: SelectHighlightChangeDetails,
	) => void;
	open: (
		details: SelectOpenChangeDetails,
		highlight?: SelectOpenHighlight,
	) => void;
	register_option: (option: RegisteredSelectOption) => void;
	register_trigger: (node: HTMLElement | null) => void;
	search: (text: string, event: KeyboardEvent) => void;
	select_value: (
		value: string | null,
		details: SelectValueChangeDetails,
	) => void;
	sync_popup: () => void;
	toggle: (details: SelectOpenChangeDetails) => void;
	unregister_option: (id: string) => void;
};

type RegisteredSelectOption = OrderedCollectionItem<string>;

type SelectOpenHighlight = "first" | "last" | "selected";

type SelectGroupRuntimeContext = GroupLabelRelationship;

const trigger_conditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	open: "&[aria-expanded='true']",
	placeholder: `&[${componentDataAttribute.placeholder}]`,
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const option_conditions = {
	disabled: `&[aria-disabled='true'], &[${componentDataAttribute.disabled}]`,
	highlighted: `&[${componentDataAttribute.highlighted}]`,
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
	selected: `&[aria-selected='true'], &[${componentDataAttribute.selected}]`,
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const open_conditions = {
	open: `&:popover-open, &[${componentDataAttribute.open}]`,
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const value_conditions = {
	placeholder: `&[${componentDataAttribute.placeholder}]`,
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const select_scope = "select";
const select_option_value_attribute = "data-vorma-select-value";
const select_page_jump_size = 10;
const select_typeahead_reset_ms = 700;

function infer_text_value(children: RemixNode): string {
	if (typeof children === "string" || typeof children === "number") {
		return String(children);
	}
	return "";
}

function is_select_keyboard_selection(event: KeyboardEvent): boolean {
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

function coerce_select_value(value: string | null | undefined): string | null {
	return value ?? null;
}

function select_option_state(selected: boolean): CheckableStateName {
	return checkableStateFromValue(selected, false);
}

export function createSelect<
	TMode extends string,
	TRecipe extends SelectRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SelectStyleSystem<TMode, TRecipe, TMetadata>,
	options: SelectOptions = {},
): SelectComponents<
	SelectRecipeVariant<TRecipe>,
	SelectRecipeSize<TRecipe>,
	SelectRecipePopupLayout<TRecipe>
> {
	type TVariant = SelectRecipeVariant<TRecipe>;
	type TSize = SelectRecipeSize<TRecipe>;
	type TPopupLayout = SelectRecipePopupLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	const select_recipe = createRecipe(style_system.token.recipe.select);

	function Root(
		handle: Handle<SelectProps, SelectRuntimeContext>,
	): (props: SelectProps) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;
		let local_value = handle.props.defaultValue ?? null;
		let local_highlighted_value =
			handle.props.defaultHighlightedValue ?? null;
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: SelectOpenChangeDetails = {
			reason: selectOpenChangeReason.trigger,
		};
		const popup_relationship = createPopupRelationship();
		const option_collection =
			create_ordered_collection<RegisteredSelectOption>();
		const option_typeahead = create_typeahead<RegisteredSelectOption>({
			timeoutMs: select_typeahead_reset_ms,
		});
		const open_state = create_controllable_state<
			boolean,
			SelectOpenChangeDetails
		>({
			getControlled: () => {
				return handle.props.open;
			},
			getLocal: () => {
				return local_open;
			},
			getOnChange: () => {
				return handle.props.onOpenChange;
			},
			setLocal: (value) => {
				local_open = value;
			},
		});
		const value_state = create_controllable_state<
			string | null,
			SelectValueChangeDetails
		>({
			getControlled: () => {
				if (handle.props.value === undefined) {
					return undefined;
				}
				return coerce_select_value(handle.props.value);
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
		const highlighted_state = create_controllable_state<
			string | null,
			SelectHighlightChangeDetails
		>({
			getControlled: () => {
				if (handle.props.highlightedValue === undefined) {
					return undefined;
				}
				return coerce_select_value(handle.props.highlightedValue);
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

		function get_open(): boolean {
			return open_state.get();
		}

		function get_value(): string | null {
			return value_state.get();
		}

		function get_highlighted_value(): string | null {
			return highlighted_state.get();
		}

		function get_options(): RegisteredSelectOption[] {
			return option_collection.getItems();
		}

		function get_enabled_options(): RegisteredSelectOption[] {
			return option_collection.getEnabledItems();
		}

		function find_option(
			value: string | null,
		): RegisteredSelectOption | undefined {
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

		function request_open(
			open: boolean,
			details: SelectOpenChangeDetails,
		): void {
			if (open && handle.props.disabled) {
				return;
			}
			const changed = open_state.set(open, details);
			if (!changed) {
				return;
			}

			last_open_details = details;
			void handle.update();
		}

		function set_highlighted_value(
			value: string | null,
			details: SelectHighlightChangeDetails,
		): void {
			const changed = highlighted_state.set(value, details);
			if (!changed) {
				return;
			}

			schedule_highlight_scroll();
			void handle.update();
		}

		function highlight_option(
			option: RegisteredSelectOption | undefined,
			details: SelectHighlightChangeDetails,
		): void {
			set_highlighted_value(option?.value ?? null, details);
		}

		function move_highlight(
			offset: number,
			details: SelectHighlightChangeDetails,
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
				fallbackValue: get_value(),
				items: enabled_options,
				loop: handle.props.loopFocus,
				offset,
			});
			if (next_option === undefined) {
				return;
			}
			highlight_option(next_option, details);
		}

		function select_value(
			value: string | null,
			details: SelectValueChangeDetails,
		): void {
			if (handle.props.disabled || handle.props.readOnly) {
				return;
			}

			const option = find_option(value);
			if (option?.disabled) {
				return;
			}
			value_state.set(value, details);
			if (handle.props.highlightedValue === undefined) {
				local_highlighted_value = value;
			}
			request_open(false, {
				event: details.event,
				reason:
					details.event instanceof KeyboardEvent &&
					details.event.key === "Tab"
						? selectOpenChangeReason.tab
						: selectOpenChangeReason.option,
			});
			void handle.update();
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
				reason: selectHighlightChangeReason.typeahead,
			});
		}

		function apply_open_highlight(
			highlight: SelectOpenHighlight | undefined,
			details: SelectOpenChangeDetails,
		): void {
			if (highlight === "first") {
				highlight_option(
					get_first_collection_item(get_enabled_options()),
					{
						event: details.event,
						reason: selectHighlightChangeReason.open,
					},
				);
				return;
			}
			if (highlight === "last") {
				highlight_option(
					get_last_collection_item(get_enabled_options()),
					{
						event: details.event,
						reason: selectHighlightChangeReason.open,
					},
				);
				return;
			}
			if (highlight === "selected" && get_highlighted_value() === null) {
				highlight_option(find_option(get_value()), {
					event: details.event,
					reason: selectHighlightChangeReason.open,
				});
			}
		}

		function create_hidden_select(props: SelectProps): RemixNode {
			return createNativeSelectFormMirror({
				autoComplete: props.autoComplete,
				disabled: props.disabled,
				form: props.form,
				name: props.name,
				options: get_options().map((option) => {
					return {
						disabled: option.disabled,
						text: option.text,
						value: option.value,
					};
				}),
				placeholder: props.placeholder,
				required: props.required,
				value: get_value(),
			});
		}

		function open_from_keyboard(
			event: KeyboardEvent,
			highlight?: SelectOpenHighlight,
		): void {
			request_open(true, {
				event,
				reason: selectOpenChangeReason.trigger,
			});
			apply_open_highlight(highlight, {
				event,
				reason: selectOpenChangeReason.trigger,
			});
		}

		function handle_keyboard(event: KeyboardEvent): void {
			if (handle.props.disabled) {
				return;
			}

			const keyboard_highlight_details: SelectHighlightChangeDetails = {
				event,
				reason: selectHighlightChangeReason.keyboard,
			};

			if (event.altKey && event.key === "ArrowDown") {
				event.preventDefault();
				open_from_keyboard(event, "selected");
				return;
			}

			if (event.altKey && event.key === "ArrowUp" && get_open()) {
				event.preventDefault();
				select_value(get_highlighted_value(), {
					event,
					reason: selectValueChangeReason.keyboard,
				});
				return;
			}

			if (event.key === "ArrowDown") {
				event.preventDefault();
				if (get_open()) {
					move_highlight(1, keyboard_highlight_details);
					return;
				}
				open_from_keyboard(event, "selected");
				return;
			}

			if (event.key === "ArrowUp") {
				event.preventDefault();
				if (get_open()) {
					move_highlight(-1, keyboard_highlight_details);
					return;
				}
				open_from_keyboard(event, "last");
				return;
			}

			if (event.key === "Home") {
				event.preventDefault();
				if (!get_open()) {
					open_from_keyboard(event, "first");
					return;
				}
				highlight_option(
					get_first_collection_item(get_enabled_options()),
					keyboard_highlight_details,
				);
				return;
			}

			if (event.key === "End") {
				event.preventDefault();
				const enabled_options = get_enabled_options();
				if (!get_open()) {
					open_from_keyboard(event, "last");
					return;
				}
				highlight_option(
					get_last_collection_item(enabled_options),
					keyboard_highlight_details,
				);
				return;
			}

			if (event.key === "PageDown" && get_open()) {
				event.preventDefault();
				move_highlight(
					select_page_jump_size,
					keyboard_highlight_details,
					true,
				);
				return;
			}

			if (event.key === "PageUp" && get_open()) {
				event.preventDefault();
				move_highlight(
					-select_page_jump_size,
					keyboard_highlight_details,
					true,
				);
				return;
			}

			if (is_select_keyboard_selection(event)) {
				event.preventDefault();
				if (!get_open()) {
					open_from_keyboard(event, "selected");
					return;
				}
				select_value(get_highlighted_value(), {
					event,
					reason: selectValueChangeReason.keyboard,
				});
				return;
			}

			if (event.key === "Escape" && get_open()) {
				event.preventDefault();
				request_open(false, {
					event,
					reason: selectOpenChangeReason.escape,
				});
				return;
			}

			if (event.key === "Tab" && get_open()) {
				const highlighted_value = get_highlighted_value();
				if (highlighted_value === null) {
					request_open(false, {
						event,
						reason: selectOpenChangeReason.tab,
					});
					return;
				}
				select_value(highlighted_value, {
					event,
					reason: selectValueChangeReason.keyboard,
				});
				return;
			}

			if (is_printable_key_event(event)) {
				event.preventDefault();
				if (!get_open()) {
					request_open(true, {
						event,
						reason: selectOpenChangeReason.trigger,
					});
				}
				search(event.key, event);
			}
		}

		const context: SelectRuntimeContext = {
			close: (details) => {
				request_open(false, details);
			},
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
			get_list_id: () => {
				return `${handle.id}-list`;
			},
			get_open,
			get_placeholder: () => {
				return handle.props.placeholder;
			},
			get_popup_relationship: () => {
				return popup_relationship;
			},
			get_popup_id: () => {
				return `${handle.id}-popup`;
			},
			get_read_only: () => {
				return handle.props.readOnly === true;
			},
			get_required: () => {
				return handle.props.required === true;
			},
			get_selected_text: () => {
				return find_option(get_value())?.text;
			},
			get_trigger_id: () => {
				return `${handle.id}-trigger`;
			},
			get_value,
			handle_keydown: handle_keyboard,
			highlight_by_offset: (offset, details) => {
				move_highlight(offset, details);
			},
			highlight_first: () => {
				highlight_option(
					get_first_collection_item(get_enabled_options()),
					{
						reason: selectHighlightChangeReason.keyboard,
					},
				);
			},
			highlight_last: () => {
				highlight_option(
					get_last_collection_item(get_enabled_options()),
					{
						reason: selectHighlightChangeReason.keyboard,
					},
				);
			},
			highlight_value: (value, details) => {
				highlight_option(find_option(value), details);
			},
			open: (details, highlight = "selected") => {
				if (handle.props.disabled) {
					return;
				}
				apply_open_highlight(highlight, details);
				request_open(true, details);
			},
			register_option: (option) => {
				const changed = option_collection.register(option);
				if (changed) {
					void handle.update();
				}
			},
			register_trigger: (node) => {
				popup_relationship.registerTrigger(node);
			},
			search,
			select_value,
			sync_popup: () => {
				popup_relationship.syncPopup(get_open());
				schedule_highlight_scroll();
			},
			toggle: (details) => {
				if (get_open()) {
					request_open(false, details);
					return;
				}
				context.open(details);
			},
			unregister_option: (id) => {
				const option = option_collection.unregister(id);
				void handle.update();
				if (option?.value === get_highlighted_value()) {
					set_highlighted_value(null, {
						reason: selectHighlightChangeReason.open,
					});
				}
			},
		};
		handle.context.set(context);

		return (props: SelectProps): RemixNode => {
			const current_open = get_open();
			if (current_open !== last_completed_open) {
				const completed_open = current_open;
				const completed_details = last_open_details;
				last_completed_open = current_open;
				handle.queueTask((signal) => {
					if (signal.aborted) {
						return;
					}
					props.onOpenChangeComplete?.(
						completed_open,
						completed_details,
					);
				});
			}

			return createElement(
				Fragment,
				{},
				props.children,
				create_hidden_select(props),
			);
		};
	}

	function create_resolve_slot<TProps extends object>(
		slot: SelectRecipeSlot,
		selection: (props: Partial<TProps>) => SelectRecipeSelection<TRecipe>,
	): (
		props: Partial<TProps>,
	) => ReturnType<typeof select_recipe.resolve>["slots"][typeof slot] {
		return (props) => {
			return select_recipe.resolve(selection(props)).slots[slot];
		};
	}

	function Trigger(
		handle: Handle<SelectTriggerProps<TVariant, TSize, TBreakpoint>>,
	): (props: SelectTriggerProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		const resolve_slot = create_resolve_slot<
			SelectTriggerStyleProps<TVariant, TSize>
		>(select_slot.trigger, (props) => {
			return {
				size: props.size,
				variant: props.variant,
			} satisfies SelectRecipeSelection<TRecipe>;
		});

		return (
			props: SelectTriggerProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				mix,
				size,
				type = "button",
				variant,
				...trigger_props
			} = props;
			const is_open = context.get_open();
			const has_value = context.get_value() !== null;
			const highlighted_id = context.get_highlighted_id();
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					[select_slot.trigger]: "button",
				},
				targets: {
					[select_slot.trigger]: {
						host: select_slot.trigger,
						conditions: mergeRecipeConditionSelectors(
							trigger_conditions,
							options.conditions?.[select_slot.trigger],
						),
						resolveSlot: resolve_slot,
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});
			const trigger_click_mix = on<HTMLButtonElement, "click">(
				"click",
				(event) => {
					context.toggle({
						event,
						reason: selectOpenChangeReason.trigger,
					});
				},
			);
			const trigger_keydown_mix = on<HTMLButtonElement, "keydown">(
				"keydown",
				(event) => {
					context.handle_keydown(event);
				},
			);
			const trigger_ref_mix = ref<HTMLButtonElement>((node, signal) => {
				context.register_trigger(node);
				signal.addEventListener("abort", () => {
					context.register_trigger(null);
				});
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.trigger,
					),
					mix: [
						trigger_ref_mix,
						trigger_click_mix,
						trigger_keydown_mix,
						parts.hosts[select_slot.trigger].mix,
					],
					props: {
						...trigger_props,
						"aria-activedescendant": is_open
							? highlighted_id
							: undefined,
						"aria-controls": context.get_list_id(),
						"aria-disabled": ariaTrue(context.get_disabled()),
						"aria-expanded": ariaBoolean(is_open),
						"aria-haspopup": "listbox",
						"aria-invalid": ariaTrue(context.get_invalid()),
						"aria-readonly": ariaTrue(context.get_read_only()),
						"aria-required": ariaTrue(context.get_required()),
						[componentDataAttribute.disabled]: dataFlag(
							context.get_disabled(),
						),
						[componentDataAttribute.invalid]: dataFlag(
							context.get_invalid(),
						),
						[componentDataAttribute.open]: dataFlag(is_open),
						[componentDataAttribute.placeholder]:
							dataFlag(!has_value),
						[componentDataAttribute.readOnly]: dataFlag(
							context.get_read_only(),
						),
						[componentDataAttribute.required]: dataFlag(
							context.get_required(),
						),
						[componentStateAttribute]:
							openStateFromBoolean(is_open),
						disabled: context.get_disabled(),
						id: context.get_trigger_id(),
						mix,
						role: "combobox",
						type,
					},
				}),
				children,
			);
		};
	}

	function Popup(
		handle: Handle<SelectPopupProps<TPopupLayout, TBreakpoint>>,
	): (props: SelectPopupProps<TPopupLayout, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		const resolve_slot = create_resolve_slot<
			SelectPopupStyleProps<TPopupLayout>
		>(select_slot.popup, (props) => {
			return {
				popupLayout: props.layout,
			} satisfies SelectRecipeSelection<TRecipe>;
		});

		return (
			props: SelectPopupProps<TPopupLayout, TBreakpoint>,
		): RemixNode => {
			const { at, children, layout, mix, ...popup_props } = props;
			const is_open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[select_slot.popup]: {
						host: select_slot.popup,
						conditions: mergeRecipeConditionSelectors(
							open_conditions,
							options.conditions?.[select_slot.popup],
						),
						resolveSlot: resolve_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});
			const popup_ref_mix = createPopupRefMix({
				getOpen: context.get_open,
				onInteractOutside: (event) => {
					context.close({
						event,
						reason: selectOpenChangeReason.interactOutside,
					});
				},
				relationship: context.get_popup_relationship(),
			});

			handle.queueTask((signal) => {
				if (signal.aborted) {
					return;
				}
				context.sync_popup();
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.popup,
					),
					mix: [
						popoverScrollLockGuard(),
						popup_ref_mix,
						parts.hosts[select_slot.popup].mix,
					],
					props: {
						...popup_props,
						"aria-labelledby": context.get_trigger_id(),
						[componentDataAttribute.open]: dataFlag(is_open),
						hidden: !is_open,
						id: context.get_popup_id(),
						mix,
						popover: "manual",
					},
				}),
				children,
			);
		};
	}

	function List(
		handle: Handle<SelectListProps<TPopupLayout, TBreakpoint>>,
	): (props: SelectListProps<TPopupLayout, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		const resolve_slot = create_resolve_slot<
			SelectPopupStyleProps<TPopupLayout>
		>(select_slot.list, (props) => {
			return {
				popupLayout: props.layout,
			} satisfies SelectRecipeSelection<TRecipe>;
		});

		return (
			props: SelectPopupProps<TPopupLayout, TBreakpoint>,
		): RemixNode => {
			const { at, children, layout, mix, ...list_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[select_slot.list]: {
						host: select_slot.list,
						conditions: mergeRecipeConditionSelectors(
							open_conditions,
							options.conditions?.[select_slot.list],
						),
						resolveSlot: resolve_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});
			const list_keydown_mix = on<HTMLElement, "keydown">(
				"keydown",
				(event) => {
					context.handle_keydown(event);
				},
			);

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.list,
					),
					mix: [list_keydown_mix, parts.hosts[select_slot.list].mix],
					props: {
						...list_props,
						"aria-labelledby": context.get_trigger_id(),
						id: context.get_list_id(),
						mix,
						role: "listbox",
					},
				}),
				children,
			);
		};
	}

	function Value(
		handle: Handle<SelectValueProps>,
	): (props: SelectValueProps) => RemixNode {
		const context = handle.context.get(Root);

		return (props: SelectValueProps): RemixNode => {
			const { children, mix, placeholder, ...value_props } = props;
			const selected_text = context.get_selected_text();
			const placeholder_text = placeholder ?? context.get_placeholder();
			const is_placeholder = selected_text === undefined;
			const parts = createComponentStyleTargets({
				targets: {
					[select_slot.value]: {
						host: select_slot.value,
						conditions: mergeRecipeConditionSelectors(
							value_conditions,
							options.conditions?.[select_slot.value],
						),
						resolveSlot: () => {
							return select_recipe.resolve().slots.value;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"span",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.value,
					),
					mix: parts.hosts[select_slot.value].mix,
					props: {
						...value_props,
						[componentDataAttribute.placeholder]:
							dataFlag(is_placeholder),
						mix,
					},
				}),
				children ?? selected_text ?? placeholder_text,
			);
		};
	}

	function create_static_span_component<TProps extends { mix?: unknown }>(
		slot: SelectRecipeSlot,
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
								return select_recipe.resolve().slots[slot];
							},
						},
					},
					props: {},
					styleSystem: style_system,
				});

				return createElement(
					"span",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(select_scope, slot),
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

	function create_static_div_component<TProps extends { mix?: unknown }>(
		slot: SelectRecipeSlot,
		role?: string,
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
								return select_recipe.resolve().slots[slot];
							},
						},
					},
					props: {},
					styleSystem: style_system,
				});

				return createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(select_scope, slot),
						mix: parts.hosts[slot].mix,
						props: {
							...rest_props,
							mix,
							role,
						},
					}),
					children,
				);
			};
		};
	}

	function Group(
		handle: Handle<SelectGroupProps, SelectGroupRuntimeContext>,
	): (props: SelectGroupProps) => RemixNode {
		const group_context = create_group_label_relationship({
			label_id: `${handle.id}-label`,
			on_change: () => {
				void handle.update();
			},
		});
		handle.context.set(group_context);

		return (props: SelectGroupProps): RemixNode => {
			const { children, mix, ...group_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[select_slot.group]: {
						host: select_slot.group,
						resolveSlot: () => {
							return select_recipe.resolve().slots.group;
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
						select_scope,
						select_slot.group,
					),
					mix: parts.hosts[select_slot.group].mix,
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
		handle: Handle<SelectGroupLabelProps>,
	): (props: SelectGroupLabelProps) => RemixNode {
		const group_context = handle.context.get(Group);

		return (props: SelectGroupLabelProps): RemixNode => {
			const { children, mix, ...group_label_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[select_slot.groupLabel]: {
						host: select_slot.groupLabel,
						resolveSlot: () => {
							return select_recipe.resolve().slots.groupLabel;
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
						select_scope,
						select_slot.groupLabel,
					),
					mix: [
						group_label_ref_mix,
						parts.hosts[select_slot.groupLabel].mix,
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

	function Option(
		handle: Handle<SelectOptionProps>,
	): (props: SelectOptionProps) => RemixNode {
		const context = handle.context.get(Root);
		let option_node: HTMLElement | null = null;
		let option_id: string | null = null;

		return (props: SelectOptionProps): RemixNode => {
			const {
				children,
				disabled = false,
				mix,
				textValue,
				value,
				...option_props
			} = props;
			const selected = context.get_value() === value;
			const highlighted = context.get_highlighted_value() === value;
			const option_text = textValue ?? infer_text_value(children);
			const current_option_id = `${handle.id}-option`;
			option_id = current_option_id;
			const parts = createComponentStyleTargets({
				targets: {
					[select_slot.option]: {
						host: select_slot.option,
						conditions: mergeRecipeConditionSelectors(
							option_conditions,
							options.conditions?.[select_slot.option],
						),
						resolveSlot: () => {
							return select_recipe.resolve().slots.option;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			const option_click_mix = on<HTMLElement, "click">(
				"click",
				(event) => {
					if (disabled) {
						return;
					}
					context.select_value(value, {
						event,
						reason: selectValueChangeReason.pointer,
					});
				},
			);
			const option_pointer_down_mix = on<HTMLElement, "pointerdown">(
				"pointerdown",
				(event) => {
					if (disabled) {
						return;
					}
					event.preventDefault();
				},
			);
			const option_pointer_move_mix = on<HTMLElement, "pointermove">(
				"pointermove",
				(event) => {
					if (disabled) {
						return;
					}
					context.highlight_value(value, {
						event,
						reason: selectHighlightChangeReason.pointer,
					});
				},
			);
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
						select_scope,
						select_slot.option,
					),
					mix: [
						option_ref_mix,
						option_pointer_down_mix,
						option_click_mix,
						option_pointer_move_mix,
						parts.hosts[select_slot.option].mix,
					],
					props: {
						...option_props,
						"aria-disabled": ariaTrue(disabled),
						"aria-selected": ariaBoolean(selected),
						[componentDataAttribute.disabled]: dataFlag(disabled),
						[componentDataAttribute.highlighted]:
							dataFlag(highlighted),
						[componentDataAttribute.selected]: dataFlag(selected),
						[componentStateAttribute]:
							select_option_state(selected),
						[select_option_value_attribute]: value,
						id: current_option_id,
						mix,
						role: "option",
					},
				}),
				children ?? option_text,
			);
		};
	}

	return {
		Group,
		GroupLabel,
		Icon: create_static_span_component<SelectIconProps>(select_slot.icon),
		List,
		Option,
		OptionIndicator:
			create_static_span_component<SelectOptionIndicatorProps>(
				select_slot.optionIndicator,
			),
		OptionText: create_static_span_component<SelectOptionTextProps>(
			select_slot.optionText,
		),
		Popup,
		Root,
		Separator: create_static_div_component<SelectSeparatorProps>(
			select_slot.separator,
			"separator",
		),
		Trigger,
		Value,
	};
}

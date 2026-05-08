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
	checkableState,
	checkableStateFromValue,
	type CheckableChecked,
	type CheckableStateName,
} from "./checkable-state.ts";
import {
	ariaBoolean,
	ariaTrue,
	componentDataAttribute,
	componentStateAttribute,
	dataFlag,
	openState,
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
	get_roving_tab_index,
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

export const menuAnatomy = {
	group: "group",
	groupLabel: "groupLabel",
	item: "item",
	itemIndicator: "itemIndicator",
	popup: "popup",
	separator: "separator",
	trigger: "trigger",
} as const;

export type MenuRecipeSlot = (typeof menuAnatomy)[keyof typeof menuAnatomy];

export type MenuRecipeCondition =
	| CommonRecipeCondition
	| "checked"
	| "closed"
	| "highlighted"
	| "indeterminate"
	| "open"
	| "unchecked";

export type MenuRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	MenuRecipeSlot,
	string,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type MenuRecipeLayout<TRecipe extends MenuRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type MenuRecipeVariant<TRecipe extends MenuRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type MenuRecipeSize<TRecipe extends MenuRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type MenuRecipeSelection<TRecipe extends MenuRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout" | "size" | "variant">;

export type MenuStyleSystem<
	TMode extends string = string,
	TRecipe extends MenuRecipeInput = MenuRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { menu: TRecipe } }, TMetadata>;

export type MenuOpenChangeDetails = {
	event?: Event;
	reason: MenuOpenChangeReason;
};

export type MenuOpenChangeHandler = (
	open: boolean,
	details?: MenuOpenChangeDetails,
) => void;

export type MenuRootProps = {
	children?: RemixNode;
	closeOnSelect?: boolean;
	defaultOpen?: boolean;
	loopFocus?: boolean;
	onOpenChange?: MenuOpenChangeHandler;
	onOpenChangeComplete?: MenuOpenChangeHandler;
	open?: boolean;
	typeahead?: boolean;
};

export type MenuTriggerStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type MenuPopupStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type MenuItemStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = MenuTriggerStyleProps<TVariant, TSize>;

export type MenuTriggerProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	MenuTriggerStyleProps<TVariant, TSize> &
	ResponsiveProps<MenuTriggerStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
	};

export type MenuPopupProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	MenuPopupStyleProps<TLayout> &
	ResponsiveProps<MenuPopupStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type MenuItemProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	MenuItemStyleProps<TVariant, TSize> &
	ResponsiveProps<MenuItemStyleProps<TVariant, TSize>, TBreakpoint> & {
		disabled?: boolean;
		style?: never;
		textValue?: string;
	};

export type MenuChecked = CheckableChecked;

export type MenuCheckedChangeHandler = (
	checked: MenuChecked,
	details?: MenuItemSelectDetails,
) => void;

export type MenuRadioValueChangeHandler<TValue extends string = string> = (
	value: TValue,
	details?: MenuItemSelectDetails,
) => void;

export type MenuCheckboxItemProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	MenuItemStyleProps<TVariant, TSize> &
	ResponsiveProps<MenuItemStyleProps<TVariant, TSize>, TBreakpoint> & {
		checked?: MenuChecked;
		defaultChecked?: MenuChecked;
		disabled?: boolean;
		onCheckedChange?: MenuCheckedChangeHandler;
		style?: never;
		textValue?: string;
	};

export type MenuRadioGroupProps<TValue extends string = string> = {
	children?: RemixNode;
	defaultValue?: TValue | null;
	onValueChange?: MenuRadioValueChangeHandler<TValue>;
	value?: TValue | null;
};

export type MenuRadioItemProps<
	TValue extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style" | "value"> &
	MenuItemStyleProps<TVariant, TSize> &
	ResponsiveProps<MenuItemStyleProps<TVariant, TSize>, TBreakpoint> & {
		disabled?: boolean;
		style?: never;
		textValue?: string;
		value: TValue;
	};

export type MenuItemIndicatorProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type MenuSeparatorProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type MenuGroupProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type MenuGroupLabelProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type MenuComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	CheckboxItem: RemixComponent<
		MenuCheckboxItemProps<TVariant, TSize>,
		MenuItemRuntimeContext
	>;
	Group: RemixComponent<MenuGroupProps, MenuGroupContext>;
	GroupLabel: RemixComponent<MenuGroupLabelProps>;
	Item: RemixComponent<MenuItemProps<TVariant, TSize>>;
	ItemIndicator: RemixComponent<MenuItemIndicatorProps>;
	Popup: RemixComponent<MenuPopupProps<TLayout>>;
	RadioGroup: RemixComponent<MenuRadioGroupProps, MenuRadioGroupContext>;
	RadioItem: RemixComponent<
		MenuRadioItemProps<string, TVariant, TSize>,
		MenuItemRuntimeContext
	>;
	Root: RemixComponent<MenuRootProps, MenuContext>;
	Separator: RemixComponent<MenuSeparatorProps>;
	Trigger: RemixComponent<MenuTriggerProps<TVariant, TSize>>;
};

type MenuContext = {
	close: (details: MenuOpenChangeDetails) => void;
	get_close_on_select: () => boolean;
	get_current_item_value: () => string | null;
	get_open: () => boolean;
	get_popup_relationship: () => PopupRelationship;
	get_popup_id: () => string;
	get_trigger_id: () => string;
	handle_popup_keydown: (event: KeyboardEvent) => void;
	open: (details: MenuOpenChangeDetails, focus?: MenuOpenFocus) => void;
	register_item: (item: RegisteredMenuItem) => void;
	register_trigger: (node: HTMLButtonElement | null) => void;
	select_item: (value: string, details: MenuItemSelectDetails) => void;
	set_current_item: (
		value: string | null,
		details: MenuItemFocusDetails,
		focus?: boolean,
	) => void;
	toggle: (details: MenuOpenChangeDetails, focus?: MenuOpenFocus) => void;
	unregister_item: (id: string) => void;
	sync_popup: () => void;
};

type MenuGroupContext = GroupLabelRelationship;

type MenuRadioGroupContext = {
	get_value: () => string | null;
	set_value: (value: string, details: MenuItemSelectDetails) => void;
};

type MenuItemRuntimeContext = {
	get_state: () => CheckableStateName | undefined;
};

export const menuOpenChangeReason = {
	escape: "escape",
	interactOutside: "interactOutside",
	item: "item",
	tab: "tab",
	trigger: "trigger",
} as const;

export const menuItemFocusReason = {
	keyboard: "keyboard",
	open: "open",
	pointer: "pointer",
	typeahead: "typeahead",
} as const;

export const menuItemSelectReason = {
	keyboard: "keyboard",
	pointer: "pointer",
} as const;

export type MenuOpenChangeReason =
	(typeof menuOpenChangeReason)[keyof typeof menuOpenChangeReason];

export type MenuItemFocusReason =
	(typeof menuItemFocusReason)[keyof typeof menuItemFocusReason];

export type MenuItemSelectReason =
	(typeof menuItemSelectReason)[keyof typeof menuItemSelectReason];

export type MenuItemFocusDetails = {
	event?: Event;
	reason: MenuItemFocusReason;
};

export type MenuItemSelectDetails = {
	event?: Event;
	reason: MenuItemSelectReason;
};

type RegisteredMenuItem = OrderedCollectionItem<string> & {
	activate: (details: MenuItemSelectDetails) => void;
};

type MenuOpenFocus = "first" | "last";

const menu_scope = "menu";
const button_type_default = "button";
const click_event = "click";
const focus_event = "focus";
const keydown_event = "keydown";
const pointermove_event = "pointermove";
const menu_typeahead_reset_ms = 700;

const menu_conditions = mergeRecipeConditionSelectors<MenuRecipeCondition>(
	commonConditions,
	{
		checked: `&[${componentStateAttribute}='${checkableState.checked}']`,
		closed: `&[${componentStateAttribute}='${openState.closed}']`,
		highlighted: `&[${componentDataAttribute.highlighted}]`,
		indeterminate: `&[${componentStateAttribute}='${checkableState.indeterminate}']`,
		open: `&[${componentStateAttribute}='${openState.open}']`,
		unchecked: `&[${componentStateAttribute}='${checkableState.unchecked}']`,
	} satisfies RecipeConditionSelectorMap<MenuRecipeCondition>,
);

function infer_text_value(children: RemixNode): string {
	if (typeof children === "string" || typeof children === "number") {
		return String(children);
	}
	return "";
}

function is_menu_keyboard_selection(event: KeyboardEvent): boolean {
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

export function createMenu<
	TMode extends string,
	TRecipe extends MenuRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: MenuStyleSystem<TMode, TRecipe, TMetadata>,
): MenuComponents<
	MenuRecipeLayout<TRecipe>,
	MenuRecipeVariant<TRecipe>,
	MenuRecipeSize<TRecipe>
> {
	type TLayout = MenuRecipeLayout<TRecipe>;
	type TVariant = MenuRecipeVariant<TRecipe>;
	type TSize = MenuRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = MenuRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.menu);

	function resolve_slot(
		slot: MenuRecipeSlot,
		props: Partial<MenuPopupStyleProps<TLayout>> &
			Partial<MenuTriggerStyleProps<TVariant, TSize>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(
		handle: Handle<MenuRootProps, MenuContext>,
	): (props: MenuRootProps) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;
		let current_item_value: string | null = null;
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: MenuOpenChangeDetails = {
			reason: menuOpenChangeReason.trigger,
		};
		const popup_relationship = createPopupRelationship();
		const item_collection = create_ordered_collection<RegisteredMenuItem>();
		const item_typeahead = create_typeahead<RegisteredMenuItem>({
			timeoutMs: menu_typeahead_reset_ms,
		});
		const open_state = create_controllable_state<
			boolean,
			MenuOpenChangeDetails
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
			setLocal: (open) => {
				local_open = open;
			},
		});

		function get_open(): boolean {
			return open_state.get();
		}

		function request_open(
			open: boolean,
			details: MenuOpenChangeDetails,
		): void {
			const changed = open_state.set(open, details);
			if (!changed) {
				return;
			}
			last_open_details = details;
			void handle.update();
		}

		function get_items(): RegisteredMenuItem[] {
			return item_collection.getItems();
		}

		function find_item(
			value: string | null,
		): RegisteredMenuItem | undefined {
			if (value === null) {
				return undefined;
			}
			return item_collection.findByValue(value);
		}

		function focus_item(item: RegisteredMenuItem | undefined): void {
			if (item === undefined) {
				return;
			}
			handle.queueTask((signal) => {
				if (signal.aborted) {
					return;
				}
				item.node.focus();
			});
		}

		function set_current_item(
			value: string | null,
			details: MenuItemFocusDetails,
			focus = false,
		): void {
			const item = find_item(value);
			if (value !== null && item === undefined) {
				return;
			}
			const changed = current_item_value !== value;
			current_item_value = value;
			if (focus) {
				focus_item(item);
			}
			if (changed) {
				void handle.update();
			}
		}

		function focus_open_item(focus: MenuOpenFocus): void {
			const items = get_items();
			const item =
				focus === "last"
					? get_last_collection_item(items)
					: get_first_collection_item(items);
			if (item === undefined) {
				return;
			}
			set_current_item(
				item.value,
				{ reason: menuItemFocusReason.open },
				true,
			);
		}

		function open_menu(
			details: MenuOpenChangeDetails,
			focus: MenuOpenFocus = "first",
		): void {
			request_open(true, details);
			focus_open_item(focus);
		}

		function close_menu(details: MenuOpenChangeDetails): void {
			request_open(false, details);
		}

		function close_and_focus_trigger(details: MenuOpenChangeDetails): void {
			close_menu(details);
			handle.queueTask((signal) => {
				if (signal.aborted) {
					return;
				}
				popup_relationship.focusTrigger();
			});
		}

		function move_current_item(
			offset: number,
			details: MenuItemFocusDetails,
			clamp = false,
		): void {
			const item = get_collection_navigation_item({
				clamp,
				currentValue: current_item_value,
				items: get_items(),
				loop: handle.props.loopFocus,
				offset,
			});
			if (item === undefined) {
				return;
			}
			set_current_item(item.value, details, true);
		}

		function select_item(
			value: string,
			details: MenuItemSelectDetails,
		): void {
			const item = find_item(value);
			if (item === undefined || item.disabled) {
				return;
			}
			item.activate(details);
		}

		function search_item(text: string, event: KeyboardEvent): void {
			if (handle.props.typeahead === false) {
				return;
			}

			const item = item_typeahead.search({
				currentValue: current_item_value,
				items: get_items(),
				key: text,
			});
			if (item === undefined) {
				return;
			}
			set_current_item(
				item.value,
				{
					event,
					reason: menuItemFocusReason.typeahead,
				},
				true,
			);
		}

		function handle_popup_keydown(event: KeyboardEvent): void {
			const keyboard_focus_details: MenuItemFocusDetails = {
				event,
				reason: menuItemFocusReason.keyboard,
			};

			if (event.key === "ArrowDown") {
				event.preventDefault();
				move_current_item(1, keyboard_focus_details);
				return;
			}

			if (event.key === "ArrowUp") {
				event.preventDefault();
				move_current_item(-1, keyboard_focus_details);
				return;
			}

			if (event.key === "Home") {
				event.preventDefault();
				const item = get_first_collection_item(get_items());
				set_current_item(
					item?.value ?? null,
					keyboard_focus_details,
					true,
				);
				return;
			}

			if (event.key === "End") {
				event.preventDefault();
				const item = get_last_collection_item(get_items());
				set_current_item(
					item?.value ?? null,
					keyboard_focus_details,
					true,
				);
				return;
			}

			if (is_menu_keyboard_selection(event)) {
				event.preventDefault();
				if (current_item_value !== null) {
					select_item(current_item_value, {
						event,
						reason: menuItemSelectReason.keyboard,
					});
				}
				return;
			}

			if (event.key === "Escape") {
				event.preventDefault();
				close_and_focus_trigger({
					event,
					reason: menuOpenChangeReason.escape,
				});
				return;
			}

			if (event.key === "Tab") {
				close_menu({
					event,
					reason: menuOpenChangeReason.tab,
				});
				return;
			}

			if (is_printable_key_event(event)) {
				event.preventDefault();
				search_item(event.key, event);
			}
		}

		const context: MenuContext = {
			close: (details) => {
				close_menu(details);
			},
			get_close_on_select: () => {
				return handle.props.closeOnSelect !== false;
			},
			get_current_item_value: () => {
				return current_item_value;
			},
			get_open,
			get_popup_relationship: () => {
				return popup_relationship;
			},
			get_popup_id: () => {
				return `${handle.id}-popup`;
			},
			get_trigger_id: () => {
				return (
					popup_relationship.getTrigger()?.id ||
					`${handle.id}-trigger`
				);
			},
			handle_popup_keydown,
			open: open_menu,
			register_item: (item) => {
				const changed = item_collection.register(item);
				if (get_open() && current_item_value === null) {
					const first_item = get_first_collection_item(get_items());
					set_current_item(
						first_item?.value ?? null,
						{ reason: menuItemFocusReason.open },
						true,
					);
				}
				if (changed) {
					void handle.update();
				}
			},
			register_trigger: (node) => {
				const changed = popup_relationship.getTrigger() !== node;
				popup_relationship.registerTrigger(node);
				if (changed) {
					void handle.update();
				}
			},
			select_item,
			set_current_item,
			toggle: (details, focus = "first") => {
				if (get_open()) {
					close_menu(details);
					return;
				}
				open_menu(details, focus);
			},
			sync_popup: () => {
				popup_relationship.syncPopup(get_open());
			},
			unregister_item: (id) => {
				const item = item_collection.unregister(id);
				if (item?.value === current_item_value) {
					current_item_value = null;
				}
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: MenuRootProps): RemixNode => {
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
			return props.children;
		};
	}

	function Trigger(
		handle: Handle<MenuTriggerProps<TVariant, TSize, TBreakpoint>>,
	): (props: MenuTriggerProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (
			props: MenuTriggerProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				mix,
				size,
				type = button_type_default,
				variant,
				...trigger_props
			} = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					[menuAnatomy.trigger]: "button",
				},
				targets: {
					[menuAnatomy.trigger]: {
						host: menuAnatomy.trigger,
						conditions: menu_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot(
								menuAnatomy.trigger,
								current_props,
							);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});
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
						menu_scope,
						menuAnatomy.trigger,
					),
					mix: [
						trigger_ref_mix,
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								context.toggle({
									event,
									reason: menuOpenChangeReason.trigger,
								});
							},
						),
						on<HTMLButtonElement, typeof keydown_event>(
							keydown_event,
							(event) => {
								if (
									event.key === "ArrowDown" ||
									event.key === "Enter" ||
									event.key === " " ||
									event.key === "Spacebar"
								) {
									event.preventDefault();
									context.open({
										event,
										reason: menuOpenChangeReason.trigger,
									});
									return;
								}
								if (event.key === "ArrowUp") {
									event.preventDefault();
									context.open(
										{
											event,
											reason: menuOpenChangeReason.trigger,
										},
										"last",
									);
								}
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-controls": context.get_popup_id(),
						"aria-expanded": ariaBoolean(open),
						"aria-haspopup": "menu",
						[componentDataAttribute.open]: dataFlag(open),
						[componentStateAttribute]: openStateFromBoolean(open),
						id: trigger_props.id ?? context.get_trigger_id(),
						mix,
						type,
					},
				}),
				children,
			);
		};
	}

	function Popup(
		handle: Handle<MenuPopupProps<TLayout, TBreakpoint>>,
	): (props: MenuPopupProps<TLayout, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (props: MenuPopupProps<TLayout, TBreakpoint>): RemixNode => {
			const { at, children, layout, mix, ...popup_props } = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					[menuAnatomy.popup]: "div",
				},
				targets: {
					[menuAnatomy.popup]: {
						host: menuAnatomy.popup,
						conditions: menu_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot(
								menuAnatomy.popup,
								current_props,
							);
						},
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
						reason: menuOpenChangeReason.interactOutside,
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
						menu_scope,
						menuAnatomy.popup,
					),
					mix: [
						popup_ref_mix,
						on<HTMLElement, typeof keydown_event>(
							keydown_event,
							(event) => {
								context.handle_popup_keydown(event);
							},
						),
						parts.hosts.popup.mix,
					],
					props: {
						...popup_props,
						"aria-labelledby": context.get_trigger_id(),
						[componentDataAttribute.open]: dataFlag(open),
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						id: popup_props.id ?? context.get_popup_id(),
						mix,
						popover: "manual",
						role: popup_props.role ?? "menu",
					},
				}),
				children,
			);
		};
	}

	type MenuItemARIAChecked = "false" | "mixed" | "true";

	type MenuItemRenderInput = {
		ariaChecked?: MenuItemARIAChecked;
		at?: Partial<
			Record<TBreakpoint, Partial<MenuItemStyleProps<TVariant, TSize>>>
		>;
		children?: RemixNode;
		disabled: boolean;
		hostProps: Omit<Props<"button">, "style">;
		mix?: unknown;
		onActivate: (details: MenuItemSelectDetails) => void;
		role: string;
		size?: TSize;
		state?: CheckableStateName;
		textValue: string;
		type: string;
		variant?: TVariant;
	};

	function create_menu_item_renderer<TProps extends object, TContext>(
		handle: Handle<TProps, TContext>,
		context: MenuContext,
	): (input: MenuItemRenderInput) => RemixNode {
		let item_node: HTMLButtonElement | null = null;
		let item_id: string | null = null;

		return (input: MenuItemRenderInput): RemixNode => {
			const current_item_id = `${handle.id}-item`;
			item_id = current_item_id;
			const current =
				context.get_current_item_value() === current_item_id;
			const parts = createComponentStyleTargets({
				at: input.at,
				hostElements: {
					[menuAnatomy.item]: "button",
				},
				targets: {
					[menuAnatomy.item]: {
						host: menuAnatomy.item,
						conditions: menu_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot(
								menuAnatomy.item,
								current_props,
							);
						},
					},
				},
				props: {
					size: input.size,
					variant: input.variant,
				},
				styleSystem: style_system,
			});

			function register_item(node: HTMLButtonElement): void {
				context.register_item({
					activate: input.onActivate,
					disabled: input.disabled,
					id: current_item_id,
					node,
					text: input.textValue || node.textContent || "",
					value: current_item_id,
				});
			}

			if (item_node) {
				handle.queueTask((signal) => {
					if (signal.aborted || !item_node) {
						return;
					}
					register_item(item_node);
				});
			}

			const item_ref_mix = ref<HTMLButtonElement>((node, signal) => {
				item_node = node;
				register_item(node);
				signal.addEventListener("abort", () => {
					if (item_id) {
						context.unregister_item(item_id);
					}
					item_node = null;
				});
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						menu_scope,
						menuAnatomy.item,
					),
					mix: [
						item_ref_mix,
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								if (input.disabled) {
									return;
								}
								context.select_item(current_item_id, {
									event,
									reason: menuItemSelectReason.pointer,
								});
							},
						),
						on<HTMLButtonElement, typeof focus_event>(
							focus_event,
							(event) => {
								context.set_current_item(current_item_id, {
									event,
									reason: menuItemFocusReason.pointer,
								});
							},
						),
						on<HTMLButtonElement, typeof pointermove_event>(
							pointermove_event,
							(event) => {
								context.set_current_item(current_item_id, {
									event,
									reason: menuItemFocusReason.pointer,
								});
							},
						),
						parts.hosts.item.mix,
					],
					props: {
						...input.hostProps,
						"aria-checked": input.ariaChecked,
						"aria-disabled": ariaTrue(input.disabled),
						[componentDataAttribute.disabled]: dataFlag(
							input.disabled,
						),
						[componentDataAttribute.highlighted]: dataFlag(current),
						[componentStateAttribute]: input.state,
						id: current_item_id,
						mix: input.mix,
						role: input.role,
						tabIndex: get_roving_tab_index({
							currentValue: context.get_current_item_value(),
							itemValue: current_item_id,
						}),
						type: input.type,
					},
				}),
				input.children ?? input.textValue,
			);
		};
	}

	function Item(
		handle: Handle<MenuItemProps<TVariant, TSize, TBreakpoint>>,
	): (props: MenuItemProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		const render_item = create_menu_item_renderer(handle, context);

		return (
			props: MenuItemProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				disabled = false,
				mix,
				size,
				textValue,
				type = button_type_default,
				variant,
				...item_props
			} = props;
			const item_text = textValue ?? infer_text_value(children);
			return render_item({
				at,
				children,
				disabled,
				hostProps: item_props,
				mix,
				onActivate: (details) => {
					if (context.get_close_on_select()) {
						context.close({
							event: details.event,
							reason: menuOpenChangeReason.item,
						});
					}
				},
				role: item_props.role ?? "menuitem",
				size,
				textValue: item_text,
				type,
				variant,
			});
		};
	}

	function CheckboxItem(
		handle: Handle<
			MenuCheckboxItemProps<TVariant, TSize, TBreakpoint>,
			MenuItemRuntimeContext
		>,
	): (
		props: MenuCheckboxItemProps<TVariant, TSize, TBreakpoint>,
	) => RemixNode {
		const context = handle.context.get(Root);
		let local_checked = handle.props.defaultChecked ?? false;
		const render_item = create_menu_item_renderer(handle, context);
		const checked_state = create_controllable_state<
			MenuChecked,
			MenuItemSelectDetails
		>({
			getControlled: () => {
				return handle.props.checked;
			},
			getLocal: () => {
				return local_checked;
			},
			getOnChange: () => {
				return handle.props.onCheckedChange;
			},
			setLocal: (checked) => {
				local_checked = checked;
			},
		});
		const item_context: MenuItemRuntimeContext = {
			get_state: () => {
				return checkableStateFromValue(checked_state.get());
			},
		};
		handle.context.set(item_context);

		return (
			props: MenuCheckboxItemProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				checked: _checked,
				children,
				defaultChecked: _default_checked,
				disabled = false,
				mix,
				onCheckedChange: _on_checked_change,
				size,
				textValue,
				type = button_type_default,
				variant,
				...item_props
			} = props;
			const item_text = textValue ?? infer_text_value(children);
			const state = checkableStateFromValue(checked_state.get());
			const aria_checked =
				state === checkableState.indeterminate
					? "mixed"
					: state === checkableState.checked
						? "true"
						: "false";
			return render_item({
				ariaChecked: aria_checked,
				at,
				children,
				disabled,
				hostProps: item_props,
				mix,
				onActivate: (details) => {
					const next_checked =
						checked_state.get() === true ? false : true;
					checked_state.set(next_checked, details);
					if (context.get_close_on_select()) {
						context.close({
							event: details.event,
							reason: menuOpenChangeReason.item,
						});
					}
					void handle.update();
				},
				role: item_props.role ?? "menuitemcheckbox",
				size,
				state,
				textValue: item_text,
				type,
				variant,
			});
		};
	}

	function RadioGroup(
		handle: Handle<MenuRadioGroupProps, MenuRadioGroupContext>,
	): (props: MenuRadioGroupProps) => RemixNode {
		let local_value = handle.props.defaultValue ?? null;

		function get_value(): string | null {
			if (handle.props.value !== undefined) {
				return handle.props.value;
			}
			return local_value;
		}

		function set_value(
			value: string,
			details: MenuItemSelectDetails,
		): void {
			if (Object.is(get_value(), value)) {
				return;
			}
			if (handle.props.value === undefined) {
				local_value = value;
			}
			handle.props.onValueChange?.(value, details);
			void handle.update();
		}

		const context: MenuRadioGroupContext = {
			get_value,
			set_value,
		};
		handle.context.set(context);

		return (props: MenuRadioGroupProps): RemixNode => {
			return props.children;
		};
	}

	function RadioItem(
		handle: Handle<
			MenuRadioItemProps<string, TVariant, TSize, TBreakpoint>,
			MenuItemRuntimeContext
		>,
	): (
		props: MenuRadioItemProps<string, TVariant, TSize, TBreakpoint>,
	) => RemixNode {
		const context = handle.context.get(Root);
		const radio_group_context = handle.context.get(RadioGroup);
		const render_item = create_menu_item_renderer(handle, context);
		const item_context: MenuItemRuntimeContext = {
			get_state: () => {
				return checkableStateFromValue(
					radio_group_context.get_value() === handle.props.value,
					false,
				);
			},
		};
		handle.context.set(item_context);

		return (
			props: MenuRadioItemProps<string, TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				disabled = false,
				mix,
				size,
				textValue,
				type = button_type_default,
				value,
				variant,
				...item_props
			} = props;
			const item_text = textValue ?? infer_text_value(children);
			const checked = radio_group_context.get_value() === value;
			const state = checkableStateFromValue(checked, false);
			return render_item({
				ariaChecked: ariaBoolean(checked),
				at,
				children,
				disabled,
				hostProps: item_props,
				mix,
				onActivate: (details) => {
					radio_group_context.set_value(value, details);
					if (context.get_close_on_select()) {
						context.close({
							event: details.event,
							reason: menuOpenChangeReason.item,
						});
					}
				},
				role: item_props.role ?? "menuitemradio",
				size,
				state,
				textValue: item_text,
				type,
				variant,
			});
		};
	}

	function ItemIndicator(
		handle: Handle<MenuItemIndicatorProps>,
	): (props: MenuItemIndicatorProps) => RemixNode {
		const checkbox_item_context = handle.context.get(CheckboxItem) as
			| MenuItemRuntimeContext
			| undefined;
		const radio_item_context = handle.context.get(RadioItem) as
			| MenuItemRuntimeContext
			| undefined;

		return (props: MenuItemIndicatorProps): RemixNode => {
			const { children, mix, ...indicator_props } = props;
			const state =
				checkbox_item_context?.get_state() ??
				radio_item_context?.get_state();
			const parts = createComponentStyleTargets({
				targets: {
					[menuAnatomy.itemIndicator]: {
						host: menuAnatomy.itemIndicator,
						conditions: menu_conditions,
						resolveSlot: () => {
							return resolve_slot(menuAnatomy.itemIndicator, {});
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
						menu_scope,
						menuAnatomy.itemIndicator,
					),
					mix: parts.hosts[menuAnatomy.itemIndicator].mix,
					props: {
						...indicator_props,
						[componentStateAttribute]: state,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Group(
		handle: Handle<MenuGroupProps, MenuGroupContext>,
	): (props: MenuGroupProps) => RemixNode {
		const group_context = create_group_label_relationship({
			label_id: `${handle.id}-label`,
			on_change: () => {
				void handle.update();
			},
		});
		handle.context.set(group_context);

		return (props: MenuGroupProps): RemixNode => {
			const { children, mix, ...group_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[menuAnatomy.group]: {
						host: menuAnatomy.group,
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot(menuAnatomy.group, {});
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
						menu_scope,
						menuAnatomy.group,
					),
					mix: parts.hosts[menuAnatomy.group].mix,
					props: {
						...group_props,
						"aria-labelledby": group_context.get_labelled_by(),
						mix,
						role: group_props.role ?? "group",
					},
				}),
				children,
			);
		};
	}

	function GroupLabel(
		handle: Handle<MenuGroupLabelProps>,
	): (props: MenuGroupLabelProps) => RemixNode {
		const group_context = handle.context.get(Group);

		return (props: MenuGroupLabelProps): RemixNode => {
			const { children, mix, ...group_label_props } = props;
			const parts = createComponentStyleTargets({
				targets: {
					[menuAnatomy.groupLabel]: {
						host: menuAnatomy.groupLabel,
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot(menuAnatomy.groupLabel, {});
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
						menu_scope,
						menuAnatomy.groupLabel,
					),
					mix: [
						group_label_ref_mix,
						parts.hosts[menuAnatomy.groupLabel].mix,
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

	function create_static_part<TElement extends "div">(
		part: typeof menuAnatomy.separator,
		role: string | undefined,
	): RemixComponent<Omit<Props<TElement>, "style"> & { style?: never }> {
		return () => {
			return (
				props: Omit<Props<TElement>, "style"> & { style?: never },
			): RemixNode => {
				const { children, mix, ...part_props } = props;
				const parts = createComponentStyleTargets({
					targets: {
						[part]: {
							host: part,
							conditions: commonConditions,
							resolveSlot: () => {
								return resolve_slot(part, {});
							},
						},
					} as Record<
						typeof part,
						{
							conditions: typeof commonConditions;
							host: typeof part;
							resolveSlot: () => ReturnType<
								typeof recipe.resolve
							>["slots"][typeof part];
						}
					>,
					props: {},
					styleSystem: style_system,
				});

				return createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(menu_scope, part),
						mix: parts.hosts[part].mix,
						props: {
							...part_props,
							mix,
							role: part_props.role ?? role,
						},
					}),
					children,
				);
			};
		};
	}

	return {
		CheckboxItem,
		Group,
		GroupLabel,
		Item,
		ItemIndicator,
		Popup,
		RadioGroup,
		RadioItem,
		Root,
		Separator: create_static_part(menuAnatomy.separator, "separator"),
		Trigger,
	};
}

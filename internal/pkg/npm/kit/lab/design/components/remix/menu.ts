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
	componentStateAttribute,
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

export type MenuRecipeCondition = CommonRecipeCondition | "closed" | "open";

export type MenuRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"group" | "groupLabel" | "item" | "popup" | "separator" | "trigger",
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
	Group: RemixComponent<MenuGroupProps, MenuGroupContext>;
	GroupLabel: RemixComponent<MenuGroupLabelProps>;
	Item: RemixComponent<MenuItemProps<TVariant, TSize>>;
	Popup: RemixComponent<MenuPopupProps<TLayout>>;
	Root: RemixComponent<MenuRootProps, MenuContext>;
	Separator: RemixComponent<MenuSeparatorProps>;
	Trigger: RemixComponent<MenuTriggerProps<TVariant, TSize>>;
};

type MenuContext = {
	close: (details: MenuOpenChangeDetails) => void;
	get_close_on_select: () => boolean;
	get_current_item_value: () => string | null;
	get_open: () => boolean;
	get_popup_id: () => string;
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
};

type MenuGroupContext = GroupLabelRelationship;

export const menuOpenChangeReason = {
	escape: "escape",
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

type RegisteredMenuItem = OrderedCollectionItem<string>;

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
		closed: `&[${componentStateAttribute}='${openState.closed}']`,
		open: `&[${componentStateAttribute}='${openState.open}']`,
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
		slot:
			| "group"
			| "groupLabel"
			| "item"
			| "popup"
			| "separator"
			| "trigger",
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
		let trigger_node: HTMLButtonElement | null = null;
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
				trigger_node?.focus();
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
			if (handle.props.closeOnSelect !== false) {
				close_menu({
					event: details.event,
					reason: menuOpenChangeReason.item,
				});
			}
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
			get_popup_id: () => {
				return `${handle.id}-popup`;
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
				trigger_node = node;
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
					trigger: "button",
				},
				targets: {
					trigger: {
						host: "trigger",
						conditions: menu_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("trigger", current_props);
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
					attrs: createComponentAnatomyAttrs(menu_scope, "trigger"),
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
						"aria-expanded": open,
						"aria-haspopup": "menu",
						[componentStateAttribute]: openStateFromBoolean(open),
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
					popup: "div",
				},
				targets: {
					popup: {
						host: "popup",
						conditions: menu_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("popup", current_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(menu_scope, "popup"),
					mix: [
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
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						id: popup_props.id ?? context.get_popup_id(),
						mix,
						role: popup_props.role ?? "menu",
					},
				}),
				children,
			);
		};
	}

	function Item(
		handle: Handle<MenuItemProps<TVariant, TSize, TBreakpoint>>,
	): (props: MenuItemProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		let item_node: HTMLButtonElement | null = null;
		let item_id: string | null = null;

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
			const current_item_id = `${handle.id}-item`;
			item_id = current_item_id;
			const current =
				context.get_current_item_value() === current_item_id;
			const item_text = textValue ?? infer_text_value(children);
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					item: "button",
				},
				targets: {
					item: {
						host: "item",
						conditions: commonConditions,
						resolveSlot: (current_props) => {
							return resolve_slot("item", current_props);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});
			function register_item(node: HTMLButtonElement): void {
				context.register_item({
					disabled,
					id: current_item_id,
					node,
					text: item_text || node.textContent || "",
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
					attrs: createComponentAnatomyAttrs(menu_scope, "item"),
					mix: [
						item_ref_mix,
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								if (disabled) {
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
						...item_props,
						"aria-disabled": disabled ? "true" : undefined,
						"data-disabled": disabled ? "" : undefined,
						"data-highlighted": current ? "" : undefined,
						id: current_item_id,
						mix,
						role: item_props.role ?? "menuitem",
						tabIndex: get_roving_tab_index({
							currentValue: context.get_current_item_value(),
							itemValue: current_item_id,
						}),
						type,
					},
				}),
				children ?? item_text,
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
					group: {
						host: "group",
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot("group", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(menu_scope, "group"),
					mix: parts.hosts.group.mix,
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
					groupLabel: {
						host: "groupLabel",
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot("groupLabel", {});
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
						"groupLabel",
					),
					mix: [group_label_ref_mix, parts.hosts.groupLabel.mix],
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
		part: "separator",
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
		Group,
		GroupLabel,
		Item,
		Popup,
		Root,
		Separator: create_static_part("separator", "separator"),
		Trigger,
	};
}

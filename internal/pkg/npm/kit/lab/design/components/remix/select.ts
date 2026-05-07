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
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import {
	popoverScrollLockGuard,
	releasePopoverScrollLock,
} from "./popover-scroll-lock.ts";
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

export type SelectOpenChangeReason =
	| "escape"
	| "interactOutside"
	| "option"
	| "trigger";

export type SelectOpenChangeDetails = {
	event?: Event;
	reason: SelectOpenChangeReason;
};

export type SelectValueChangeDetails = {
	event?: Event;
};

export type SelectValueChangeHandler<TValue extends string = string> = (
	value: TValue | null,
	details?: SelectValueChangeDetails,
) => void;

export type SelectOpenChangeHandler = (
	open: boolean,
	details?: SelectOpenChangeDetails,
) => void;

export type SelectProps<TValue extends string = string> = {
	children?: RemixNode;
	defaultOpen?: boolean;
	defaultValue?: TValue | null;
	disabled?: boolean;
	name?: string;
	onOpenChange?: SelectOpenChangeHandler;
	onOpenChangeComplete?: SelectOpenChangeHandler;
	onValueChange?: SelectValueChangeHandler<TValue>;
	open?: boolean;
	placeholder?: string;
	required?: boolean;
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
	Group: RemixComponent<SelectGroupProps>;
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
	contains_target: (target: EventTarget | null) => boolean;
	get_disabled: () => boolean;
	get_highlighted_id: () => string | undefined;
	get_highlighted_value: () => string | null;
	get_list_id: () => string;
	get_open: () => boolean;
	get_placeholder: () => string | undefined;
	get_popup_id: () => string;
	get_selected_text: () => string | undefined;
	get_trigger_id: () => string;
	get_value: () => string | null;
	highlight_first: () => void;
	highlight_last: () => void;
	highlight_next: () => void;
	highlight_previous: () => void;
	highlight_value: (value: string | null) => void;
	open: (details: SelectOpenChangeDetails) => void;
	register_list: (node: HTMLElement | null) => void;
	register_option: (option: RegisteredSelectOption) => void;
	register_popup: (node: HTMLElement | null) => void;
	register_trigger: (node: HTMLElement | null) => void;
	search: (text: string) => void;
	select_value: (
		value: string | null,
		details?: SelectValueChangeDetails,
	) => void;
	sync_popup: () => void;
	toggle: (details: SelectOpenChangeDetails) => void;
	unregister_option: (id: string) => void;
};

type RegisteredSelectOption = {
	disabled: boolean;
	id: string;
	node: HTMLElement;
	text: string;
	value: string;
};

const trigger_conditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	open: "&[aria-expanded='true']",
	placeholder: "&[data-placeholder]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const option_conditions = {
	disabled: "&[aria-disabled='true']",
	highlighted: "&[data-highlighted]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
	selected: "&[aria-selected='true']",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const open_conditions = {
	open: "&:popover-open, &[data-open]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const value_conditions = {
	placeholder: "&[data-placeholder]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const select_scope = "select";
const select_option_value_attribute = "data-vorma-select-value";
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

function sync_native_popover(node: HTMLElement, open: boolean): void {
	if (open) {
		node.hidden = false;
		if ("showPopover" in node && !node.matches(":popover-open")) {
			node.showPopover();
		}
		return;
	}

	if ("hidePopover" in node && node.matches(":popover-open")) {
		node.hidePopover();
	}
	node.hidden = true;
	releasePopoverScrollLock(node.ownerDocument);
}

function contains_event_target(
	node: HTMLElement | null,
	target: EventTarget | null,
): boolean {
	return target instanceof Node && node?.contains(target) === true;
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
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: SelectOpenChangeDetails = {
			reason: "trigger",
		};
		let list_node: HTMLElement | null = null;
		let popup_node: HTMLElement | null = null;
		let trigger_node: HTMLElement | null = null;
		let highlighted_value: string | null = null;
		let typeahead_text = "";
		let typeahead_timer: number | undefined;
		const options_by_id = new Map<string, RegisteredSelectOption>();

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		function get_value(): string | null {
			return handle.props.value ?? local_value;
		}

		function get_enabled_options(): RegisteredSelectOption[] {
			return Array.from(options_by_id.values()).filter((option) => {
				return !option.disabled;
			});
		}

		function find_option(
			value: string | null,
		): RegisteredSelectOption | undefined {
			if (value === null) {
				return undefined;
			}
			return Array.from(options_by_id.values()).find((option) => {
				return option.value === value;
			});
		}

		function request_open(
			open: boolean,
			details: SelectOpenChangeDetails,
		): void {
			if (open && handle.props.disabled) {
				return;
			}
			if (get_open() === open) {
				return;
			}

			last_open_details = details;
			if (handle.props.open === undefined) {
				local_open = open;
			}
			handle.props.onOpenChange?.(open, details);
			void handle.update();
		}

		function highlight_option(
			option: RegisteredSelectOption | undefined,
		): void {
			const next_value = option?.value ?? null;
			if (highlighted_value === next_value) {
				return;
			}

			highlighted_value = next_value;
			void handle.update();
		}

		function move_highlight(offset: number): void {
			const enabled_options = get_enabled_options();
			if (enabled_options.length === 0) {
				highlight_option(undefined);
				return;
			}

			const current_index = enabled_options.findIndex((option) => {
				return option.value === highlighted_value;
			});
			const selected_index = enabled_options.findIndex((option) => {
				return option.value === get_value();
			});
			const base_index =
				current_index >= 0 ? current_index : selected_index;
			const next_index =
				base_index >= 0
					? (base_index + offset + enabled_options.length) %
						enabled_options.length
					: offset > 0
						? 0
						: enabled_options.length - 1;

			highlight_option(enabled_options[next_index]);
		}

		function select_value(
			value: string | null,
			details?: SelectValueChangeDetails,
		): void {
			const option = find_option(value);
			if (option?.disabled) {
				return;
			}
			if (get_value() !== value) {
				if (handle.props.value === undefined) {
					local_value = value;
				}
				handle.props.onValueChange?.(value, details);
			}
			highlighted_value = value;
			request_open(false, {
				event: details?.event,
				reason: "option",
			});
			void handle.update();
		}

		function search(text: string): void {
			window.clearTimeout(typeahead_timer);
			typeahead_text += text.toLowerCase();
			typeahead_timer = window.setTimeout(() => {
				typeahead_text = "";
			}, select_typeahead_reset_ms);

			const enabled_options = get_enabled_options();
			if (enabled_options.length === 0) {
				return;
			}

			const start_index = Math.max(
				0,
				enabled_options.findIndex((option) => {
					return option.value === highlighted_value;
				}),
			);
			const ordered_options = [
				...enabled_options.slice(start_index + 1),
				...enabled_options.slice(0, start_index + 1),
			];
			const match = ordered_options.find((option) => {
				return option.text.toLowerCase().startsWith(typeahead_text);
			});

			highlight_option(match);
		}

		const context: SelectRuntimeContext = {
			close: (details) => {
				request_open(false, details);
			},
			contains_target: (target) => {
				return (
					contains_event_target(trigger_node, target) ||
					contains_event_target(popup_node, target)
				);
			},
			get_disabled: () => {
				return handle.props.disabled === true;
			},
			get_highlighted_id: () => {
				return find_option(highlighted_value)?.id;
			},
			get_highlighted_value: () => {
				return highlighted_value;
			},
			get_list_id: () => {
				return `${handle.id}-list`;
			},
			get_open,
			get_placeholder: () => {
				return handle.props.placeholder;
			},
			get_popup_id: () => {
				return `${handle.id}-popup`;
			},
			get_selected_text: () => {
				return find_option(get_value())?.text;
			},
			get_trigger_id: () => {
				return `${handle.id}-trigger`;
			},
			get_value,
			highlight_first: () => {
				highlight_option(get_enabled_options()[0]);
			},
			highlight_last: () => {
				const enabled_options = get_enabled_options();
				highlight_option(enabled_options[enabled_options.length - 1]);
			},
			highlight_next: () => {
				move_highlight(1);
			},
			highlight_previous: () => {
				move_highlight(-1);
			},
			highlight_value: (value) => {
				highlight_option(find_option(value));
			},
			open: (details) => {
				if (highlighted_value === null) {
					highlight_option(
						find_option(get_value()) ?? get_enabled_options()[0],
					);
				}
				request_open(true, details);
			},
			register_list: (node) => {
				list_node = node;
			},
			register_option: (option) => {
				options_by_id.set(option.id, option);
			},
			register_popup: (node) => {
				popup_node = node;
			},
			register_trigger: (node) => {
				trigger_node = node;
			},
			search,
			select_value,
			sync_popup: () => {
				if (popup_node) {
					sync_native_popover(popup_node, get_open());
				}
				if (get_open()) {
					list_node?.focus();
				}
			},
			toggle: (details) => {
				if (get_open()) {
					request_open(false, details);
					return;
				}
				context.open(details);
			},
			unregister_option: (id) => {
				const option = options_by_id.get(id);
				options_by_id.delete(id);
				if (option?.value === highlighted_value) {
					highlighted_value = null;
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
				props.name
					? createElement("input", {
							name: props.name,
							required: props.required,
							type: "hidden",
							value: get_value() ?? "",
						})
					: null,
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
					context.toggle({ event, reason: "trigger" });
				},
			);
			const trigger_keydown_mix = on<HTMLButtonElement, "keydown">(
				"keydown",
				(event) => {
					if (
						event.key !== "ArrowDown" &&
						event.key !== "ArrowUp" &&
						event.key !== "Enter" &&
						event.key !== " " &&
						event.key !== "Spacebar"
					) {
						return;
					}

					event.preventDefault();
					if (event.key === "ArrowUp") {
						context.highlight_last();
					} else {
						context.highlight_first();
					}
					context.open({ event, reason: "trigger" });
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
						"aria-controls": context.get_list_id(),
						"aria-disabled": context.get_disabled()
							? "true"
							: undefined,
						"aria-expanded": is_open ? "true" : "false",
						"aria-haspopup": "listbox",
						"data-open": is_open ? "" : undefined,
						"data-placeholder": has_value ? undefined : "",
						disabled: context.get_disabled(),
						id: context.get_trigger_id(),
						mix,
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
			const popup_ref_mix = ref<HTMLElement>((node, signal) => {
				context.register_popup(node);
				sync_native_popover(node, context.get_open());
				const on_pointer_down = (event: PointerEvent): void => {
					if (!context.get_open()) {
						return;
					}
					if (context.contains_target(event.target)) {
						return;
					}
					context.close({ event, reason: "interactOutside" });
				};
				node.ownerDocument.addEventListener(
					"pointerdown",
					on_pointer_down,
					true,
				);
				signal.addEventListener("abort", () => {
					context.register_popup(null);
					node.ownerDocument.removeEventListener(
						"pointerdown",
						on_pointer_down,
						true,
					);
				});
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
						"data-open": is_open ? "" : undefined,
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
			const highlighted_id = context.get_highlighted_id();
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
					if (event.key === "ArrowDown") {
						event.preventDefault();
						context.highlight_next();
						return;
					}
					if (event.key === "ArrowUp") {
						event.preventDefault();
						context.highlight_previous();
						return;
					}
					if (event.key === "Home") {
						event.preventDefault();
						context.highlight_first();
						return;
					}
					if (event.key === "End") {
						event.preventDefault();
						context.highlight_last();
						return;
					}
					if (is_select_keyboard_selection(event)) {
						event.preventDefault();
						context.select_value(context.get_highlighted_value(), {
							event,
						});
						return;
					}
					if (event.key === "Escape") {
						event.preventDefault();
						context.close({ event, reason: "escape" });
						return;
					}
					if (event.key === "Tab") {
						context.close({ event, reason: "interactOutside" });
						return;
					}
					if (is_printable_key_event(event)) {
						event.preventDefault();
						context.search(event.key);
					}
				},
			);
			const list_ref_mix = ref<HTMLElement>((node, signal) => {
				context.register_list(node);
				signal.addEventListener("abort", () => {
					context.register_list(null);
				});
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.list,
					),
					mix: [
						list_ref_mix,
						list_keydown_mix,
						parts.hosts[select_slot.list].mix,
					],
					props: {
						...list_props,
						"aria-activedescendant": highlighted_id,
						"aria-labelledby": context.get_trigger_id(),
						id: context.get_list_id(),
						mix,
						role: "listbox",
						tabIndex: -1,
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
						"data-placeholder": is_placeholder ? "" : undefined,
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

	function Option(
		handle: Handle<SelectOptionProps>,
	): (props: SelectOptionProps) => RemixNode {
		const context = handle.context.get(Root);

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
			const option_id = `${handle.id}-option-${value}`;
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
					context.select_value(value, { event });
				},
			);
			const option_pointer_move_mix = on<HTMLElement, "pointermove">(
				"pointermove",
				() => {
					if (disabled) {
						return;
					}
					context.highlight_value(value);
				},
			);
			const option_ref_mix = ref<HTMLElement>((node, signal) => {
				context.register_option({
					disabled,
					id: option_id,
					node,
					text: option_text,
					value,
				});
				signal.addEventListener("abort", () => {
					context.unregister_option(option_id);
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
						option_click_mix,
						option_pointer_move_mix,
						parts.hosts[select_slot.option].mix,
					],
					props: {
						...option_props,
						"aria-disabled": disabled ? "true" : undefined,
						"aria-selected": selected ? "true" : "false",
						"data-highlighted": highlighted ? "" : undefined,
						"data-selected": selected ? "" : undefined,
						[select_option_value_attribute]: value,
						id: option_id,
						mix,
						role: "option",
					},
				}),
				children ?? option_text,
			);
		};
	}

	return {
		Group: create_static_div_component<SelectGroupProps>(
			select_slot.group,
			"group",
		),
		GroupLabel: create_static_div_component<SelectGroupLabelProps>(
			select_slot.groupLabel,
		),
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

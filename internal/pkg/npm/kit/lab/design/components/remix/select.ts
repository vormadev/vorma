import {
	createElement,
	on,
	type Handle,
	type Props,
	type RemixNode,
} from "remix/ui";
import * as remixPopover from "remix/ui/popover";
import * as remixSelect from "remix/ui/select";
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
	list: "list",
	option: "option",
	popup: "popup",
	trigger: "trigger",
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

export type SelectValueChangeHandler<TValue extends string = string> = (
	value: TValue | null,
) => void;

export type SelectProps<TValue extends string = string> = {
	children?: RemixNode;
	defaultValue?: TValue | null;
	disabled?: boolean;
	name?: string;
	onValueChange?: SelectValueChangeHandler<TValue>;
	onValueChangeClose?: SelectValueChangeHandler<TValue>;
	placeholder: string;
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

export type SelectOptionProps<TValue extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	disabled?: boolean;
	label: string;
	style?: never;
	value: TValue;
};

export type SelectComponents<
	TVariant extends string = string,
	TSize extends string = string,
	TPopupLayout extends string = string,
> = {
	Option: RemixComponent<SelectOptionProps>;
	Popup: RemixComponent<SelectPopupProps<TPopupLayout>>;
	Root: RemixComponent<SelectProps, SelectRuntimeContext>;
	Trigger: RemixComponent<SelectTriggerProps<TVariant, TSize>>;
};

export type SelectOptions<TVariant extends string, TSize extends string> = {
	conditions?: Partial<
		Readonly<Record<SelectRecipeSlot, RecipeConditionSelectorMap<string>>>
	>;
	defaultSize?: NoInfer<TSize>;
	defaultVariant?: NoInfer<TVariant>;
};

type SelectRuntimeContext = {
	emit_value_change: (value: string | null) => void;
	emit_value_change_close: (value: string | null) => void;
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
	open: "&:popover-open",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<SelectRecipeCondition>;

const select_scope = "select";
const select_option_value_attribute = "data-vorma-select-value";

function is_select_keyboard_selection(event: KeyboardEvent): boolean {
	return event.key === "Enter" || event.key === " ";
}

function get_active_select_option_value(list: HTMLElement): string | undefined {
	const active_id = list.getAttribute("aria-activedescendant");
	if (!active_id) {
		return undefined;
	}

	const active_option = list.ownerDocument.getElementById(active_id);
	if (!(active_option instanceof HTMLElement)) {
		return undefined;
	}
	if (active_option.getAttribute("aria-disabled") === "true") {
		return undefined;
	}

	return (
		active_option.getAttribute(select_option_value_attribute) ?? undefined
	);
}

export function createSelect<
	TMode extends string,
	TRecipe extends SelectRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: SelectStyleSystem<TMode, TRecipe, TMetadata>,
	options: SelectOptions<
		SelectRecipeVariant<TRecipe>,
		SelectRecipeSize<TRecipe>
	> = {},
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
		let current_value = handle.props.defaultValue ?? null;
		const context = {
			emit_value_change: (value: string | null): void => {
				if (current_value === value) {
					return;
				}
				current_value = value;
				handle.props.onValueChange?.(value);
			},
			emit_value_change_close: (value: string | null): void => {
				context.emit_value_change(value);
				handle.props.onValueChangeClose?.(value);
			},
		};
		handle.context.set(context);

		return (props: SelectProps): RemixNode => {
			const { children, defaultValue, disabled, name, placeholder } =
				props;

			return createElement(
				remixSelect.Context,
				{
					defaultLabel: placeholder,
					defaultValue: defaultValue ?? null,
					disabled,
					name,
				},
				children,
				name
					? createElement("input", { mix: remixSelect.hiddenInput() })
					: null,
			);
		};
	}

	function Trigger(
		handle: Handle<SelectTriggerProps<TVariant, TSize, TBreakpoint>>,
	): (props: SelectTriggerProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		function resolve_slot(
			props: Partial<SelectTriggerStyleProps<TVariant, TSize>>,
		): ReturnType<typeof select_recipe.resolve>["slots"]["trigger"] {
			return select_recipe.resolve({
				size: props.size,
				variant: props.variant,
			} satisfies SelectRecipeSelection<TRecipe>).slots.trigger;
		}

		return (
			props: SelectTriggerProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
			const {
				at,
				children,
				mix,
				size = options.defaultSize,
				type = "button",
				variant = options.defaultVariant,
				...trigger_props
			} = props;
			const parts = createComponentStyleTargets({
				at,
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
			const change_mix = remixSelect.onSelectChange((event) => {
				context.emit_value_change_close(event.value);
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.trigger,
					),
					mix: [
						remixSelect.trigger(),
						change_mix,
						parts.hosts[select_slot.trigger].mix,
					],
					props: {
						...trigger_props,
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

		function resolve_slot(
			slot: "list" | "popup",
			props: Partial<SelectPopupStyleProps<TPopupLayout>>,
		): ReturnType<typeof select_recipe.resolve>["slots"][typeof slot] {
			return select_recipe.resolve({
				popupLayout: props.layout,
			} satisfies SelectRecipeSelection<TRecipe>).slots[slot];
		}

		return (
			props: SelectPopupProps<TPopupLayout, TBreakpoint>,
		): RemixNode => {
			const { at, children, layout, mix, ...popup_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					[select_slot.list]: {
						host: select_slot.list,
						conditions: mergeRecipeConditionSelectors(
							open_conditions,
							options.conditions?.[select_slot.list],
						),
						resolveSlot: (style_props) => {
							return resolve_slot(select_slot.list, style_props);
						},
					},
					[select_slot.popup]: {
						host: select_slot.popup,
						conditions: mergeRecipeConditionSelectors(
							open_conditions,
							options.conditions?.[select_slot.popup],
						),
						resolveSlot: (style_props) => {
							return resolve_slot(select_slot.popup, style_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});
			const change_mix = remixSelect.onSelectChange((event) => {
				context.emit_value_change_close(event.value);
			});
			const keyboard_select_mix = on<HTMLElement, "keydown">(
				"keydown",
				(event) => {
					if (!is_select_keyboard_selection(event)) {
						return;
					}

					const value = get_active_select_option_value(
						event.currentTarget,
					);
					if (value === undefined) {
						return;
					}

					context.emit_value_change(value);
				},
			);

			return createElement(
				remixPopover.Context,
				{},
				createElement(
					"div",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							select_scope,
							select_slot.popup,
						),
						mix: [
							remixSelect.popover(),
							change_mix,
							parts.hosts[select_slot.popup].mix,
						],
						props: {
							...popup_props,
							mix,
						},
					}),
					createElement(
						"div",
						createComponentSlotProps({
							attrs: createComponentAnatomyAttrs(
								select_scope,
								select_slot.list,
							),
							mix: [
								remixSelect.list(),
								keyboard_select_mix,
								parts.hosts[select_slot.list].mix,
							],
						}),
						children,
					),
				),
			);
		};
	}

	function Option(
		handle: Handle<SelectOptionProps>,
	): (props: SelectOptionProps) => RemixNode {
		const context = handle.context.get(Root);

		return (props: SelectOptionProps): RemixNode => {
			const { children, disabled, label, mix, value, ...option_props } =
				props;
			const resolved = select_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					[select_slot.option]: {
						host: select_slot.option,
						conditions: mergeRecipeConditionSelectors(
							option_conditions,
							options.conditions?.[select_slot.option],
						),
						resolveSlot: () => {
							return resolved.slots.option;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			const option_select_mix = on<HTMLElement, "click">("click", () => {
				context.emit_value_change(value);
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						select_scope,
						select_slot.option,
					),
					mix: [
						disabled ? undefined : option_select_mix,
						remixSelect.option({ disabled, label, value }),
						parts.hosts[select_slot.option].mix,
					],
					props: {
						...option_props,
						[select_option_value_attribute]: value,
						mix,
					},
				}),
				children ?? label,
			);
		};
	}

	return {
		Option,
		Popup,
		Root,
		Trigger,
	};
}

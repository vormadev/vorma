import { createElement, on, type Handle, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	checkableChangeEvent,
	checkableInitialChecked,
	checkableState,
	checkableStateAttribute,
	checkableStateFromValue,
	checkableStateMixin,
} from "./checkable-state.ts";
import { ariaTrue, componentDataAttribute, dataFlag } from "./component-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
import { create_controllable_state } from "./controllable-state.ts";
import { formResetMixin } from "./form-reset.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type RadioGroupRecipeCondition = CommonRecipeCondition | "checked" | "unchecked";

export type RadioGroupRecipeInput<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = RecipeWithVariantGroups<
	"item" | "root",
	string,
	ComponentStyle,
	{
		layout: TLayout;
		size: TSize;
		variant: TVariant;
	}
>;

export type RadioGroupRecipeLayout<TRecipe extends RadioGroupRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type RadioGroupRecipeVariant<TRecipe extends RadioGroupRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type RadioGroupRecipeSize<TRecipe extends RadioGroupRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type RadioGroupRecipeSelection<TRecipe extends RadioGroupRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout" | "size" | "variant">;

export type RadioGroupStyleSystem<
	TMode extends string = string,
	TRecipe extends RadioGroupRecipeInput = RadioGroupRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			radioGroup: TRecipe;
		};
	},
	TMetadata
>;

export type RadioGroupValueChangeDetails = {
	event?: Event;
};

export type RadioGroupValueChangeHandler<TValue extends string = string> = (
	value: TValue,
	details?: RadioGroupValueChangeDetails,
) => void;

export type RadioGroupRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type RadioGroupItemStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type RadioGroupRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"div">, "style"> &
	RadioGroupRootStyleProps<TLayout> &
	ResponsiveProps<RadioGroupRootStyleProps<TLayout>, TBreakpoint> & {
		defaultValue?: TValue | null;
		disabled?: boolean;
		form?: string;
		name?: string;
		onValueChange?: RadioGroupValueChangeHandler<TValue>;
		readOnly?: boolean;
		required?: boolean;
		style?: never;
		value?: TValue | null;
	};

export type RadioGroupItemProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<
	Props<"input">,
	"checked" | "defaultChecked" | "name" | "style" | "type" | "value"
> &
	RadioGroupItemStyleProps<TVariant, TSize> &
	ResponsiveProps<RadioGroupItemStyleProps<TVariant, TSize>, TBreakpoint> & {
		style?: never;
		value: TValue;
	};

export type RadioGroupComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	Item: RemixComponent<RadioGroupItemProps<TVariant, TSize>>;
	Root: RemixComponent<RadioGroupRootProps<TLayout>, RadioGroupContext>;
};

type RadioGroupContext = {
	get_disabled: () => boolean;
	get_form: () => string | undefined;
	get_name: () => string;
	get_read_only: () => boolean;
	get_required: () => boolean;
	get_value: () => string | null;
	set_value: (value: string, details?: RadioGroupValueChangeDetails) => void;
	sync_items: () => void;
};

const radio_group_scope = "radioGroup";
const radio_group_role = "radiogroup";
const radio_type = "radio";

const item_conditions = mergeRecipeConditionSelectors<RadioGroupRecipeCondition>(
	commonConditions,
	{
		checked: `&[${checkableStateAttribute}='${checkableState.checked}']`,
		unchecked: `&[${checkableStateAttribute}='${checkableState.unchecked}']`,
	} satisfies RecipeConditionSelectorMap<RadioGroupRecipeCondition>,
);

export function createRadioGroup<
	TMode extends string,
	TRecipe extends RadioGroupRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: RadioGroupStyleSystem<TMode, TRecipe, TMetadata>,
): RadioGroupComponents<
	RadioGroupRecipeLayout<TRecipe>,
	RadioGroupRecipeVariant<TRecipe>,
	RadioGroupRecipeSize<TRecipe>
> {
	type TLayout = RadioGroupRecipeLayout<TRecipe>;
	type TVariant = RadioGroupRecipeVariant<TRecipe>;
	type TSize = RadioGroupRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = RadioGroupRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.radioGroup);

	function resolve_root_slot(
		props: Partial<RadioGroupRootStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies TSelection).slots.root;
	}

	function resolve_item_slot(
		props: Partial<RadioGroupItemStyleProps<TVariant, TSize>>,
	): ReturnType<typeof recipe.resolve>["slots"]["item"] {
		return recipe.resolve({
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots.item;
	}

	function Root(
		handle: Handle<RadioGroupRootProps<TLayout, TBreakpoint>, RadioGroupContext>,
	): (props: RadioGroupRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_value = handle.props.defaultValue ?? null;
		const value_state = create_controllable_state<
			string | null,
			RadioGroupValueChangeDetails
		>({
			getControlled: () => {
				if (handle.props.value === undefined) {
					return undefined;
				}
				return handle.props.value;
			},
			getLocal: () => {
				return local_value;
			},
			getOnChange: () => {
				const on_value_change = handle.props.onValueChange;
				if (!on_value_change) {
					return undefined;
				}
				return (value, details) => {
					if (value === null) {
						return;
					}
					on_value_change(value, details);
				};
			},
			setLocal: (value) => {
				local_value = value;
			},
		});

		function reset_value(): void {
			if (handle.props.value !== undefined) {
				return;
			}
			local_value = handle.props.defaultValue ?? null;
			void handle.update();
		}

		const context: RadioGroupContext = {
			get_disabled: () => {
				return handle.props.disabled === true;
			},
			get_form: () => {
				return handle.props.form;
			},
			get_name: () => {
				return handle.props.name ?? handle.id;
			},
			get_read_only: () => {
				return handle.props.readOnly === true;
			},
			get_required: () => {
				return handle.props.required === true;
			},
			get_value: () => {
				return value_state.get();
			},
			set_value: (value, details) => {
				const changed = value_state.set(value, details ?? {});
				if (changed) {
					void handle.update();
				}
			},
			sync_items: () => {
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: RadioGroupRootProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultValue: _default_value,
				disabled,
				form: _form,
				layout,
				mix,
				name: _name,
				onValueChange: _on_value_change,
				readOnly,
				required: _required,
				value: _value,
				...root_props
			} = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: resolve_root_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(radio_group_scope, "root"),
					mix: [
						formResetMixin({
							form: _form,
							onReset: () => {
								reset_value();
							},
						}),
						parts.hosts.root.mix,
					],
					props: {
						...root_props,
						"aria-disabled": ariaTrue(disabled === true),
						"aria-required": ariaTrue(_required === true),
						[componentDataAttribute.disabled]: dataFlag(disabled === true),
						[componentDataAttribute.readOnly]: dataFlag(readOnly === true),
						[componentDataAttribute.required]: dataFlag(_required === true),
						mix,
						role: root_props.role ?? radio_group_role,
					},
				}),
				children,
			);
		};
	}

	function Item(
		handle: Handle<RadioGroupItemProps<TVariant, TSize, TBreakpoint>>,
	): (props: RadioGroupItemProps<TVariant, TSize, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (props: RadioGroupItemProps<TVariant, TSize, TBreakpoint>): RemixNode => {
			const {
				at,
				disabled,
				form,
				mix,
				readOnly,
				required,
				size,
				value,
				variant,
				...item_props
			} = props;
			const checked = context.get_value() === value;
			const is_disabled = context.get_disabled() || disabled === true;
			const is_read_only = context.get_read_only() || readOnly === true;
			const is_required = context.get_required() || required === true;
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					item: "input",
				},
				targets: {
					item: {
						host: "item",
						conditions: item_conditions,
						resolveSlot: resolve_item_slot,
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			return createElement(
				"input",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(radio_group_scope, "item"),
					mix: [
						checkableStateMixin({
							allowIndeterminate: false,
							checked,
							defaultChecked: undefined,
							disabled: is_disabled,
							readOnly: is_read_only,
						}),
						on<HTMLInputElement, typeof checkableChangeEvent>(
							checkableChangeEvent,
							(event) => {
								if (is_disabled || is_read_only) {
									context.sync_items();
									return;
								}
								if (event.currentTarget.checked) {
									context.set_value(value, { event });
								}
							},
						),
						parts.hosts.item.mix,
					],
					props: {
						...item_props,
						[checkableStateAttribute]: checkableStateFromValue(
							checked,
							false,
						),
						[componentDataAttribute.disabled]: dataFlag(is_disabled),
						[componentDataAttribute.readOnly]: dataFlag(is_read_only),
						[componentDataAttribute.required]: dataFlag(is_required),
						checked: checkableInitialChecked(checked),
						disabled: is_disabled || undefined,
						form: form ?? context.get_form(),
						mix,
						name: context.get_name(),
						readOnly: is_read_only || undefined,
						required: is_required || undefined,
						type: radio_type,
						value,
					},
				}),
			);
		};
	}

	return {
		Item,
		Root,
	};
}

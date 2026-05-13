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
	checkableChangeEvent,
	checkableInitialChecked,
	checkableState,
	checkableStateAttribute,
	checkableStateFromValue,
	checkableStateMixin,
} from "./checkable-state.ts";
import {
	ariaTrue,
	componentDataAttribute,
	dataFlag,
} from "./component-state.ts";
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
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type CheckboxGroupRecipeCondition =
	| CommonRecipeCondition
	| "checked"
	| "unchecked";

export type CheckboxGroupRecipeInput<
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

export type CheckboxGroupRecipeLayout<
	TRecipe extends CheckboxGroupRecipeInput,
> = RecipeVariantValue<TRecipe, "layout">;

export type CheckboxGroupRecipeVariant<
	TRecipe extends CheckboxGroupRecipeInput,
> = RecipeVariantValue<TRecipe, "variant">;

export type CheckboxGroupRecipeSize<TRecipe extends CheckboxGroupRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type CheckboxGroupRecipeSelection<
	TRecipe extends CheckboxGroupRecipeInput,
> = RecipeVariantPropsFor<TRecipe, "layout" | "size" | "variant">;

export type CheckboxGroupStyleSystem<
	TMode extends string = string,
	TRecipe extends CheckboxGroupRecipeInput = CheckboxGroupRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			checkboxGroup: TRecipe;
		};
	},
	TMetadata
>;

export type CheckboxGroupValueChangeDetails = {
	event?: Event;
};

export type CheckboxGroupValueChangeHandler<TValue extends string = string> = (
	value: readonly TValue[],
	details?: CheckboxGroupValueChangeDetails,
) => void;

export type CheckboxGroupRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type CheckboxGroupItemStyleProps<
	TVariant extends string = string,
	TSize extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type CheckboxGroupRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<Props<"div">, "style"> &
	CheckboxGroupRootStyleProps<TLayout> &
	ResponsiveProps<CheckboxGroupRootStyleProps<TLayout>, TBreakpoint> & {
		defaultValue?: readonly TValue[];
		disabled?: boolean;
		form?: string;
		name?: string;
		onValueChange?: CheckboxGroupValueChangeHandler<TValue>;
		readOnly?: boolean;
		required?: boolean;
		style?: never;
		value?: readonly TValue[];
	};

export type CheckboxGroupItemProps<
	TVariant extends string = string,
	TSize extends string = string,
	TBreakpoint extends string = string,
	TValue extends string = string,
> = Omit<
	Props<"input">,
	"checked" | "defaultChecked" | "name" | "style" | "type" | "value"
> &
	CheckboxGroupItemStyleProps<TVariant, TSize> &
	ResponsiveProps<
		CheckboxGroupItemStyleProps<TVariant, TSize>,
		TBreakpoint
	> & {
		style?: never;
		value: TValue;
	};

export type CheckboxGroupComponents<
	TLayout extends string = string,
	TVariant extends string = string,
	TSize extends string = string,
> = {
	Item: RemixComponent<CheckboxGroupItemProps<TVariant, TSize>>;
	Root: RemixComponent<CheckboxGroupRootProps<TLayout>, CheckboxGroupContext>;
};

type CheckboxGroupContext = {
	get_disabled: () => boolean;
	get_form: () => string | undefined;
	get_name: () => string | undefined;
	get_read_only: () => boolean;
	get_required: () => boolean;
	get_values: () => readonly string[];
	set_item_checked: (
		value: string,
		checked: boolean,
		details?: CheckboxGroupValueChangeDetails,
	) => void;
};

const checkbox_group_scope = "checkboxGroup";
const checkbox_group_role = "group";
const checkbox_type = "checkbox";

const item_conditions =
	mergeRecipeConditionSelectors<CheckboxGroupRecipeCondition>(
		commonConditions,
		{
			checked: `&[${checkableStateAttribute}='${checkableState.checked}']`,
			unchecked: `&[${checkableStateAttribute}='${checkableState.unchecked}']`,
		} satisfies RecipeConditionSelectorMap<CheckboxGroupRecipeCondition>,
	);

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

export function createCheckboxGroup<
	TMode extends string,
	TRecipe extends CheckboxGroupRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: CheckboxGroupStyleSystem<TMode, TRecipe, TMetadata>,
): CheckboxGroupComponents<
	CheckboxGroupRecipeLayout<TRecipe>,
	CheckboxGroupRecipeVariant<TRecipe>,
	CheckboxGroupRecipeSize<TRecipe>
> {
	type TLayout = CheckboxGroupRecipeLayout<TRecipe>;
	type TVariant = CheckboxGroupRecipeVariant<TRecipe>;
	type TSize = CheckboxGroupRecipeSize<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = CheckboxGroupRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.checkboxGroup);

	function resolve_root_slot(
		props: Partial<CheckboxGroupRootStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"]["root"] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies TSelection).slots.root;
	}

	function resolve_item_slot(
		props: Partial<CheckboxGroupItemStyleProps<TVariant, TSize>>,
	): ReturnType<typeof recipe.resolve>["slots"]["item"] {
		return recipe.resolve({
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots.item;
	}

	function Root(
		handle: Handle<
			CheckboxGroupRootProps<TLayout, TBreakpoint>,
			CheckboxGroupContext
		>,
	): (props: CheckboxGroupRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_values = [...(handle.props.defaultValue ?? [])];
		const values_state = create_controllable_state<
			readonly string[],
			CheckboxGroupValueChangeDetails
		>({
			equals: is_same_values,
			getControlled: () => {
				return handle.props.value;
			},
			getLocal: () => {
				return local_values;
			},
			getOnChange: () => {
				return handle.props.onValueChange;
			},
			setLocal: (values) => {
				local_values = [...values];
			},
		});

		function reset_values(): void {
			if (handle.props.value !== undefined) {
				return;
			}
			local_values = [...(handle.props.defaultValue ?? [])];
			void handle.update();
		}

		const context: CheckboxGroupContext = {
			get_disabled: () => {
				return handle.props.disabled === true;
			},
			get_form: () => {
				return handle.props.form;
			},
			get_name: () => {
				return handle.props.name;
			},
			get_read_only: () => {
				return handle.props.readOnly === true;
			},
			get_required: () => {
				return handle.props.required === true;
			},
			get_values: () => {
				return values_state.get();
			},
			set_item_checked: (value, checked, details) => {
				const current_values = values_state.get();
				const next_values = checked
					? add_value(current_values, value)
					: remove_value(current_values, value);
				const changed = values_state.set(next_values, details ?? {});
				if (changed) {
					void handle.update();
				}
			},
		};
		handle.context.set(context);

		return (
			props: CheckboxGroupRootProps<TLayout, TBreakpoint>,
		): RemixNode => {
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
					attrs: createComponentAnatomyAttrs(
						checkbox_group_scope,
						"root",
					),
					mix: [
						formResetMixin({
							form: _form,
							onReset: () => {
								reset_values();
							},
						}),
						parts.hosts.root.mix,
					],
					props: {
						...root_props,
						"aria-disabled": ariaTrue(disabled === true),
						"aria-required": ariaTrue(_required === true),
						[componentDataAttribute.disabled]: dataFlag(
							disabled === true,
						),
						[componentDataAttribute.readOnly]: dataFlag(
							readOnly === true,
						),
						[componentDataAttribute.required]: dataFlag(
							_required === true,
						),
						mix,
						role: root_props.role ?? checkbox_group_role,
					},
				}),
				children,
			);
		};
	}

	function Item(
		handle: Handle<CheckboxGroupItemProps<TVariant, TSize, TBreakpoint>>,
	): (
		props: CheckboxGroupItemProps<TVariant, TSize, TBreakpoint>,
	) => RemixNode {
		const context = handle.context.get(Root);

		return (
			props: CheckboxGroupItemProps<TVariant, TSize, TBreakpoint>,
		): RemixNode => {
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
			const checked = context.get_values().includes(value);
			const is_disabled = context.get_disabled() || disabled === true;
			const is_read_only = context.get_read_only() || readOnly === true;
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
					attrs: createComponentAnatomyAttrs(
						checkbox_group_scope,
						"item",
					),
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
									return;
								}
								context.set_item_checked(
									value,
									event.currentTarget.checked,
									{ event },
								);
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
						[componentDataAttribute.disabled]:
							dataFlag(is_disabled),
						[componentDataAttribute.readOnly]:
							dataFlag(is_read_only),
						[componentDataAttribute.required]: dataFlag(
							required === true,
						),
						checked: checkableInitialChecked(checked),
						disabled: is_disabled || undefined,
						form: form ?? context.get_form(),
						mix,
						name: context.get_name(),
						readOnly: is_read_only || undefined,
						required: required === true || undefined,
						type: checkbox_type,
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

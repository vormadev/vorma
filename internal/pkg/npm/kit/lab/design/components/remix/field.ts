import {
	createElement,
	type Handle,
	type Props,
	type RemixNode,
} from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
	type ComponentStyleTargetsOutput,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

const field_slot = {
	description: "description",
	error: "error",
	label: "label",
	root: "root",
} as const;

export type FieldRecipeSlot = (typeof field_slot)[keyof typeof field_slot];
export type FieldRecipeInput = RecipeWithVariantGroups<
	FieldRecipeSlot,
	string,
	ComponentStyle,
	Record<never, string>
>;

export type FieldStyleSystem<
	TMode extends string = string,
	TRecipe extends FieldRecipeInput = FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			field: TRecipe;
		};
	},
	TMetadata
>;

export type FieldProps = Omit<Props<"div">, "style"> & {
	disabled?: boolean;
	invalid?: boolean;
	style?: never;
};

export type FieldLabelProps = Omit<Props<"label">, "style"> & {
	style?: never;
};

export type FieldDescriptionProps = Omit<Props<"p">, "style"> & {
	style?: never;
};

export type FieldErrorProps = Omit<Props<"p">, "style"> & {
	match?: boolean;
	style?: never;
};

type FieldContext = {
	disabled: () => boolean;
	invalid: () => boolean;
};

export type FieldComponents = {
	Description: RemixComponent<FieldDescriptionProps>;
	Error: RemixComponent<FieldErrorProps>;
	Label: RemixComponent<FieldLabelProps>;
	Root: RemixComponent<FieldProps, FieldContext>;
};

const field_parts_by_system = new WeakMap<object, FieldComponents>();
const field_scope = "field";

function create_field_parts_for_system<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>): FieldComponents {
	const recipe = createRecipe(style_system.token.recipe.field);

	function create_parts(): ComponentStyleTargetsOutput<
		FieldRecipeSlot,
		FieldRecipeSlot
	> {
		const resolved = recipe.resolve();

		return createComponentStyleTargets({
			targets: {
				[field_slot.description]: {
					host: field_slot.description,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.description;
					},
				},
				[field_slot.error]: {
					host: field_slot.error,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.error;
					},
				},
				[field_slot.label]: {
					host: field_slot.label,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.label;
					},
				},
				[field_slot.root]: {
					host: field_slot.root,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.root;
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});
	}

	function Field(handle: Handle<FieldProps, FieldContext>) {
		const context = {
			disabled: (): boolean => {
				return Boolean(handle.props.disabled);
			},
			invalid: (): boolean => {
				return Boolean(handle.props.invalid);
			},
		};
		handle.context.set(context);

		return (props: FieldProps): RemixNode => {
			const { children, disabled, invalid, mix, ...host_props } = props;
			const parts = create_parts();

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						field_scope,
						field_slot.root,
					),
					mix: parts.hosts[field_slot.root].mix,
					props: {
						...host_props,
						"aria-disabled": disabled || undefined,
						"data-invalid": invalid || undefined,
						mix,
					},
				}),
				children,
			);
		};
	}

	function FieldLabel(handle: Handle<FieldLabelProps>) {
		const context = handle.context.get(Field);

		return (props: FieldLabelProps): RemixNode => {
			const { children, mix, ...host_props } = props;
			const parts = create_parts();

			return createElement(
				"label",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						field_scope,
						field_slot.label,
					),
					mix: parts.hosts[field_slot.label].mix,
					props: {
						...host_props,
						"aria-disabled": context.disabled() || undefined,
						"data-invalid": context.invalid() || undefined,
						mix,
					},
				}),
				children,
			);
		};
	}

	function FieldDescription(handle: Handle<FieldDescriptionProps>) {
		const context = handle.context.get(Field);

		return (props: FieldDescriptionProps): RemixNode => {
			const { children, mix, ...host_props } = props;
			const parts = create_parts();

			return createElement(
				"p",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						field_scope,
						field_slot.description,
					),
					mix: parts.hosts[field_slot.description].mix,
					props: {
						...host_props,
						"aria-disabled": context.disabled() || undefined,
						mix,
					},
				}),
				children,
			);
		};
	}

	function FieldError(handle: Handle<FieldErrorProps>) {
		const context = handle.context.get(Field);

		return (props: FieldErrorProps): RemixNode => {
			const { children, match = false, mix, ...host_props } = props;
			if (match && !context.invalid()) {
				return null;
			}

			const parts = create_parts();

			return createElement(
				"p",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						field_scope,
						field_slot.error,
					),
					mix: parts.hosts[field_slot.error].mix,
					props: {
						...host_props,
						"aria-disabled": context.disabled() || undefined,
						mix,
					},
				}),
				children,
			);
		};
	}

	return {
		Description: FieldDescription,
		Error: FieldError,
		Label: FieldLabel,
		Root: Field,
	};
}

export function createFieldParts<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>): FieldComponents {
	let parts = field_parts_by_system.get(style_system);
	if (!parts) {
		parts = create_field_parts_for_system(style_system);
		field_parts_by_system.set(style_system, parts);
	}
	return parts;
}

export function createField<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<FieldProps, FieldContext> {
	return createFieldParts(style_system).Root;
}

export function createFieldLabel<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<FieldLabelProps> {
	return createFieldParts(style_system).Label;
}

export function createFieldDescription<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<FieldDescriptionProps> {
	return createFieldParts(style_system).Description;
}

export function createFieldError<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<FieldErrorProps> {
	return createFieldParts(style_system).Error;
}

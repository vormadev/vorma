import {
	createElement,
	createMixin,
	type ElementProps,
	type Handle,
	type MixInput,
	type Props,
	type RemixNode,
} from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	ariaTrue,
	componentDataAttribute,
	dataFlag,
} from "./component-state.ts";
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

export const fieldAnatomy = {
	description: "description",
	error: "error",
	label: "label",
	root: "root",
} as const;

export type FieldRecipeSlot = (typeof fieldAnatomy)[keyof typeof fieldAnatomy];
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
	controlID?: string;
	disabled?: boolean;
	invalid?: boolean;
	readOnly?: boolean;
	required?: boolean;
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
	controlID: () => string;
	descriptionID: () => string;
	describedBy: () => string;
	disabled: () => boolean;
	errorID: () => string;
	invalid: () => boolean;
	readOnly: () => boolean;
	required: () => boolean;
};

export type FieldControlMixin = <
	TElement extends HTMLElement = HTMLElement,
>() => MixInput<TElement>;

export type FieldComponents = {
	controlMixin: FieldControlMixin;
	Description: RemixComponent<FieldDescriptionProps>;
	Error: RemixComponent<FieldErrorProps>;
	Label: RemixComponent<FieldLabelProps>;
	Root: RemixComponent<FieldProps, FieldContext>;
};

const field_parts_by_system = new WeakMap<object, FieldComponents>();
const field_scope = "field";

function merge_id_refs(
	own_id_refs: unknown,
	context_id_refs: string,
): string | undefined {
	if (typeof own_id_refs !== "string" || !own_id_refs.trim()) {
		return context_id_refs || undefined;
	}
	if (!context_id_refs) {
		return own_id_refs;
	}

	const ids = new Set([
		...own_id_refs.split(/\s+/).filter(Boolean),
		...context_id_refs.split(/\s+/).filter(Boolean),
	]);
	return Array.from(ids).join(" ");
}

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
				[fieldAnatomy.description]: {
					host: fieldAnatomy.description,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.description;
					},
				},
				[fieldAnatomy.error]: {
					host: fieldAnatomy.error,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.error;
					},
				},
				[fieldAnatomy.label]: {
					host: fieldAnatomy.label,
					conditions: commonConditions,
					resolveSlot: () => {
						return resolved.slots.label;
					},
				},
				[fieldAnatomy.root]: {
					host: fieldAnatomy.root,
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
			controlID: (): string => {
				return handle.props.controlID ?? `${handle.id}-control`;
			},
			descriptionID: (): string => {
				return `${handle.id}-description`;
			},
			describedBy: (): string => {
				return context.invalid()
					? `${context.descriptionID()} ${context.errorID()}`
					: context.descriptionID();
			},
			disabled: (): boolean => {
				return Boolean(handle.props.disabled);
			},
			errorID: (): string => {
				return `${handle.id}-error`;
			},
			invalid: (): boolean => {
				return Boolean(handle.props.invalid);
			},
			readOnly: (): boolean => {
				return Boolean(handle.props.readOnly);
			},
			required: (): boolean => {
				return Boolean(handle.props.required);
			},
		};
		handle.context.set(context);

		return (props: FieldProps): RemixNode => {
			const {
				children,
				controlID: _control_id,
				disabled,
				invalid,
				mix,
				readOnly,
				required,
				...host_props
			} = props;
			const parts = create_parts();

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						field_scope,
						fieldAnatomy.root,
					),
					mix: parts.hosts[fieldAnatomy.root].mix,
					props: {
						...host_props,
						"aria-disabled": ariaTrue(disabled === true),
						[componentDataAttribute.disabled]: dataFlag(
							disabled === true,
						),
						[componentDataAttribute.invalid]: dataFlag(
							invalid === true,
						),
						[componentDataAttribute.readOnly]: dataFlag(
							readOnly === true,
						),
						[componentDataAttribute.required]: dataFlag(
							required === true,
						),
						mix,
					},
				}),
				children,
			);
		};
	}

	const field_control_mixin = createMixin<HTMLElement, [], ElementProps>(
		(handle) => {
			const context = handle.context.get(Field);

			return (props) => {
				return createElement(handle.element, {
					...props,
					"aria-describedby": merge_id_refs(
						props["aria-describedby"],
						context.describedBy(),
					),
					"aria-invalid":
						props["aria-invalid"] ??
						(context.invalid() ? true : undefined),
					disabled: context.disabled() ? true : props.disabled,
					id: props.id ?? context.controlID(),
					readOnly: context.readOnly() ? true : props.readOnly,
					required: context.required() ? true : props.required,
				});
			};
		},
	);

	function control_mixin<
		TElement extends HTMLElement = HTMLElement,
	>(): MixInput<TElement> {
		return field_control_mixin() as MixInput<TElement>;
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
						fieldAnatomy.label,
					),
					mix: parts.hosts[fieldAnatomy.label].mix,
					props: {
						...host_props,
						"aria-disabled": ariaTrue(context.disabled()),
						[componentDataAttribute.disabled]: dataFlag(
							context.disabled(),
						),
						[componentDataAttribute.invalid]: dataFlag(
							context.invalid(),
						),
						[componentDataAttribute.readOnly]: dataFlag(
							context.readOnly(),
						),
						[componentDataAttribute.required]: dataFlag(
							context.required(),
						),
						htmlFor: host_props.htmlFor ?? context.controlID(),
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
						fieldAnatomy.description,
					),
					mix: parts.hosts[fieldAnatomy.description].mix,
					props: {
						...host_props,
						"aria-disabled": ariaTrue(context.disabled()),
						[componentDataAttribute.disabled]: dataFlag(
							context.disabled(),
						),
						[componentDataAttribute.invalid]: dataFlag(
							context.invalid(),
						),
						[componentDataAttribute.readOnly]: dataFlag(
							context.readOnly(),
						),
						[componentDataAttribute.required]: dataFlag(
							context.required(),
						),
						id: host_props.id ?? context.descriptionID(),
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
						fieldAnatomy.error,
					),
					mix: parts.hosts[fieldAnatomy.error].mix,
					props: {
						...host_props,
						"aria-disabled": ariaTrue(context.disabled()),
						[componentDataAttribute.disabled]: dataFlag(
							context.disabled(),
						),
						[componentDataAttribute.invalid]: dataFlag(
							context.invalid(),
						),
						[componentDataAttribute.readOnly]: dataFlag(
							context.readOnly(),
						),
						[componentDataAttribute.required]: dataFlag(
							context.required(),
						),
						id: host_props.id ?? context.errorID(),
						mix,
					},
				}),
				children,
			);
		};
	}

	return {
		controlMixin: control_mixin,
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

export function createFieldControlMixin<
	TMode extends string,
	TRecipe extends FieldRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: FieldStyleSystem<TMode, TRecipe, TMetadata>,
): FieldControlMixin {
	return createFieldParts(style_system).controlMixin;
}

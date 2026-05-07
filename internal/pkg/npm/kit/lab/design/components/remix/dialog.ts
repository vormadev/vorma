import {
	createElement,
	createMixin,
	on,
	type ElementProps,
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
	type ResolvedRecipeSlot,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { type RecipeConditionSelectorMap } from "./recipe.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type DialogRequiredRecipeSlot =
	| "close"
	| "description"
	| "popup"
	| "title"
	| "trigger";

export type DialogRecipeSlot = DialogRequiredRecipeSlot | "overlay";

export type DialogRecipeCondition =
	| "active"
	| "disabled"
	| "focusVisible"
	| "hover"
	| "modal"
	| "open"
	| "reducedMotion";

export type DialogRecipeInput<TPopupLayout extends string = string> =
	RecipeWithVariantGroups<
		DialogRequiredRecipeSlot,
		string,
		ComponentStyle,
		{
			popupLayout: TPopupLayout;
		}
	>;

export type DialogRecipePopupLayout<TRecipe extends DialogRecipeInput> =
	RecipeVariantValue<TRecipe, "popupLayout">;
export type DialogRecipeSelection<TRecipe extends DialogRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "popupLayout">;

export type DialogStyleSystem<
	TMode extends string = string,
	TRecipe extends DialogRecipeInput = DialogRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { dialog: TRecipe } }, TMetadata>;

export type DialogProps = {
	children?: RemixNode;
	closeOnOutsideClick?: boolean;
	defaultOpen?: boolean;
	modal?: boolean;
	onOpenChange?: (open: boolean) => void;
	onOpenChangeComplete?: (open: boolean) => void;
	open?: boolean;
};

export type DialogTriggerProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type DialogPopupStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type DialogPopupProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"dialog">, "style"> &
	DialogPopupStyleProps<TLayout> &
	ResponsiveProps<DialogPopupStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type DialogTitleProps = Omit<Props<"h2">, "style"> & {
	style?: never;
};

export type DialogDescriptionProps = Omit<Props<"p">, "style"> & {
	style?: never;
};

export type DialogCloseProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type DialogComponents<TPopupLayout extends string = string> = {
	Close: RemixComponent<DialogCloseProps>;
	Description: RemixComponent<DialogDescriptionProps>;
	Popup: RemixComponent<DialogPopupProps<TPopupLayout>>;
	Root: RemixComponent<DialogProps, DialogRuntimeContext>;
	Title: RemixComponent<DialogTitleProps>;
	Trigger: RemixComponent<DialogTriggerProps>;
};

type DialogRuntimeContext = {
	closeOnOutsideClick: () => boolean;
	isModal: () => boolean;
	isOpen: () => boolean;
	setOpen: (open: boolean) => void;
};

const action_conditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<DialogRecipeCondition>;

const popup_conditions = {
	modal: "&[data-modal='true']",
	open: "&[open]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<DialogRecipeCondition>;

const dialog_scope = "dialog";
const dialog_backdrop_selector = "&::backdrop";
const click_event = "click";
const close_event = "close";
const cancel_event = "cancel";
const button_type_default = "button";
const empty_dialog_overlay_slot = {
	base: {},
	conditions: {},
} as const satisfies ResolvedRecipeSlot<DialogRecipeCondition, ComponentStyle>;

const native_dialog = createMixin<
	HTMLDialogElement,
	[
		options: {
			modal: boolean;
			onOpenChange: (open: boolean) => void;
			open: boolean;
		},
	],
	ElementProps
>((handle) => {
	let dialog: HTMLDialogElement | undefined;
	let options: {
		modal: boolean;
		onOpenChange: (open: boolean) => void;
		open: boolean;
	} = {
		modal: true,
		onOpenChange: () => {},
		open: false,
	};

	function sync(): void {
		if (!dialog) {
			return;
		}

		if (options.open && !dialog.open) {
			if (options.modal) {
				dialog.showModal();
			} else {
				dialog.show();
			}
			return;
		}

		if (!options.open && dialog.open) {
			dialog.close();
		}
	}

	handle.addEventListener("insert", (event) => {
		dialog = event.node;
		sync();
	});
	handle.addEventListener("commit", () => {
		sync();
	});
	handle.addEventListener("remove", () => {
		dialog = undefined;
	});

	return (nextOptions) => {
		options = nextOptions;
		return handle.element;
	};
});

function is_dialog_outside_click(event: MouseEvent): boolean {
	const dialog = event.currentTarget;
	if (!(dialog instanceof HTMLDialogElement)) {
		return false;
	}

	const rect = dialog.getBoundingClientRect();
	return (
		event.clientX < rect.left ||
		event.clientX > rect.right ||
		event.clientY < rect.top ||
		event.clientY > rect.bottom
	);
}

export function createDialog<
	TMode extends string,
	TRecipe extends DialogRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: DialogStyleSystem<TMode, TRecipe, TMetadata>,
): DialogComponents<DialogRecipePopupLayout<TRecipe>> {
	type TPopupLayout = DialogRecipePopupLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const dialog_recipe = createRecipe(style_system.token.recipe.dialog);

	function Root(
		handle: Handle<DialogProps, DialogRuntimeContext>,
	): (props: DialogProps) => RemixNode {
		let uncontrolled_open = handle.props.defaultOpen ?? false;
		let requested_open = handle.props.open ?? uncontrolled_open;
		const context = {
			closeOnOutsideClick: (): boolean => {
				return handle.props.closeOnOutsideClick ?? true;
			},
			isModal: (): boolean => {
				return handle.props.modal ?? true;
			},
			isOpen: (): boolean => {
				return handle.props.open ?? uncontrolled_open;
			},
			setOpen: (open: boolean): void => {
				if (requested_open === open) {
					return;
				}
				requested_open = open;
				if (handle.props.open === undefined) {
					uncontrolled_open = open;
				}
				handle.props.onOpenChange?.(open);
				handle.props.onOpenChangeComplete?.(open);
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: DialogProps): RemixNode => {
			requested_open = props.open ?? uncontrolled_open;
			return props.children;
		};
	}

	function Trigger(
		handle: Handle<DialogTriggerProps>,
	): (props: DialogTriggerProps) => RemixNode {
		const context = handle.context.get(Root);
		const resolved = dialog_recipe.resolve();
		const parts = createComponentStyleTargets({
			targets: {
				trigger: {
					host: "trigger",
					conditions: action_conditions,
					resolveSlot: () => {
						return resolved.slots.trigger;
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});

		return (props: DialogTriggerProps): RemixNode => {
			const {
				children,
				mix,
				type = button_type_default,
				...trigger_props
			} = props;
			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(dialog_scope, "trigger"),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							() => {
								context.setOpen(true);
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-expanded": context.isOpen(),
						mix,
						type,
					},
				}),
				children,
			);
		};
	}

	function Popup(
		handle: Handle<DialogPopupProps<TPopupLayout, TBreakpoint>>,
	): (props: DialogPopupProps<TPopupLayout, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		function resolve_slot(
			props: Partial<DialogPopupStyleProps<TPopupLayout>>,
		): ReturnType<typeof dialog_recipe.resolve>["slots"]["popup"] {
			return dialog_recipe.resolve({
				popupLayout: props.layout,
			} satisfies DialogRecipeSelection<TRecipe>).slots.popup;
		}

		function resolve_overlay_slot(
			props: Partial<DialogPopupStyleProps<TPopupLayout>>,
		): ResolvedRecipeSlot<DialogRecipeCondition, ComponentStyle> {
			const resolved = dialog_recipe.resolve({
				popupLayout: props.layout,
			} satisfies DialogRecipeSelection<TRecipe>);
			const slots = resolved.slots as Partial<
				Record<
					DialogRecipeSlot,
					ResolvedRecipeSlot<DialogRecipeCondition, ComponentStyle>
				>
			>;

			return slots.overlay ?? empty_dialog_overlay_slot;
		}

		return (
			props: DialogPopupProps<TPopupLayout, TBreakpoint>,
		): RemixNode => {
			const { at, children, layout, mix, ...popup_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					overlay: {
						host: "popup",
						conditions: popup_conditions,
						resolveSlot: resolve_overlay_slot,
						selectors: [dialog_backdrop_selector],
					},
					popup: {
						host: "popup",
						conditions: popup_conditions,
						resolveSlot: resolve_slot,
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"dialog",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(dialog_scope, "popup"),
					mix: [
						native_dialog({
							modal: context.isModal(),
							onOpenChange: context.setOpen,
							open: context.isOpen(),
						}),
						on<HTMLDialogElement, typeof close_event>(
							close_event,
							() => {
								context.setOpen(false);
							},
						),
						on<HTMLDialogElement, typeof cancel_event>(
							cancel_event,
							() => {
								context.setOpen(false);
							},
						),
						on<HTMLDialogElement, typeof click_event>(
							click_event,
							(event) => {
								if (
									context.closeOnOutsideClick() &&
									is_dialog_outside_click(event)
								) {
									context.setOpen(false);
								}
							},
						),
						parts.hosts.popup.mix,
					],
					props: {
						...popup_props,
						"data-modal": context.isModal() ? "true" : undefined,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Title(): (props: DialogTitleProps) => RemixNode {
		const resolved = dialog_recipe.resolve();
		const parts = createComponentStyleTargets({
			targets: {
				title: {
					host: "title",
					resolveSlot: () => {
						return resolved.slots.title;
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});
		return (props: DialogTitleProps): RemixNode => {
			const { children, mix, ...title_props } = props;
			return createElement(
				"h2",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(dialog_scope, "title"),
					mix: parts.hosts.title.mix,
					props: {
						...title_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Description(): (props: DialogDescriptionProps) => RemixNode {
		const resolved = dialog_recipe.resolve();
		const parts = createComponentStyleTargets({
			targets: {
				description: {
					host: "description",
					resolveSlot: () => {
						return resolved.slots.description;
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});
		return (props: DialogDescriptionProps): RemixNode => {
			const { children, mix, ...description_props } = props;
			return createElement(
				"p",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						dialog_scope,
						"description",
					),
					mix: parts.hosts.description.mix,
					props: {
						...description_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Close(
		handle: Handle<DialogCloseProps>,
	): (props: DialogCloseProps) => RemixNode {
		const context = handle.context.get(Root);
		const resolved = dialog_recipe.resolve();
		const parts = createComponentStyleTargets({
			targets: {
				close: {
					host: "close",
					conditions: action_conditions,
					resolveSlot: () => {
						return resolved.slots.close;
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});

		return (props: DialogCloseProps): RemixNode => {
			const {
				children,
				mix,
				type = button_type_default,
				...close_props
			} = props;
			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(dialog_scope, "close"),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							() => {
								context.setOpen(false);
							},
						),
						parts.hosts.close.mix,
					],
					props: {
						...close_props,
						mix,
						type,
					},
				}),
				children,
			);
		};
	}

	return {
		Close,
		Description,
		Popup,
		Root,
		Title,
		Trigger,
	};
}

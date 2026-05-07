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
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import {
	componentStateAttribute,
	openState,
	openStateFromBoolean,
} from "./component-state.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
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

export type AlertDialogRecipeCondition =
	| CommonRecipeCondition
	| "closed"
	| "open";

export type AlertDialogRecipeInput<TLayout extends string = string> =
	RecipeWithVariantGroups<
		| "action"
		| "cancel"
		| "description"
		| "overlay"
		| "popup"
		| "title"
		| "trigger",
		string,
		ComponentStyle,
		{
			layout: TLayout;
		}
	>;

export type AlertDialogRecipeLayout<TRecipe extends AlertDialogRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type AlertDialogRecipeSelection<
	TRecipe extends AlertDialogRecipeInput,
> = RecipeVariantPropsFor<TRecipe, "layout">;

export type AlertDialogStyleSystem<
	TMode extends string = string,
	TRecipe extends AlertDialogRecipeInput = AlertDialogRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{ recipe: { alertDialog: TRecipe } },
	TMetadata
>;

export type AlertDialogOpenChangeReason =
	| "action"
	| "cancel"
	| "escape"
	| "trigger";

export type AlertDialogOpenChangeDetails = {
	event?: Event;
	reason: AlertDialogOpenChangeReason;
};

export type AlertDialogOpenChangeHandler = (
	open: boolean,
	details?: AlertDialogOpenChangeDetails,
) => void;

export type AlertDialogRootProps = {
	children?: RemixNode;
	defaultOpen?: boolean;
	onOpenChange?: AlertDialogOpenChangeHandler;
	onOpenChangeComplete?: AlertDialogOpenChangeHandler;
	open?: boolean;
};

export type AlertDialogPopupStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type AlertDialogTriggerProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type AlertDialogPopupProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"dialog">, "style"> &
	AlertDialogPopupStyleProps<TLayout> &
	ResponsiveProps<AlertDialogPopupStyleProps<TLayout>, TBreakpoint> & {
		style?: never;
	};

export type AlertDialogTitleProps = Omit<Props<"h2">, "style"> & {
	style?: never;
};

export type AlertDialogDescriptionProps = Omit<Props<"p">, "style"> & {
	style?: never;
};

export type AlertDialogActionProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type AlertDialogCancelProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type AlertDialogComponents<TLayout extends string = string> = {
	Action: RemixComponent<AlertDialogActionProps>;
	Cancel: RemixComponent<AlertDialogCancelProps>;
	Description: RemixComponent<AlertDialogDescriptionProps>;
	Popup: RemixComponent<AlertDialogPopupProps<TLayout>>;
	Root: RemixComponent<AlertDialogRootProps, AlertDialogContext>;
	Title: RemixComponent<AlertDialogTitleProps>;
	Trigger: RemixComponent<AlertDialogTriggerProps>;
};

type AlertDialogContext = {
	get_description_id: () => string;
	get_open: () => boolean;
	get_title_id: () => string;
	set_open: (
		open: boolean,
		details?: AlertDialogOpenChangeDetails,
	) => void;
};

const alert_dialog_scope = "alertDialog";
const alert_dialog_role = "alertdialog";
const button_type_default = "button";
const cancel_event = "cancel";
const click_event = "click";
const close_event = "close";
const backdrop_selector = "&::backdrop";

const alert_dialog_conditions =
	mergeRecipeConditionSelectors<AlertDialogRecipeCondition>(
		commonConditions,
		{
			closed: `&[${componentStateAttribute}='${openState.closed}']`,
			open: `&[${componentStateAttribute}='${openState.open}'], &[open]`,
		} satisfies RecipeConditionSelectorMap<AlertDialogRecipeCondition>,
	);

const native_alert_dialog = createMixin<
	HTMLDialogElement,
	[
		options: {
			open: boolean;
		},
	],
	ElementProps
>((handle) => {
	let dialog: HTMLDialogElement | undefined;
	let options = {
		open: false,
	};

	function sync(): void {
		if (!dialog) {
			return;
		}
		if (options.open && !dialog.open) {
			dialog.showModal();
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

	return (next_options) => {
		options = next_options;
		return handle.element;
	};
});

export function createAlertDialog<
	TMode extends string,
	TRecipe extends AlertDialogRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: AlertDialogStyleSystem<TMode, TRecipe, TMetadata>,
): AlertDialogComponents<AlertDialogRecipeLayout<TRecipe>> {
	type TLayout = AlertDialogRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.alertDialog);

	function resolve_slot(
		slot:
			| "action"
			| "cancel"
			| "description"
			| "overlay"
			| "popup"
			| "title"
			| "trigger",
		props: Partial<AlertDialogPopupStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies AlertDialogRecipeSelection<TRecipe>).slots[slot];
	}

	function Root(
		handle: Handle<AlertDialogRootProps, AlertDialogContext>,
	): (props: AlertDialogRootProps) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: AlertDialogOpenChangeDetails | undefined;

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		const context: AlertDialogContext = {
			get_description_id: () => {
				return `${handle.id}-description`;
			},
			get_open,
			get_title_id: () => {
				return `${handle.id}-title`;
			},
			set_open: (open, details) => {
				if (get_open() === open) {
					return;
				}
				last_open_details = details;
				if (handle.props.open === undefined) {
					local_open = open;
				}
				handle.props.onOpenChange?.(open, details);
				void handle.update();
			},
		};
		handle.context.set(context);

		return (props: AlertDialogRootProps): RemixNode => {
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
		handle: Handle<AlertDialogTriggerProps>,
	): (props: AlertDialogTriggerProps) => RemixNode {
		const context = handle.context.get(Root);
		return (props: AlertDialogTriggerProps): RemixNode => {
			const {
				children,
				mix,
				type = button_type_default,
				...trigger_props
			} = props;
			const parts = createComponentStyleTargets({
				targets: {
					trigger: {
						host: "trigger",
						conditions: commonConditions,
						resolveSlot: () => {
							return resolve_slot("trigger", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						alert_dialog_scope,
						"trigger",
					),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								context.set_open(true, {
									event,
									reason: "trigger",
								});
							},
						),
						parts.hosts.trigger.mix,
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
		handle: Handle<AlertDialogPopupProps<TLayout, TBreakpoint>>,
	): (props: AlertDialogPopupProps<TLayout, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);
		return (
			props: AlertDialogPopupProps<TLayout, TBreakpoint>,
		): RemixNode => {
			const { at, children, layout, mix, ...popup_props } = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					popup: "dialog",
				},
				targets: {
					overlay: {
						host: "popup",
						conditions: alert_dialog_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("overlay", current_props);
						},
						selectors: [backdrop_selector],
					},
					popup: {
						host: "popup",
						conditions: alert_dialog_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("popup", current_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});
			return createElement(
				"dialog",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						alert_dialog_scope,
						"popup",
					),
					mix: [
						native_alert_dialog({ open }),
						on<HTMLDialogElement, typeof close_event>(
							close_event,
							(event) => {
								context.set_open(false, {
									event,
									reason: "cancel",
								});
							},
						),
						on<HTMLDialogElement, typeof cancel_event>(
							cancel_event,
							(event) => {
								context.set_open(false, {
									event,
									reason: "escape",
								});
							},
						),
						parts.hosts.popup.mix,
					],
					props: {
						...popup_props,
						"aria-describedby":
							popup_props["aria-describedby"] ??
							context.get_description_id(),
						"aria-labelledby":
							popup_props["aria-labelledby"] ??
							context.get_title_id(),
						[componentStateAttribute]: openStateFromBoolean(open),
						mix,
						role: popup_props.role ?? alert_dialog_role,
					},
				}),
				children,
			);
		};
	}

	function create_button_part(
		slot: "action" | "cancel",
		reason: AlertDialogOpenChangeReason,
	): RemixComponent<AlertDialogActionProps | AlertDialogCancelProps> {
		return (handle: Handle<AlertDialogActionProps>) => {
			const context = handle.context.get(Root);
			return (props: AlertDialogActionProps): RemixNode => {
				const {
					children,
					mix,
					type = button_type_default,
					...button_props
				} = props;
				const parts = createComponentStyleTargets({
					targets: {
						[slot]: {
							host: slot,
							conditions: commonConditions,
							resolveSlot: () => {
								return resolve_slot(slot, {});
							},
						},
					} as Record<
						typeof slot,
						{
							conditions: typeof commonConditions;
							host: typeof slot;
							resolveSlot: () => ReturnType<
								typeof recipe.resolve
							>["slots"][typeof slot];
						}
					>,
					props: {},
					styleSystem: style_system,
				});
				return createElement(
					"button",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							alert_dialog_scope,
							slot,
						),
						mix: [
							on<HTMLButtonElement, typeof click_event>(
								click_event,
								(event) => {
									context.set_open(false, {
										event,
										reason,
									});
								},
							),
							parts.hosts[slot].mix,
						],
						props: {
							...button_props,
							mix,
							type,
						},
					}),
					children,
				);
			};
		};
	}

	function create_text_part<TElement extends "h2" | "p">(
		element: TElement,
		slot: "description" | "title",
	): RemixComponent<Omit<Props<TElement>, "style"> & { style?: never }> {
		return (handle: Handle<Omit<Props<TElement>, "style">>) => {
			const context = handle.context.get(Root);
			return (
				props: Omit<Props<TElement>, "style"> & { style?: never },
			): RemixNode => {
				const { children, mix, ...text_props } = props;
				const parts = createComponentStyleTargets({
					targets: {
						[slot]: {
							host: slot,
							conditions: commonConditions,
							resolveSlot: () => {
								return resolve_slot(slot, {});
							},
						},
					} as Record<
						typeof slot,
						{
							conditions: typeof commonConditions;
							host: typeof slot;
							resolveSlot: () => ReturnType<
								typeof recipe.resolve
							>["slots"][typeof slot];
						}
					>,
					props: {},
					styleSystem: style_system,
				});
				return createElement(
					element,
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							alert_dialog_scope,
							slot,
						),
						mix: parts.hosts[slot].mix,
						props: {
							...text_props,
							id:
								text_props.id ??
								(slot === "title"
									? context.get_title_id()
									: context.get_description_id()),
							mix,
						},
					}),
					children,
				);
			};
		};
	}

	return {
		Action: create_button_part("action", "action"),
		Cancel: create_button_part("cancel", "cancel"),
		Description: create_text_part("p", "description"),
		Popup,
		Root,
		Title: create_text_part("h2", "title"),
		Trigger,
	};
}

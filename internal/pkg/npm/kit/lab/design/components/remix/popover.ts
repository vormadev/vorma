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
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
import {
	popoverScrollLockGuard,
	releasePopoverScrollLock,
} from "./popover-scroll-lock.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export const popoverAnatomy = {
	parts: {
		arrow: "arrow",
		close: "close",
		description: "description",
		popup: "popup",
		title: "title",
		trigger: "trigger",
	},
	scope: "popover",
} as const;

export type PopoverRecipeSlot =
	(typeof popoverAnatomy.parts)[keyof typeof popoverAnatomy.parts];
export type PopoverRecipeCondition = string;

export type PopoverRecipeInput<TSize extends string = string> =
	RecipeWithVariantGroups<
		PopoverRecipeSlot,
		string,
		ComponentStyle,
		{
			size: TSize;
		}
	>;

export type PopoverRecipeSize<TRecipe extends PopoverRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type PopoverStyleSystem<
	TMode extends string = string,
	TRecipe extends PopoverRecipeInput = PopoverRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<
	TMode,
	{
		recipe: {
			popover: TRecipe;
		};
	},
	TMetadata
>;

export type PopoverOpenChangeReason =
	| "close"
	| "escape"
	| "interactOutside"
	| "trigger";

export type PopoverOpenChangeDetails = {
	event?: Event;
	reason: PopoverOpenChangeReason;
};

export type PopoverOpenChangeHandler = (
	open: boolean,
	details?: PopoverOpenChangeDetails,
) => void;

export type PopoverRootProps = {
	children?: RemixNode;
	closeOnEscape?: boolean;
	closeOnInteractOutside?: boolean;
	defaultOpen?: boolean;
	modal?: boolean;
	open?: boolean;
	onOpenChange?: PopoverOpenChangeHandler;
	onOpenChangeComplete?: PopoverOpenChangeHandler;
	shouldCloseOnInteractOutside?: (element: Element) => boolean;
};

export type PopoverTriggerProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type PopoverSide = "bottom" | "left" | "right" | "top";
export type PopoverAlign = "center" | "end" | "start";

export type PopoverPopupStyleProps<TSize extends string = string> = {
	size?: TSize;
};

export type PopoverPopupProps<TSize extends string = string> = Omit<
	Props<"div">,
	"style"
> &
	PopoverPopupStyleProps<TSize> & {
		align?: PopoverAlign;
		alignOffset?: number;
		arrowPadding?: number;
		avoidCollisions?: boolean;
		collisionPadding?: number;
		side?: PopoverSide;
		sideOffset?: number;
		style?: never;
	};

export type PopoverArrowProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type PopoverTitleProps = Omit<Props<"h2">, "style"> & {
	style?: never;
};

export type PopoverDescriptionProps = Omit<Props<"p">, "style"> & {
	style?: never;
};

export type PopoverCloseProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type PopoverComponents<TSize extends string = string> = {
	Arrow: RemixComponent<PopoverArrowProps>;
	Close: RemixComponent<PopoverCloseProps>;
	Description: RemixComponent<PopoverDescriptionProps>;
	Popup: RemixComponent<PopoverPopupProps<TSize>>;
	Root: RemixComponent<PopoverRootProps, PopoverRuntimeContext>;
	Title: RemixComponent<PopoverTitleProps>;
	Trigger: RemixComponent<PopoverTriggerProps>;
};

export type PopoverConditionSelectors = Partial<
	Readonly<Record<PopoverRecipeSlot, RecipeConditionSelectorMap<string>>>
>;

export type PopoverOptions = {
	conditions?: PopoverConditionSelectors;
};

type PopoverRuntimeContext = {
	close: (details: PopoverOpenChangeDetails) => void;
	contains_target: (target: EventTarget | null) => boolean;
	get_close_on_escape: () => boolean;
	get_close_on_interact_outside: () => boolean;
	get_description_id: () => string;
	get_open: () => boolean;
	get_popup_id: () => string;
	get_should_close_on_interact_outside: () =>
		| ((element: Element) => boolean)
		| undefined;
	get_title_id: () => string;
	get_trigger_id: () => string;
	open: (details: PopoverOpenChangeDetails) => void;
	register_popup: (node: HTMLElement | null) => void;
	register_trigger: (node: HTMLElement | null) => void;
	sync_popup: () => void;
	toggle: (details: PopoverOpenChangeDetails) => void;
};

const popover_trigger_conditions = mergeRecipeConditionSelectors<string>(
	commonConditions,
	{
		closed: "&[aria-expanded='false']",
		open: "&[aria-expanded='true']",
	},
);

const popover_popup_conditions = {
	closed: "&:not(:popover-open), &:not([data-open])",
	open: "&:popover-open, &[data-open]",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
} as const satisfies RecipeConditionSelectorMap<string>;

const popover_static_conditions = commonConditions;

const button_type_default = "button";
const popover_align_default = "start" satisfies PopoverAlign;
const popover_side_default = "bottom" satisfies PopoverSide;

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

export function createPopover<
	TMode extends string,
	TRecipe extends PopoverRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: PopoverStyleSystem<TMode, TRecipe, TMetadata>,
	options: PopoverOptions = {},
): PopoverComponents<PopoverRecipeSize<TRecipe>> {
	type TSize = PopoverRecipeSize<TRecipe>;

	const popover_recipe = createRecipe(style_system.token.recipe.popover);

	function Root(
		handle: Handle<PopoverRootProps, PopoverRuntimeContext>,
	): (props: PopoverRootProps) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: PopoverOpenChangeDetails = {
			reason: "trigger",
		};
		let popup_node: HTMLElement | null = null;
		let trigger_node: HTMLElement | null = null;

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		function request_open(
			open: boolean,
			details: PopoverOpenChangeDetails,
		): void {
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

		const context: PopoverRuntimeContext = {
			close: (details) => {
				request_open(false, details);
			},
			contains_target: (target) => {
				return (
					contains_event_target(trigger_node, target) ||
					contains_event_target(popup_node, target)
				);
			},
			get_close_on_escape: () => {
				return handle.props.closeOnEscape !== false;
			},
			get_close_on_interact_outside: () => {
				return handle.props.closeOnInteractOutside !== false;
			},
			get_description_id: () => {
				return `${handle.id}-description`;
			},
			get_open,
			get_popup_id: () => {
				return `${handle.id}-popup`;
			},
			get_should_close_on_interact_outside: () => {
				return handle.props.shouldCloseOnInteractOutside;
			},
			get_title_id: () => {
				return `${handle.id}-title`;
			},
			get_trigger_id: () => {
				return `${handle.id}-trigger`;
			},
			open: (details) => {
				request_open(true, details);
			},
			register_popup: (node) => {
				popup_node = node;
			},
			register_trigger: (node) => {
				trigger_node = node;
			},
			sync_popup: () => {
				if (popup_node) {
					sync_native_popover(popup_node, get_open());
				}
				if (!get_open()) {
					trigger_node?.focus();
				}
			},
			toggle: (details) => {
				request_open(!get_open(), details);
			},
		};
		handle.context.set(context);

		return (props: PopoverRootProps): RemixNode => {
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

			return createElement(Fragment, {}, props.children);
		};
	}

	function Trigger(
		handle: Handle<PopoverTriggerProps>,
	): (props: PopoverTriggerProps) => RemixNode {
		const context = handle.context.get(Root);
		const resolved = popover_recipe.resolve();
		const parts = createComponentStyleTargets({
			hostElements: {
				trigger: "button",
			},
			targets: {
				trigger: {
					host: "trigger",
					conditions: mergeRecipeConditionSelectors(
						popover_trigger_conditions,
						options.conditions?.trigger,
					),
					resolveSlot: () => {
						return resolved.slots[popoverAnatomy.parts.trigger];
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});

		return (props: PopoverTriggerProps): RemixNode => {
			const {
				children,
				mix,
				type = button_type_default,
				...trigger_props
			} = props;
			const is_open = context.get_open();
			const trigger_ref_mix = ref<HTMLButtonElement>((node, signal) => {
				context.register_trigger(node);
				signal.addEventListener("abort", () => {
					context.register_trigger(null);
				});
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
						event.key !== "Enter" &&
						event.key !== " "
					) {
						return;
					}
					event.preventDefault();
					context.open({ event, reason: "trigger" });
				},
			);

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						popoverAnatomy.scope,
						popoverAnatomy.parts.trigger,
					),
					mix: [
						trigger_ref_mix,
						trigger_click_mix,
						trigger_keydown_mix,
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-controls": context.get_popup_id(),
						"aria-expanded": is_open ? "true" : "false",
						"data-open": is_open ? "" : undefined,
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
		handle: Handle<PopoverPopupProps<TSize>>,
	): (props: PopoverPopupProps<TSize>) => RemixNode {
		const context = handle.context.get(Root);

		return (props: PopoverPopupProps<TSize>): RemixNode => {
			const {
				align = popover_align_default,
				alignOffset,
				arrowPadding,
				avoidCollisions,
				children,
				collisionPadding,
				mix,
				side = popover_side_default,
				sideOffset,
				size,
				...popup_props
			} = props;
			const is_open = context.get_open();
			const parts = createComponentStyleTargets({
				targets: {
					popup: {
						host: "popup",
						conditions: mergeRecipeConditionSelectors(
							popover_popup_conditions,
							options.conditions?.popup,
						),
						resolveSlot: (style_props) => {
							return popover_recipe.resolve({
								size: style_props.size,
							}).slots[popoverAnatomy.parts.popup];
						},
					},
				},
				props: { size },
				styleSystem: style_system,
			});
			const popup_ref_mix = ref<HTMLElement>((node, signal) => {
				context.register_popup(node);
				sync_native_popover(node, context.get_open());
				const on_pointer_down = (event: PointerEvent): void => {
					if (
						!context.get_open() ||
						context.contains_target(event.target)
					) {
						return;
					}
					if (!context.get_close_on_interact_outside()) {
						return;
					}
					const should_close =
						event.target instanceof Element
							? context.get_should_close_on_interact_outside()?.(
									event.target,
								)
							: undefined;
					if (should_close === false) {
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
			const popup_keydown_mix = on<HTMLElement, "keydown">(
				"keydown",
				(event) => {
					if (
						event.key !== "Escape" ||
						!context.get_close_on_escape()
					) {
						return;
					}
					event.preventDefault();
					context.close({ event, reason: "escape" });
				},
			);

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
						popoverAnatomy.scope,
						popoverAnatomy.parts.popup,
					),
					mix: [
						popoverScrollLockGuard(),
						popup_ref_mix,
						popup_keydown_mix,
						parts.hosts.popup.mix,
					],
					props: {
						...popup_props,
						"aria-describedby": context.get_description_id(),
						"aria-labelledby": context.get_title_id(),
						"data-align": align,
						"data-avoid-collisions": avoidCollisions,
						"data-open": is_open ? "" : undefined,
						"data-side": side,
						"data-vorma-align-offset": alignOffset,
						"data-vorma-arrow-padding": arrowPadding,
						"data-vorma-collision-padding": collisionPadding,
						"data-vorma-side-offset": sideOffset,
						hidden: !is_open,
						id: context.get_popup_id(),
						mix,
						popover: "manual",
						role: "dialog",
					},
				}),
				children,
			);
		};
	}

	function create_static_component<TProps extends { mix?: unknown }>(
		slot: PopoverRecipeSlot,
		element: "button" | "div" | "h2" | "p",
		extra_props?: Record<string, unknown>,
	): RemixComponent<TProps> {
		return (handle: Handle<TProps>) => {
			const context = handle.context.get(Root);

			return (props: TProps): RemixNode => {
				const { children, mix, ...rest_props } = props as TProps & {
					children?: RemixNode;
					mix?: unknown;
				};
				const parts = createComponentStyleTargets({
					targets: {
						[slot]: {
							host: slot,
							conditions: mergeRecipeConditionSelectors(
								popover_static_conditions,
								options.conditions?.[slot],
							),
							resolveSlot: () => {
								return popover_recipe.resolve().slots[slot];
							},
						},
					},
					props: {},
					styleSystem: style_system,
				});
				const close_mix =
					slot === popoverAnatomy.parts.close
						? on<HTMLButtonElement, "click">("click", (event) => {
								context.close({ event, reason: "close" });
							})
						: undefined;

				return createElement(
					element,
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							popoverAnatomy.scope,
							slot,
						),
						mix: [close_mix, parts.hosts[slot].mix],
						props: {
							...extra_props,
							...rest_props,
							id:
								slot === popoverAnatomy.parts.title
									? context.get_title_id()
									: slot === popoverAnatomy.parts.description
										? context.get_description_id()
										: (rest_props as { id?: string }).id,
							mix,
						},
					}),
					children,
				);
			};
		};
	}

	return {
		Arrow: create_static_component<PopoverArrowProps>(
			popoverAnatomy.parts.arrow,
			"div",
			{ "aria-hidden": "true" },
		),
		Close: create_static_component<PopoverCloseProps>(
			popoverAnatomy.parts.close,
			"button",
			{ type: button_type_default },
		),
		Description: create_static_component<PopoverDescriptionProps>(
			popoverAnatomy.parts.description,
			"p",
		),
		Popup,
		Root,
		Title: create_static_component<PopoverTitleProps>(
			popoverAnatomy.parts.title,
			"h2",
		),
		Trigger,
	};
}

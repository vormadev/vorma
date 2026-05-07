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

export type TooltipRecipeCondition =
	| CommonRecipeCondition
	| "closed"
	| "open";

export type TooltipRecipeInput<
	TSize extends string = string,
	TVariant extends string = string,
> = RecipeWithVariantGroups<
	"popup" | "trigger",
	string,
	ComponentStyle,
	{
		size: TSize;
		variant: TVariant;
	}
>;

export type TooltipRecipeSize<TRecipe extends TooltipRecipeInput> =
	RecipeVariantValue<TRecipe, "size">;

export type TooltipRecipeVariant<TRecipe extends TooltipRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type TooltipRecipeSelection<TRecipe extends TooltipRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "size" | "variant">;

export type TooltipStyleSystem<
	TMode extends string = string,
	TRecipe extends TooltipRecipeInput = TooltipRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { tooltip: TRecipe } }, TMetadata>;

export type TooltipOpenChangeDetails = {
	event?: Event;
	reason: "blur" | "focus" | "pointerEnter" | "pointerLeave";
};

export type TooltipOpenChangeHandler = (
	open: boolean,
	details?: TooltipOpenChangeDetails,
) => void;

export type TooltipRootProps = {
	children?: RemixNode;
	defaultOpen?: boolean;
	onOpenChange?: TooltipOpenChangeHandler;
	open?: boolean;
};

export type TooltipStyleProps<
	TSize extends string = string,
	TVariant extends string = string,
> = {
	size?: TSize;
	variant?: TVariant;
};

export type TooltipTriggerProps<
	TSize extends string = string,
	TVariant extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"button">, "style"> &
	TooltipStyleProps<TSize, TVariant> &
	ResponsiveProps<TooltipStyleProps<TSize, TVariant>, TBreakpoint> & {
		style?: never;
	};

export type TooltipPopupProps<
	TSize extends string = string,
	TVariant extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	TooltipStyleProps<TSize, TVariant> &
	ResponsiveProps<TooltipStyleProps<TSize, TVariant>, TBreakpoint> & {
		style?: never;
	};

export type TooltipComponents<
	TSize extends string = string,
	TVariant extends string = string,
> = {
	Popup: RemixComponent<TooltipPopupProps<TSize, TVariant>>;
	Root: RemixComponent<TooltipRootProps, TooltipContext>;
	Trigger: RemixComponent<TooltipTriggerProps<TSize, TVariant>>;
};

type TooltipContext = {
	close: (details: TooltipOpenChangeDetails) => void;
	get_open: () => boolean;
	get_popup_id: () => string;
	open: (details: TooltipOpenChangeDetails) => void;
};

const tooltip_scope = "tooltip";
const button_type_default = "button";
const blur_event = "blur";
const focus_event = "focus";
const mouse_enter_event = "mouseenter";
const mouse_leave_event = "mouseleave";

const tooltip_conditions = mergeRecipeConditionSelectors<TooltipRecipeCondition>(
	commonConditions,
	{
		closed: `&[${componentStateAttribute}='${openState.closed}']`,
		open: `&[${componentStateAttribute}='${openState.open}']`,
	} satisfies RecipeConditionSelectorMap<TooltipRecipeCondition>,
);

export function createTooltip<
	TMode extends string,
	TRecipe extends TooltipRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TooltipStyleSystem<TMode, TRecipe, TMetadata>,
): TooltipComponents<
	TooltipRecipeSize<TRecipe>,
	TooltipRecipeVariant<TRecipe>
> {
	type TSize = TooltipRecipeSize<TRecipe>;
	type TVariant = TooltipRecipeVariant<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = TooltipRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.tooltip);

	function resolve_slot(
		slot: "popup" | "trigger",
		props: Partial<TooltipStyleProps<TSize, TVariant>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			size: props.size,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(
		handle: Handle<TooltipRootProps, TooltipContext>,
	): (props: TooltipRootProps) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		function request_open(
			open: boolean,
			details: TooltipOpenChangeDetails,
		): void {
			if (get_open() === open) {
				return;
			}
			if (handle.props.open === undefined) {
				local_open = open;
			}
			handle.props.onOpenChange?.(open, details);
			void handle.update();
		}

		const context: TooltipContext = {
			close: (details) => {
				request_open(false, details);
			},
			get_open,
			get_popup_id: () => {
				return `${handle.id}-popup`;
			},
			open: (details) => {
				request_open(true, details);
			},
		};
		handle.context.set(context);

		return (props: TooltipRootProps): RemixNode => {
			return props.children;
		};
	}

	function Trigger(
		handle: Handle<TooltipTriggerProps<TSize, TVariant, TBreakpoint>>,
	): (props: TooltipTriggerProps<TSize, TVariant, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (
			props: TooltipTriggerProps<TSize, TVariant, TBreakpoint>,
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
						conditions: tooltip_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("trigger", current_props);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						tooltip_scope,
						"trigger",
					),
					mix: [
						on<HTMLButtonElement, typeof mouse_enter_event>(
							mouse_enter_event,
							(event) => {
								context.open({
									event,
									reason: "pointerEnter",
								});
							},
						),
						on<HTMLButtonElement, typeof mouse_leave_event>(
							mouse_leave_event,
							(event) => {
								context.close({
									event,
									reason: "pointerLeave",
								});
							},
						),
						on<HTMLButtonElement, typeof focus_event>(
							focus_event,
							(event) => {
								context.open({ event, reason: "focus" });
							},
						),
						on<HTMLButtonElement, typeof blur_event>(
							blur_event,
							(event) => {
								context.close({ event, reason: "blur" });
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-describedby": context.get_popup_id(),
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
		handle: Handle<TooltipPopupProps<TSize, TVariant, TBreakpoint>>,
	): (props: TooltipPopupProps<TSize, TVariant, TBreakpoint>) => RemixNode {
		const context = handle.context.get(Root);

		return (
			props: TooltipPopupProps<TSize, TVariant, TBreakpoint>,
		): RemixNode => {
			const { at, children, mix, size, variant, ...popup_props } = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				at,
				hostElements: {
					popup: "div",
				},
				targets: {
					popup: {
						host: "popup",
						conditions: tooltip_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("popup", current_props);
						},
					},
				},
				props: { size, variant },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(tooltip_scope, "popup"),
					mix: parts.hosts.popup.mix,
					props: {
						...popup_props,
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						id: popup_props.id ?? context.get_popup_id(),
						mix,
						role: popup_props.role ?? "tooltip",
					},
				}),
				children,
			);
		};
	}

	return {
		Popup,
		Root,
		Trigger,
	};
}

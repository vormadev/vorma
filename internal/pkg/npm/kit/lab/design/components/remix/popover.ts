import {
	createElement,
	on,
	type Handle,
	type Props,
	type RemixNode,
} from "remix/ui";
import type { AnchorOptions } from "remix/ui/anchor";
import * as remixPopover from "remix/ui/popover";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions } from "./conditions.ts";
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
		content: "content",
		surface: "surface",
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

export type PopoverRootProps = {
	children?: RemixNode;
	defaultOpen?: boolean;
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
};

export type PopoverTriggerProps = Omit<Props<"button">, "style"> & {
	placement?: AnchorOptions["placement"];
	style?: never;
};

export type PopoverSurfaceProps<TSize extends string = string> = Omit<
	Props<"div">,
	"style"
> & {
	closeOnAnchorClick?: boolean;
	restoreFocusOnHide?: boolean;
	size?: TSize;
	stopOutsideClickPropagation?: boolean;
	style?: never;
};

export type PopoverContentProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type PopoverComponents<TSize extends string = string> = {
	Content: RemixComponent<PopoverContentProps>;
	Root: RemixComponent<PopoverRootProps, PopoverRuntimeContext>;
	Surface: RemixComponent<PopoverSurfaceProps<TSize>>;
	Trigger: RemixComponent<PopoverTriggerProps>;
};

export type PopoverConditionSelectors = Partial<
	Readonly<Record<PopoverRecipeSlot, RecipeConditionSelectorMap<string>>>
>;

export type PopoverOptions = {
	conditions?: PopoverConditionSelectors;
};

type PopoverRuntimeContext = {
	isOpen: () => boolean;
	setOpen: (open: boolean) => void;
	toggleOpen: () => void;
};

const popover_trigger_conditions = mergeRecipeConditionSelectors<string>(
	commonConditions,
	{
		closed: "&[aria-expanded='false']",
		open: "&[aria-expanded='true']",
	},
);

const popover_surface_conditions = {
	closed: "&:not(:popover-open)",
	open: "&:popover-open",
} as const satisfies RecipeConditionSelectorMap<string>;

const popover_content_conditions = commonConditions;

const click_event = "click";
const button_type_default = "button";
const placement_default = "bottom-start";

export function createPopover<
	TMode extends string,
	TRecipe extends PopoverRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: PopoverStyleSystem<TMode, TRecipe, TMetadata>,
	options: PopoverOptions = {},
): PopoverComponents<
	TRecipe extends PopoverRecipeInput<infer TSize> ? TSize : string
> {
	type TSize =
		TRecipe extends PopoverRecipeInput<infer TPopoverSize>
			? TPopoverSize
			: string;

	const popover_recipe = createRecipe(style_system.token.recipe.popover);

	function Root(
		handle: Handle<PopoverRootProps, PopoverRuntimeContext>,
	): (props: PopoverRootProps) => RemixNode {
		let uncontrolled_open = handle.props.defaultOpen ?? false;
		const context = {
			isOpen: (): boolean => {
				return handle.props.open ?? uncontrolled_open;
			},
			setOpen: (open: boolean): void => {
				if (handle.props.open === undefined) {
					uncontrolled_open = open;
				}
				handle.props.onOpenChange?.(open);
				void handle.update();
			},
			toggleOpen: (): void => {
				context.setOpen(!context.isOpen());
			},
		};
		handle.context.set(context);

		return (props: PopoverRootProps): RemixNode => {
			return createElement(remixPopover.Context, {}, props.children);
		};
	}

	function Trigger(
		handle: Handle<PopoverTriggerProps>,
	): (props: PopoverTriggerProps) => RemixNode {
		const context = handle.context.get(Root);
		const resolved = popover_recipe.resolve();
		const parts = createComponentStyleTargets({
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
				placement = placement_default,
				type = button_type_default,
				...trigger_props
			} = props;

			return createElement(
				"button",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						popoverAnatomy.scope,
						popoverAnatomy.parts.trigger,
					),
					mix: [
						remixPopover.anchor({
							placement,
						}),
						remixPopover.focusOnHide(),
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							() => {
								context.toggleOpen();
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

	function Surface(
		handle: Handle<PopoverSurfaceProps<TSize>>,
	): (props: PopoverSurfaceProps<TSize>) => RemixNode {
		const context = handle.context.get(Root);

		return (props: PopoverSurfaceProps<TSize>): RemixNode => {
			const {
				children,
				closeOnAnchorClick,
				mix,
				restoreFocusOnHide,
				size,
				stopOutsideClickPropagation,
				...surface_props
			} = props;
			const parts = createComponentStyleTargets({
				targets: {
					surface: {
						host: "surface",
						conditions: mergeRecipeConditionSelectors(
							popover_surface_conditions,
							options.conditions?.surface,
						),
						resolveSlot: (style_props) => {
							return popover_recipe.resolve({
								size: style_props.size,
							}).slots[popoverAnatomy.parts.surface];
						},
					},
				},
				props: { size },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						popoverAnatomy.scope,
						popoverAnatomy.parts.surface,
					),
					mix: [
						remixPopover.surface({
							closeOnAnchorClick,
							open: context.isOpen(),
							onHide: () => {
								context.setOpen(false);
							},
							restoreFocusOnHide,
							stopOutsideClickPropagation,
						}),
						parts.hosts.surface.mix,
					],
					props: {
						...surface_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	function Content(
		_handle: Handle<PopoverContentProps>,
	): (props: PopoverContentProps) => RemixNode {
		const resolved = popover_recipe.resolve();
		const parts = createComponentStyleTargets({
			targets: {
				content: {
					host: "content",
					conditions: mergeRecipeConditionSelectors(
						popover_content_conditions,
						options.conditions?.content,
					),
					resolveSlot: () => {
						return resolved.slots[popoverAnatomy.parts.content];
					},
				},
			},
			props: {},
			styleSystem: style_system,
		});

		return (props: PopoverContentProps): RemixNode => {
			const { children, mix, ...content_props } = props;

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						popoverAnatomy.scope,
						popoverAnatomy.parts.content,
					),
					mix: [remixPopover.focusOnShow(), parts.hosts.content.mix],
					props: {
						...content_props,
						mix,
					},
				}),
				children,
			);
		};
	}

	return {
		Content,
		Root,
		Surface,
		Trigger,
	};
}

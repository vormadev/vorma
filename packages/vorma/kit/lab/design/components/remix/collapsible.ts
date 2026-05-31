import { createElement, on, type Handle, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	componentStateAttribute,
	openState,
	openStateFromBoolean,
} from "./component-state.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { commonConditions, type CommonRecipeCondition } from "./conditions.ts";
import {
	mergeRecipeConditionSelectors,
	type RecipeConditionSelectorMap,
} from "./recipe.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type CollapsibleRecipeCondition = CommonRecipeCondition | "closed" | "open";

export type CollapsibleRecipeInput<TLayout extends string = string> =
	RecipeWithVariantGroups<
		"content" | "root" | "trigger",
		string,
		ComponentStyle,
		{
			layout: TLayout;
		}
	>;

export type CollapsibleRecipeLayout<TRecipe extends CollapsibleRecipeInput> =
	RecipeVariantValue<TRecipe, "layout">;

export type CollapsibleRecipeSelection<TRecipe extends CollapsibleRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "layout">;

export type CollapsibleStyleSystem<
	TMode extends string = string,
	TRecipe extends CollapsibleRecipeInput = CollapsibleRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { collapsible: TRecipe } }, TMetadata>;

export type CollapsibleOpenChangeDetails = {
	event?: Event;
	reason: "trigger";
};

export type CollapsibleOpenChangeHandler = (
	open: boolean,
	details?: CollapsibleOpenChangeDetails,
) => void;

export type CollapsibleRootStyleProps<TLayout extends string = string> = {
	layout?: TLayout;
};

export type CollapsibleRootProps<
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	CollapsibleRootStyleProps<TLayout> &
	ResponsiveProps<CollapsibleRootStyleProps<TLayout>, TBreakpoint> & {
		defaultOpen?: boolean;
		onOpenChange?: CollapsibleOpenChangeHandler;
		onOpenChangeComplete?: CollapsibleOpenChangeHandler;
		open?: boolean;
		style?: never;
	};

export type CollapsibleTriggerProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type CollapsibleContentProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type CollapsibleComponents<TLayout extends string = string> = {
	Content: RemixComponent<CollapsibleContentProps>;
	Root: RemixComponent<CollapsibleRootProps<TLayout>, CollapsibleContext>;
	Trigger: RemixComponent<CollapsibleTriggerProps>;
};

type CollapsibleContext = {
	get_content_id: () => string;
	get_open: () => boolean;
	get_trigger_id: () => string;
	toggle: (details: CollapsibleOpenChangeDetails) => void;
};

const collapsible_scope = "collapsible";
const button_type_default = "button";
const click_event = "click";

const collapsible_conditions = mergeRecipeConditionSelectors<CollapsibleRecipeCondition>(
	commonConditions,
	{
		closed: `&[${componentStateAttribute}='${openState.closed}']`,
		open: `&[${componentStateAttribute}='${openState.open}']`,
	} satisfies RecipeConditionSelectorMap<CollapsibleRecipeCondition>,
);

export function createCollapsible<
	TMode extends string,
	TRecipe extends CollapsibleRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: CollapsibleStyleSystem<TMode, TRecipe, TMetadata>,
): CollapsibleComponents<CollapsibleRecipeLayout<TRecipe>> {
	type TLayout = CollapsibleRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const recipe = createRecipe(style_system.token.recipe.collapsible);

	function resolve_slot(
		slot: "content" | "root" | "trigger",
		props: Partial<CollapsibleRootStyleProps<TLayout>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			layout: props.layout,
		} satisfies CollapsibleRecipeSelection<TRecipe>).slots[slot];
	}

	function Root(
		handle: Handle<CollapsibleRootProps<TLayout, TBreakpoint>, CollapsibleContext>,
	): (props: CollapsibleRootProps<TLayout, TBreakpoint>) => RemixNode {
		let local_open = handle.props.defaultOpen ?? false;
		let last_completed_open = handle.props.open ?? local_open;
		let last_open_details: CollapsibleOpenChangeDetails | undefined;

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		function request_open(
			open: boolean,
			details: CollapsibleOpenChangeDetails,
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

		const context: CollapsibleContext = {
			get_content_id: () => {
				return `${handle.id}-content`;
			},
			get_open,
			get_trigger_id: () => {
				return `${handle.id}-trigger`;
			},
			toggle: (details) => {
				request_open(!get_open(), details);
			},
		};
		handle.context.set(context);

		return (props: CollapsibleRootProps<TLayout, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultOpen: _default_open,
				layout,
				mix,
				onOpenChange: _on_open_change,
				onOpenChangeComplete: _on_open_change_complete,
				open: _open,
				...root_props
			} = props;
			const current_open = get_open();
			if (current_open !== last_completed_open) {
				const completed_open = current_open;
				const completed_details = last_open_details;
				last_completed_open = current_open;
				handle.queueTask((signal) => {
					if (signal.aborted) {
						return;
					}
					props.onOpenChangeComplete?.(completed_open, completed_details);
				});
			}
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: collapsible_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("root", current_props);
						},
					},
				},
				props: { layout },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(collapsible_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...root_props,
						[componentStateAttribute]: openStateFromBoolean(current_open),
						mix,
					},
				}),
				children,
			);
		};
	}

	function Trigger(
		handle: Handle<CollapsibleTriggerProps>,
	): (props: CollapsibleTriggerProps) => RemixNode {
		const context = handle.context.get(Root);

		return (props: CollapsibleTriggerProps): RemixNode => {
			const { children, mix, type = button_type_default, ...trigger_props } = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				targets: {
					trigger: {
						host: "trigger",
						conditions: collapsible_conditions,
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
					attrs: createComponentAnatomyAttrs(collapsible_scope, "trigger"),
					mix: [
						on<HTMLButtonElement, typeof click_event>(
							click_event,
							(event) => {
								context.toggle({
									event,
									reason: "trigger",
								});
							},
						),
						parts.hosts.trigger.mix,
					],
					props: {
						...trigger_props,
						"aria-controls": context.get_content_id(),
						"aria-expanded": open,
						[componentStateAttribute]: openStateFromBoolean(open),
						id: trigger_props.id ?? context.get_trigger_id(),
						mix,
						type,
					},
				}),
				children,
			);
		};
	}

	function Content(
		handle: Handle<CollapsibleContentProps>,
	): (props: CollapsibleContentProps) => RemixNode {
		const context = handle.context.get(Root);

		return (props: CollapsibleContentProps): RemixNode => {
			const { children, mix, ...content_props } = props;
			const open = context.get_open();
			const parts = createComponentStyleTargets({
				targets: {
					content: {
						host: "content",
						conditions: collapsible_conditions,
						resolveSlot: () => {
							return resolve_slot("content", {});
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(collapsible_scope, "content"),
					mix: parts.hosts.content.mix,
					props: {
						...content_props,
						"aria-labelledby": context.get_trigger_id(),
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						id: content_props.id ?? context.get_content_id(),
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
		Trigger,
	};
}

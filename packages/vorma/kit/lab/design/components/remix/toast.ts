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

export type ToastRecipeCondition = CommonRecipeCondition | "closed" | "open";

export type ToastRecipeInput<
	TTone extends string = string,
	TVariant extends string = string,
> = RecipeWithVariantGroups<
	"close" | "description" | "root" | "title" | "viewport",
	string,
	ComponentStyle,
	{
		tone: TTone;
		variant: TVariant;
	}
>;

export type ToastRecipeTone<TRecipe extends ToastRecipeInput> = RecipeVariantValue<
	TRecipe,
	"tone"
>;

export type ToastRecipeVariant<TRecipe extends ToastRecipeInput> = RecipeVariantValue<
	TRecipe,
	"variant"
>;

export type ToastRecipeSelection<TRecipe extends ToastRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "tone" | "variant">;

export type ToastStyleSystem<
	TMode extends string = string,
	TRecipe extends ToastRecipeInput = ToastRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { toast: TRecipe } }, TMetadata>;

export type ToastOpenChangeDetails = {
	event?: Event;
	reason: "close";
};

export type ToastOpenChangeHandler = (
	open: boolean,
	details?: ToastOpenChangeDetails,
) => void;

export type ToastRootStyleProps<
	TTone extends string = string,
	TVariant extends string = string,
> = {
	tone?: TTone;
	variant?: TVariant;
};

export type ToastRootProps<
	TTone extends string = string,
	TVariant extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	ToastRootStyleProps<TTone, TVariant> &
	ResponsiveProps<ToastRootStyleProps<TTone, TVariant>, TBreakpoint> & {
		defaultOpen?: boolean;
		onOpenChange?: ToastOpenChangeHandler;
		open?: boolean;
		style?: never;
	};

export type ToastViewportProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type ToastTitleProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type ToastDescriptionProps = Omit<Props<"div">, "style"> & {
	style?: never;
};

export type ToastCloseProps = Omit<Props<"button">, "style"> & {
	style?: never;
};

export type ToastComponents<
	TTone extends string = string,
	TVariant extends string = string,
> = {
	Close: RemixComponent<ToastCloseProps>;
	Description: RemixComponent<ToastDescriptionProps>;
	Root: RemixComponent<ToastRootProps<TTone, TVariant>, ToastContext>;
	Title: RemixComponent<ToastTitleProps>;
	Viewport: RemixComponent<ToastViewportProps>;
};

type ToastContext = {
	close: (details: ToastOpenChangeDetails) => void;
	get_open: () => boolean;
};

const toast_scope = "toast";
const button_type_default = "button";
const click_event = "click";

const toast_conditions = mergeRecipeConditionSelectors<ToastRecipeCondition>(
	commonConditions,
	{
		closed: `&[${componentStateAttribute}='${openState.closed}']`,
		open: `&[${componentStateAttribute}='${openState.open}']`,
	} satisfies RecipeConditionSelectorMap<ToastRecipeCondition>,
);

export function createToast<
	TMode extends string,
	TRecipe extends ToastRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ToastStyleSystem<TMode, TRecipe, TMetadata>,
): ToastComponents<ToastRecipeTone<TRecipe>, ToastRecipeVariant<TRecipe>> {
	type TTone = ToastRecipeTone<TRecipe>;
	type TVariant = ToastRecipeVariant<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = ToastRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.toast);

	function resolve_slot(
		slot: "close" | "description" | "root" | "title" | "viewport",
		props: Partial<ToastRootStyleProps<TTone, TVariant>>,
	): ReturnType<typeof recipe.resolve>["slots"][typeof slot] {
		return recipe.resolve({
			tone: props.tone,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(
		handle: Handle<ToastRootProps<TTone, TVariant, TBreakpoint>, ToastContext>,
	): (props: ToastRootProps<TTone, TVariant, TBreakpoint>) => RemixNode {
		let local_open = handle.props.defaultOpen ?? true;

		function get_open(): boolean {
			return handle.props.open ?? local_open;
		}

		const context: ToastContext = {
			close: (details) => {
				if (!get_open()) {
					return;
				}
				if (handle.props.open === undefined) {
					local_open = false;
				}
				handle.props.onOpenChange?.(false, details);
				void handle.update();
			},
			get_open,
		};
		handle.context.set(context);

		return (props: ToastRootProps<TTone, TVariant, TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				defaultOpen: _default_open,
				mix,
				onOpenChange: _on_open_change,
				open: _open,
				tone,
				variant,
				...root_props
			} = props;
			const open = get_open();
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: toast_conditions,
						resolveSlot: (current_props) => {
							return resolve_slot("root", current_props);
						},
					},
				},
				props: { tone, variant },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(toast_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...root_props,
						[componentStateAttribute]: openStateFromBoolean(open),
						hidden: !open,
						mix,
						role: root_props.role ?? "status",
					},
				}),
				children,
			);
		};
	}

	function create_static_part<TElement extends "button" | "div">(
		element: TElement,
		slot: "close" | "description" | "title" | "viewport",
	): RemixComponent<Omit<Props<TElement>, "style"> & { style?: never }> {
		return (handle: Handle<Omit<Props<TElement>, "style">>) => {
			const context = slot === "close" ? handle.context.get(Root) : null;

			return (
				props: Omit<Props<TElement>, "style"> & { style?: never },
			): RemixNode => {
				const { children, mix, ...part_props } = props;
				const host_props: Record<string, unknown> = {
					...part_props,
					mix,
				};
				if (element === "button" && host_props.type === undefined) {
					host_props.type = button_type_default;
				}
				const parts = createComponentStyleTargets({
					targets: {
						[slot]: {
							host: slot,
							conditions: toast_conditions,
							resolveSlot: () => {
								return resolve_slot(slot, {});
							},
						},
					} as Record<
						typeof slot,
						{
							conditions: typeof toast_conditions;
							host: typeof slot;
							resolveSlot: () => ReturnType<
								typeof recipe.resolve
							>["slots"][typeof slot];
						}
					>,
					props: {},
					styleSystem: style_system,
				});
				const close_mix =
					slot === "close" && context
						? on<HTMLButtonElement, typeof click_event>(
								click_event,
								(event) => {
									context.close({ event, reason: "close" });
								},
							)
						: undefined;

				return createElement(
					element,
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(toast_scope, slot),
						mix: [close_mix, parts.hosts[slot].mix],
						props: host_props,
					}),
					children,
				);
			};
		};
	}

	return {
		Close: create_static_part("button", "close"),
		Description: create_static_part("div", "description"),
		Root,
		Title: create_static_part("div", "title"),
		Viewport: create_static_part("div", "viewport"),
	};
}

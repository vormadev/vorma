import { createElement, type Props, type RemixNode } from "remix/ui";
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
import { commonConditions } from "./conditions.ts";
import {
	type BreakpointForStyleSystem,
	type ResponsiveProps,
} from "./responsive.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

const alert_slot = {
	description: "description",
	icon: "icon",
	root: "root",
	title: "title",
} as const;

export type AlertRecipeSlot = (typeof alert_slot)[keyof typeof alert_slot];

export type AlertRecipeInput<
	TTone extends string = string,
	TVariant extends string = string,
> = RecipeWithVariantGroups<
	AlertRecipeSlot,
	string,
	ComponentStyle,
	{
		tone: TTone;
		variant: TVariant;
	}
>;

export type AlertRecipeTone<TRecipe extends AlertRecipeInput> =
	RecipeVariantValue<TRecipe, "tone">;

export type AlertRecipeVariant<TRecipe extends AlertRecipeInput> =
	RecipeVariantValue<TRecipe, "variant">;

export type AlertRecipeSelection<TRecipe extends AlertRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "tone" | "variant">;

export type AlertStyleSystem<
	TMode extends string = string,
	TRecipe extends AlertRecipeInput = AlertRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { alert: TRecipe } }, TMetadata>;

export type AlertRootStyleProps<
	TTone extends string = string,
	TVariant extends string = string,
> = {
	tone?: TTone;
	variant?: TVariant;
};

export type AlertRootProps<
	TTone extends string = string,
	TVariant extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"div">, "style"> &
	AlertRootStyleProps<TTone, TVariant> &
	ResponsiveProps<AlertRootStyleProps<TTone, TVariant>, TBreakpoint> & {
		style?: never;
	};

export type AlertIconProps = Omit<Props<"span">, "style"> & {
	style?: never;
};

export type AlertTitleProps = Omit<Props<"h2">, "style"> & {
	style?: never;
};

export type AlertDescriptionProps = Omit<Props<"p">, "style"> & {
	style?: never;
};

export type AlertComponents<
	TTone extends string = string,
	TVariant extends string = string,
> = {
	Description: RemixComponent<AlertDescriptionProps>;
	Icon: RemixComponent<AlertIconProps>;
	Root: RemixComponent<AlertRootProps<TTone, TVariant>>;
	Title: RemixComponent<AlertTitleProps>;
};

const alert_scope = "alert";
const alert_role = "alert";

export function createAlert<
	TMode extends string,
	TRecipe extends AlertRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: AlertStyleSystem<TMode, TRecipe, TMetadata>,
): AlertComponents<AlertRecipeTone<TRecipe>, AlertRecipeVariant<TRecipe>> {
	type TTone = AlertRecipeTone<TRecipe>;
	type TVariant = AlertRecipeVariant<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	type TSelection = AlertRecipeSelection<TRecipe>;
	const recipe = createRecipe(style_system.token.recipe.alert);

	function resolve_slot(
		slot: AlertRecipeSlot,
		props: Partial<AlertRootStyleProps<TTone, TVariant>>,
	): ReturnType<typeof recipe.resolve>["slots"][AlertRecipeSlot] {
		return recipe.resolve({
			tone: props.tone,
			variant: props.variant,
		} satisfies TSelection).slots[slot];
	}

	function Root(): (props: AlertRootProps<TTone, TVariant, TBreakpoint>) => RemixNode {
		return (props: AlertRootProps<TTone, TVariant, TBreakpoint>): RemixNode => {
			const { at, children, mix, tone, variant, ...root_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						conditions: commonConditions,
						resolveSlot: (current_props) => {
							return resolve_slot(alert_slot.root, current_props);
						},
					},
				},
				props: { tone, variant },
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						alert_scope,
						alert_slot.root,
					),
					mix: parts.hosts.root.mix,
					props: {
						...root_props,
						mix,
						role: root_props.role ?? alert_role,
					},
				}),
				children,
			);
		};
	}

	function create_static_part<TElement extends "h2" | "p" | "span">(
		element: TElement,
		slot: AlertRecipeSlot,
	): RemixComponent<Omit<Props<TElement>, "style"> & { style?: never }> {
		return () => {
			return (
				props: Omit<Props<TElement>, "style"> & { style?: never },
			): RemixNode => {
				const { children, mix, ...part_props } = props;
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
							>["slots"][AlertRecipeSlot];
						}
					>,
					props: {},
					styleSystem: style_system,
				});

				return createElement(
					element,
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(alert_scope, slot),
						mix: parts.hosts[slot].mix,
						props: {
							...part_props,
							mix,
						},
					}),
					children,
				);
			};
		};
	}

	return {
		Description: create_static_part("p", alert_slot.description),
		Icon: create_static_part("span", alert_slot.icon),
		Root,
		Title: create_static_part("h2", alert_slot.title),
	};
}

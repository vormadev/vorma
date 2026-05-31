import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeStyle,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
	type ComponentSlotProps,
} from "./component-style.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type ProgressRecipeInput<TTone extends string = string> = RecipeWithVariantGroups<
	"root" | "segment",
	never,
	ComponentStyle,
	{
		tone: TTone;
	}
>;

export type ProgressRecipeTone<TRecipe extends ProgressRecipeInput> = RecipeVariantValue<
	TRecipe,
	"tone"
>;
export type ProgressRecipeSelection<TRecipe extends ProgressRecipeInput> =
	RecipeVariantPropsFor<TRecipe, "tone">;

export type ProgressStyleSystem<
	TMode extends string = string,
	TRecipe extends ProgressRecipeInput = ProgressRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { progress: TRecipe } }, TMetadata>;

export type ProgressSegment<TTone extends string = string> = {
	key?: string;
	label?: RemixNode;
	props?: ComponentSlotProps<"div">;
	tone?: TTone;
	value: number;
};

export type ProgressProps<TTone extends string = string> = Omit<Props<"div">, "style"> & {
	max?: number;
	segments?: readonly ProgressSegment<TTone>[];
	style?: never;
	tone?: TTone;
	value?: number;
};

const progress_scope = "progress";
const progress_min = 0;
const progress_max_default = 100;

function normalize_progress_max(value: number | undefined): number {
	if (value === undefined || !Number.isFinite(value) || value <= 0) {
		return progress_max_default;
	}
	return value;
}

function normalize_progress_value(value: number, max: number): number {
	if (!Number.isFinite(value)) {
		return progress_min;
	}
	return Math.min(max, Math.max(progress_min, value));
}

function progress_value_width(value: number, max: number): string {
	const normalized_value = normalize_progress_value(value, max);
	return `${(normalized_value / max) * 100}%`;
}

export function createProgress<
	TMode extends string,
	TRecipe extends ProgressRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: ProgressStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<ProgressProps<ProgressRecipeTone<TRecipe>>> {
	type TTone = ProgressRecipeTone<TRecipe>;
	const progress_recipe = createRecipe(style_system.token.recipe.progress);

	return () => {
		return (props: ProgressProps<TTone>): RemixNode => {
			const {
				children,
				max: max_input,
				segments: segments_input,
				tone,
				value,
				...progress_props
			} = props;
			const max = normalize_progress_max(max_input);
			const segments =
				segments_input ?? (value === undefined ? [] : [{ tone, value }]);
			const value_now =
				value === undefined && segments_input === undefined
					? undefined
					: normalize_progress_value(
							segments.reduce((total, segment) => {
								return (
									total + normalize_progress_value(segment.value, max)
								);
							}, 0),
							max,
						);
			const root = progress_recipe.resolve();
			const root_parts = createComponentStyleTargets({
				targets: {
					root: {
						host: "root",
						resolveSlot: () => {
							return root.slots.root;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(progress_scope, "root"),
					mix: root_parts.hosts.root.mix,
					props: {
						...progress_props,
						"aria-valuemax": progress_props["aria-valuemax"] ?? max,
						"aria-valuemin": progress_props["aria-valuemin"] ?? progress_min,
						"aria-valuenow": progress_props["aria-valuenow"] ?? value_now,
						role: progress_props.role ?? "progressbar",
					},
				}),
				segments.map((segment, index) => {
					const { children: _segment_children, ...segment_props } =
						segment.props ?? {};
					const resolved = progress_recipe.resolve({
						tone: segment.tone,
					} satisfies ProgressRecipeSelection<TRecipe>);
					const segment_parts = createComponentStyleTargets({
						targets: {
							segment: {
								host: "segment",
								resolveSlot: () => {
									return resolved.slots.segment;
								},
								resolveStyle: (): RecipeStyle => {
									return {
										width: progress_value_width(segment.value, max),
									};
								},
							},
						},
						props: {},
						styleSystem: style_system,
					});

					return createElement(
						"div",
						createComponentSlotProps({
							attrs: createComponentAnatomyAttrs(progress_scope, "segment"),
							mix: segment_parts.hosts.segment.mix,
							props: {
								...segment_props,
								"aria-label":
									segment_props["aria-label"] ??
									(typeof segment.label === "string"
										? segment.label
										: undefined),
								key: segment.key ?? segment_props.key ?? index,
							},
						}),
					);
				}),
				children,
			);
		};
	};
}

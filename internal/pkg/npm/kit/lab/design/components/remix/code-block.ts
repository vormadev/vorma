import { createElement, type Props, type RemixNode } from "remix/ui";
import {
	createRecipe,
	type AnyGeneratedSystemMetadata,
	type RecipeWithVariantGroups,
} from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
	type ComponentSlotProps,
} from "./component-style.ts";
import type {
	ComponentStyle,
	ComponentStyleSystem,
	RemixComponent,
} from "./types.ts";

export type CodeBlockRecipeInput = RecipeWithVariantGroups<
	"caption" | "code" | "pre" | "root" | "summary",
	never,
	ComponentStyle,
	Record<string, never>
>;

export type CodeBlockStyleSystem<
	TMode extends string = string,
	TRecipe extends CodeBlockRecipeInput = CodeBlockRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { codeBlock: TRecipe } }, TMetadata>;

export type CodeBlockProps = Omit<Props<"figure">, "style"> & {
	caption?: RemixNode;
	captionProps?: ComponentSlotProps<"figcaption">;
	codeProps?: ComponentSlotProps<"code">;
	preProps?: ComponentSlotProps<"pre">;
	style?: never;
	summary?: RemixNode;
	summaryProps?: ComponentSlotProps<"figcaption">;
};

const code_block_scope = "codeBlock";

export function createCodeBlock<
	TMode extends string,
	TRecipe extends CodeBlockRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: CodeBlockStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<CodeBlockProps> {
	const code_block_recipe = createRecipe(style_system.token.recipe.codeBlock);

	return () => {
		return (props: CodeBlockProps): RemixNode => {
			const {
				caption,
				captionProps: caption_props_input,
				children,
				codeProps: code_props,
				preProps: pre_props,
				summary,
				summaryProps: summary_props,
				...figure_props
			} = props;
			const resolved = code_block_recipe.resolve();
			const {
				children: _code_children,
				mix: code_mix,
				...code_host_props
			} = code_props ?? {};
			const {
				children: _pre_children,
				mix: pre_mix,
				...pre_host_props
			} = pre_props ?? {};
			const { children: _summary_children, ...summary_host_props } =
				summary_props ?? {};
			const { children: _caption_children, ...caption_host_props } =
				caption_props_input ?? {};
			const parts = createComponentStyleTargets({
				targets: {
					caption: {
						host: "caption",
						resolveSlot: () => {
							return resolved.slots.caption;
						},
					},
					code: {
						host: "code",
						resolveSlot: () => {
							return resolved.slots.code;
						},
					},
					pre: {
						host: "pre",
						resolveSlot: () => {
							return resolved.slots.pre;
						},
					},
					root: {
						host: "root",
						resolveSlot: () => {
							return resolved.slots.root;
						},
					},
					summary: {
						host: "summary",
						resolveSlot: () => {
							return resolved.slots.summary;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"figure",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						code_block_scope,
						"root",
					),
					mix: parts.hosts.root.mix,
					props: figure_props,
				}),
				summary === undefined
					? null
					: createElement(
							"figcaption",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									code_block_scope,
									"summary",
								),
								mix: parts.hosts.summary.mix,
								props: summary_host_props,
							}),
							summary,
						),
				createElement(
					"pre",
					createComponentSlotProps({
						attrs: createComponentAnatomyAttrs(
							code_block_scope,
							"pre",
						),
						mix: parts.hosts.pre.mix,
						props: {
							...pre_host_props,
							mix: pre_mix,
						},
					}),
					createElement(
						"code",
						createComponentSlotProps({
							attrs: createComponentAnatomyAttrs(
								code_block_scope,
								"code",
							),
							mix: parts.hosts.code.mix,
							props: {
								...code_host_props,
								mix: code_mix,
							},
						}),
						children,
					),
				),
				caption === undefined
					? null
					: createElement(
							"figcaption",
							createComponentSlotProps({
								attrs: createComponentAnatomyAttrs(
									code_block_scope,
									"caption",
								),
								mix: parts.hosts.caption.mix,
								props: caption_host_props,
							}),
							caption,
						),
			);
		};
	};
}

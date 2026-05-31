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
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyle, ComponentStyleSystem, RemixComponent } from "./types.ts";

export type TableCellRole = "body" | "columnHeader" | "rowHeader";
export type TableHeaderCellScope = "col" | "row";

export type TableRecipeInput<
	TDensity extends string = string,
	TLayout extends string = string,
	TRowVariant extends string = string,
	TCellVariant extends string = string,
> = RecipeWithVariantGroups<
	"body" | "cell" | "head" | "root" | "row",
	never,
	ComponentStyle,
	{
		cellRole: TableCellRole;
		cellVariant: TCellVariant;
		density: TDensity;
		layout: TLayout;
		rowVariant: TRowVariant;
	}
>;

export type TableRecipeDensity<TRecipe extends TableRecipeInput> = RecipeVariantValue<
	TRecipe,
	"density"
>;
export type TableRecipeLayout<TRecipe extends TableRecipeInput> = RecipeVariantValue<
	TRecipe,
	"layout"
>;
export type TableRecipeRowVariant<TRecipe extends TableRecipeInput> = RecipeVariantValue<
	TRecipe,
	"rowVariant"
>;
export type TableRecipeCellVariant<TRecipe extends TableRecipeInput> = RecipeVariantValue<
	TRecipe,
	"cellVariant"
>;
export type TableRecipeSelection<TRecipe extends TableRecipeInput> =
	RecipeVariantPropsFor<
		TRecipe,
		"cellRole" | "cellVariant" | "density" | "layout" | "rowVariant"
	>;

export type TableStyleSystem<
	TMode extends string = string,
	TRecipe extends TableRecipeInput = TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = ComponentStyleSystem<TMode, { recipe: { table: TRecipe } }, TMetadata>;

export type TableStyleProps<
	TDensity extends string = string,
	TLayout extends string = string,
> = {
	density?: TDensity;
	layout?: TLayout;
};

export type TableProps<
	TDensity extends string = string,
	TLayout extends string = string,
	TBreakpoint extends string = string,
> = Omit<Props<"table">, "style"> &
	TableStyleProps<TDensity, TLayout> &
	ResponsiveProps<TableStyleProps<TDensity, TLayout>, TBreakpoint> & {
		style?: never;
	};

export type TableSectionProps<TElement extends "tbody" | "thead"> = Omit<
	Props<TElement>,
	"style"
> & {
	style?: never;
};

export type TableRowProps<TRowVariant extends string = string> = Omit<
	Props<"tr">,
	"style"
> & {
	style?: never;
	variant?: TRowVariant;
};

export type TableCellProps<TCellVariant extends string = string> = Omit<
	Props<"td">,
	"style"
> & {
	style?: never;
	variant?: TCellVariant;
};

export type TableHeaderCellProps<TCellVariant extends string = string> = Omit<
	Props<"th">,
	"scope" | "style"
> & {
	scope?: TableHeaderCellScope;
	style?: never;
	variant?: TCellVariant;
};

const table_scope = "table";

export function createTable<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<
	TableProps<
		TableRecipeDensity<TRecipe>,
		TableRecipeLayout<TRecipe>,
		BreakpointForStyleSystem<typeof style_system>
	>
> {
	type TDensity = TableRecipeDensity<TRecipe>;
	type TLayout = TableRecipeLayout<TRecipe>;
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;
	const table_recipe = createRecipe(style_system.token.recipe.table);

	function resolve_root_slot(
		props: Partial<TableStyleProps<TDensity, TLayout>>,
	): ReturnType<typeof table_recipe.resolve>["slots"]["root"] {
		return table_recipe.resolve({
			density: props.density,
			layout: props.layout,
		} satisfies TableRecipeSelection<TRecipe>).slots.root;
	}

	return () => {
		return (props: TableProps<TDensity, TLayout, TBreakpoint>): RemixNode => {
			const { at, children, density, layout, mix, ...table_props } = props;
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						resolveSlot: resolve_root_slot,
					},
				},
				props: { density, layout },
				styleSystem: style_system,
			});

			return createElement(
				"table",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...table_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createTableHead<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<TableSectionProps<"thead">> {
	const table_recipe = createRecipe(style_system.token.recipe.table);

	return () => {
		return (props: TableSectionProps<"thead">): RemixNode => {
			const { children, mix, ...headProps } = props;
			const resolved = table_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					head: {
						host: "head",
						resolveSlot: () => {
							return resolved.slots.head;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			return createElement(
				"thead",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "head"),
					mix: parts.hosts.head.mix,
					props: {
						...headProps,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createTableBody<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<TableSectionProps<"tbody">> {
	const table_recipe = createRecipe(style_system.token.recipe.table);

	return () => {
		return (props: TableSectionProps<"tbody">): RemixNode => {
			const { children, mix, ...bodyProps } = props;
			const resolved = table_recipe.resolve();
			const parts = createComponentStyleTargets({
				targets: {
					body: {
						host: "body",
						resolveSlot: () => {
							return resolved.slots.body;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});
			return createElement(
				"tbody",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "body"),
					mix: parts.hosts.body.mix,
					props: {
						...bodyProps,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createTableRow<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<TableRowProps<TableRecipeRowVariant<TRecipe>>> {
	const table_recipe = createRecipe(style_system.token.recipe.table);

	return () => {
		return (props: TableRowProps<TableRecipeRowVariant<TRecipe>>): RemixNode => {
			const { children, mix, variant, ...row_props } = props;
			const resolved = table_recipe.resolve({
				rowVariant: variant,
			} satisfies TableRecipeSelection<TRecipe>);
			const parts = createComponentStyleTargets({
				targets: {
					row: {
						host: "row",
						resolveSlot: () => {
							return resolved.slots.row;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"tr",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "row"),
					mix: parts.hosts.row.mix,
					props: {
						...row_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createTableCell<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<TableCellProps<TableRecipeCellVariant<TRecipe>>> {
	const table_recipe = createRecipe(style_system.token.recipe.table);

	return () => {
		return (props: TableCellProps<TableRecipeCellVariant<TRecipe>>): RemixNode => {
			const { children, mix, variant, ...cell_props } = props;
			const resolved = table_recipe.resolve({
				cellRole: "body",
				cellVariant: variant,
			} satisfies TableRecipeSelection<TRecipe>);
			const parts = createComponentStyleTargets({
				targets: {
					cell: {
						host: "cell",
						resolveSlot: () => {
							return resolved.slots.cell;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"td",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "cell"),
					mix: parts.hosts.cell.mix,
					props: {
						...cell_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

export function createTableHeaderCell<
	TMode extends string,
	TRecipe extends TableRecipeInput,
	TMetadata extends AnyGeneratedSystemMetadata,
>(
	style_system: TableStyleSystem<TMode, TRecipe, TMetadata>,
): RemixComponent<TableHeaderCellProps<TableRecipeCellVariant<TRecipe>>> {
	const table_recipe = createRecipe(style_system.token.recipe.table);

	return () => {
		return (
			props: TableHeaderCellProps<TableRecipeCellVariant<TRecipe>>,
		): RemixNode => {
			const { children, mix, scope = "col", variant, ...cell_props } = props;
			const cell_role: Extract<TableCellRole, "columnHeader" | "rowHeader"> =
				scope === "row" ? "rowHeader" : "columnHeader";
			const resolved = table_recipe.resolve({
				cellRole: cell_role,
				cellVariant: variant,
			} satisfies TableRecipeSelection<TRecipe>);
			const parts = createComponentStyleTargets({
				targets: {
					cell: {
						host: "cell",
						resolveSlot: () => {
							return resolved.slots.cell;
						},
					},
				},
				props: {},
				styleSystem: style_system,
			});

			return createElement(
				"th",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(table_scope, "cell"),
					mix: parts.hosts.cell.mix,
					props: {
						...cell_props,
						mix,
						scope,
					},
				}),
				children,
			);
		};
	};
}

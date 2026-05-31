// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createTable,
	createTableBody,
	createTableCell,
	createTableHead,
	createTableHeaderCell,
	createTableRow,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Table", () => {
	setup_remix_component_test_environment();

	it("renders native header scope semantics", () => {
		const table_recipe = {
			slots: { body: {}, cell: {}, head: {}, root: {}, row: {} },
			variants: {
				cellRole: {
					body: { cell: {} },
					columnHeader: { cell: {} },
					rowHeader: { cell: {} },
				},
				cellVariant: { default: { cell: {} } },
				density: { compact: { root: {} } },
				layout: { data: { root: {} } },
				rowVariant: { striped: { row: {} } },
			},
		} as const;
		const style_system = {
			metadata: { breakpoint: { md: "48rem" } },
			modes: { light: { variables: {} } },
			token: { recipe: { table: table_recipe } },
			variablePrefix: "test",
		} as const;
		const Table = createTable(style_system);
		const TableHead = createTableHead(style_system);
		const TableBody = createTableBody(style_system);
		const TableRow = createTableRow(style_system);
		const TableCell = createTableCell(style_system);
		const TableHeaderCell = createTableHeaderCell(style_system);
		const result = render(
			createElement(
				Table,
				{ density: "compact", layout: "data" },
				createElement(
					TableHead,
					{},
					createElement(
						TableRow,
						{ variant: "striped" },
						createElement(TableHeaderCell, { variant: "default" }, "Name"),
					),
				),
				createElement(
					TableBody,
					{},
					createElement(
						TableRow,
						{},
						createElement(
							TableHeaderCell,
							{ scope: "row", variant: "default" },
							"Ada",
						),
						createElement(TableCell, { variant: "default" }, "42"),
					),
				),
			),
		);

		const table = result.$("table");
		const column_header = result.$("thead th");
		const row_header = result.$("tbody th");
		const body_cell = result.$("td");
		expect(table?.getAttribute(componentAnatomyAttrs.scope)).toBe("table");
		expect(column_header?.getAttribute("scope")).toBe("col");
		expect(row_header?.getAttribute("scope")).toBe("row");
		expect(body_cell?.textContent).toBe("42");

		result.cleanup();
	});
});

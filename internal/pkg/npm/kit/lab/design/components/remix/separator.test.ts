// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	createSeparator,
	separatorDecorativeRole,
	separatorHiddenAttribute,
	separatorOrientation,
	separatorOrientationAttribute,
	separatorRole,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const separator_test_id = "separator";

function create_test_separator(): ReturnType<typeof createSeparator> {
	return createSeparator({
		metadata: {},
		modes: { light: { variables: {} } },
		token: { recipe: { separator: { slots: { root: {} } } } },
		variablePrefix: "test",
	});
}

describe("Remix Separator", () => {
	setup_remix_component_test_environment();

	it("renders an accessible horizontal separator by default", () => {
		const Separator = create_test_separator();
		const result = render(
			createElement(Separator, { "data-testid": separator_test_id }),
		);
		const separator = result.$(`[data-testid='${separator_test_id}']`);

		expect(separator?.tagName).toBe("DIV");
		expect(separator?.getAttribute("role")).toBe(separatorRole);
		expect(separator?.getAttribute(separatorOrientationAttribute)).toBe(
			separatorOrientation.horizontal,
		);
		expect(separator?.hasAttribute(separatorHiddenAttribute)).toBe(false);

		result.cleanup();
	});

	it("supports vertical orientation", () => {
		const Separator = create_test_separator();
		const result = render(
			createElement(Separator, {
				"data-testid": separator_test_id,
				orientation: separatorOrientation.vertical,
			}),
		);
		const separator = result.$(`[data-testid='${separator_test_id}']`);

		expect(separator?.getAttribute("role")).toBe(separatorRole);
		expect(separator?.getAttribute(separatorOrientationAttribute)).toBe(
			separatorOrientation.vertical,
		);

		result.cleanup();
	});

	it("can be removed from the accessibility tree when decorative", () => {
		const Separator = create_test_separator();
		const result = render(
			createElement(Separator, {
				"data-testid": separator_test_id,
				decorative: true,
			}),
		);
		const separator = result.$(`[data-testid='${separator_test_id}']`);

		expect(separator?.getAttribute("role")).toBe(separatorDecorativeRole);
		expect(separator?.getAttribute(separatorHiddenAttribute)).toBe("true");

		result.cleanup();
	});
});

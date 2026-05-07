// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createVisuallyHidden,
	visuallyHiddenDefaultElement,
	visuallyHiddenPart,
	visuallyHiddenScope,
	visuallyHiddenStyle,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const visually_hidden_test_id = "visually-hidden";
const test_variable_prefix = "test";

function create_test_visually_hidden(): ReturnType<
	typeof createVisuallyHidden
> {
	return createVisuallyHidden({
		metadata: {},
		modes: { light: { variables: {} } },
		token: {},
		variablePrefix: test_variable_prefix,
	});
}

function style_text(): string {
	return document.adoptedStyleSheets
		.flatMap((sheet) => {
			return Array.from(sheet.cssRules).map((rule) => {
				return rule.cssText;
			});
		})
		.join("\n");
}

describe("Remix VisuallyHidden", () => {
	setup_remix_component_test_environment();

	it("renders accessible content in a visually hidden host", () => {
		const VisuallyHidden = create_test_visually_hidden();
		const result = render(
			createElement(
				VisuallyHidden,
				{ "data-testid": visually_hidden_test_id },
				"Settings",
			),
		);
		const host = result.$(`[data-testid='${visually_hidden_test_id}']`);

		expect(host?.tagName.toLowerCase()).toBe(visuallyHiddenDefaultElement);
		expect(host?.textContent).toBe("Settings");
		expect(host?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			visuallyHiddenScope,
		);
		expect(host?.getAttribute(componentAnatomyAttrs.part)).toBe(
			visuallyHiddenPart.root,
		);
		const styles = style_text();
		expect(styles).toContain(`clip-path: ${visuallyHiddenStyle.clipPath}`);
		expect(styles).toContain(`position: ${visuallyHiddenStyle.position}`);

		result.cleanup();
	});

	it("allows the host element to be changed", () => {
		const VisuallyHidden = create_test_visually_hidden();
		const result = render(
			createElement(
				VisuallyHidden,
				{
					"data-testid": visually_hidden_test_id,
					as: "div",
				},
				"Hidden region label",
			),
		);
		const host = result.$(`[data-testid='${visually_hidden_test_id}']`);

		expect(host?.tagName).toBe("DIV");

		result.cleanup();
	});

	it("keeps focusable content visible on focus or active", () => {
		const VisuallyHidden = create_test_visually_hidden();
		const result = render(
			createElement(
				VisuallyHidden,
				{
					"data-testid": visually_hidden_test_id,
					isFocusable: true,
				},
				"Skip to content",
			),
		);
		const host = result.$(`[data-testid='${visually_hidden_test_id}']`);

		expect(host?.textContent).toBe("Skip to content");
		expect(host?.className).toContain("rmxc-");

		result.cleanup();
	});
});

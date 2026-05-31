// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createAspectRatio } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix AspectRatio", () => {
	setup_remix_component_test_environment();

	it("renders aspect ratio anatomy", () => {
		const AspectRatio = createAspectRatio({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {},
			variablePrefix: "test",
		});
		const result = render(
			createElement(AspectRatio, {
				"data-testid": "aspect",
				ratio: "16 / 9",
			}),
		);

		expect(
			result.$("[data-testid='aspect']")?.getAttribute(componentAnatomyAttrs.scope),
		).toBe("aspectRatio");

		result.cleanup();
	});
});

// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createAlert, type AlertStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Alert", () => {
	setup_remix_component_test_environment();

	it("renders composable alert anatomy", () => {
		const alert_recipe = {
			defaultVariants: {
				tone: "info",
				variant: "soft",
			},
			slots: {
				description: {},
				icon: {},
				root: {},
				title: {},
			},
			variants: {
				tone: {
					info: {
						root: {},
					},
				},
				variant: {
					soft: {
						root: {},
					},
				},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					alert: alert_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies AlertStyleSystem<"light", typeof alert_recipe>;
		const Alert = createAlert(style_system);
		const result = render(
			createElement(
				Alert.Root,
				{},
				createElement(Alert.Icon, {}, "!"),
				createElement(Alert.Title, {}, "Heads up"),
				createElement(Alert.Description, {}, "Something changed."),
			),
		);

		const root = result.$("[role='alert']");
		expect(root?.getAttribute(componentAnatomyAttrs.scope)).toBe("alert");
		expect(root?.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(result.$("span")?.getAttribute(componentAnatomyAttrs.part)).toBe("icon");
		expect(result.$("h2")?.getAttribute(componentAnatomyAttrs.part)).toBe("title");
		expect(result.$("p")?.getAttribute(componentAnatomyAttrs.part)).toBe(
			"description",
		);

		result.cleanup();
	});
});

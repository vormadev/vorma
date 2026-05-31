// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createCodeBlock, type CodeBlockStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix CodeBlock", () => {
	setup_remix_component_test_environment();

	it("renders figure semantics and named slot ownership", () => {
		const code_block_recipe = {
			slots: {
				caption: {
					base: {
						color: "gray",
					},
				},
				code: {
					base: {
						fontFamily: "monospace",
					},
				},
				header: {
					base: {
						fontWeight: 600,
					},
				},
				pre: {
					base: {
						overflowX: "auto",
					},
				},
				root: {
					base: {
						margin: 0,
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
					codeBlock: code_block_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies CodeBlockStyleSystem<"light", typeof code_block_recipe>;
		const CodeBlock = createCodeBlock(style_system);
		const result = render(
			createElement(
				CodeBlock,
				{
					caption: "Use it anywhere",
					codeProps: {
						"data-code": "example",
					},
					header: "Install",
					headerProps: {
						"data-header": "example",
					},
					preProps: {
						"data-pre": "example",
					},
				},
				"pnpm add vorma",
			),
		);

		expect(result.$("figure")).not.toBeNull();
		expect(result.$("details")).toBeNull();
		expect(result.$("[data-vorma-part='header']")?.textContent).toBe("Install");
		expect(result.$("[data-vorma-part='header']")?.getAttribute("data-header")).toBe(
			"example",
		);
		expect(result.$("figcaption")?.textContent).toBe("Use it anywhere");
		expect(result.$("pre")?.getAttribute("data-pre")).toBe("example");
		expect(result.$("code")?.getAttribute("data-code")).toBe("example");
		expect(result.$("code")?.textContent).toBe("pnpm add vorma");

		result.cleanup();
	});
});

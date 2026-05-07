// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
	createRecipeStyle,
	createResponsiveRecipeStyle,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix component style compiler", () => {
	setup_remix_component_test_environment();

	it("maps renderer-neutral recipe conditions to Remix selectors", () => {
		const style = createRecipeStyle({
			conditions: {
				hover: "&:hover",
				open: "&[data-open]",
			},
			slot: {
				base: {
					color: "black",
				},
				conditions: {
					hover: {
						color: "blue",
					},
					open: {
						opacity: 1,
					},
				},
			},
		});

		expect(style).toEqual({
			"&:hover": {
				color: "blue",
			},
			"&[data-open]": {
				opacity: 1,
			},
			color: "black",
		});
	});

	it("requires component layers to define selectors for recipe conditions", () => {
		expect(() => {
			createRecipeStyle({
				slot: {
					base: {},
					conditions: {
						hover: {
							color: "blue",
						},
					},
				},
			});
		}).toThrow(/Missing Remix recipe condition selector for "hover"/);
	});

	it("maps responsive recipe slots with condition selectors inside media queries", () => {
		const style = createResponsiveRecipeStyle({
			at: {
				md: {
					variant: "primary",
				},
			},
			conditions: {
				hover: "&:hover",
			},
			resolve: () => {
				return {
					base: {
						color: "blue",
					},
					conditions: {
						hover: {
							color: "navy",
						},
					},
				};
			},
			styleSystem: {
				metadata: {
					breakpoint: {
						md: "48rem",
					},
				},
				modes: {
					light: {
						variables: {},
					},
				},
				token: {},
				variablePrefix: "test",
			},
		});

		expect(style).toEqual({
			"@media (min-width: 48rem)": {
				"&:hover": {
					color: "navy",
				},
				color: "blue",
			},
		});
	});

	it("compiles selector-backed style targets into host styles", () => {
		const targets = createComponentStyleTargets({
			at: {
				md: {
					tone: "strong",
				},
			},
			props: {},
			styleSystem: {
				metadata: {
					breakpoint: {
						md: "48rem",
					},
				},
				modes: {
					light: {
						variables: {},
					},
				},
				token: {},
				variablePrefix: "test",
			},
			targets: {
				input: {
					conditions: {
						focusVisible: "&:focus-visible",
					},
					host: "input",
					resolveSlot: () => {
						return {
							base: {
								color: "black",
							},
							conditions: {
								focusVisible: {
									outline: "2px solid blue",
								},
							},
						};
					},
				},
				thumb: {
					conditions: {
						focusVisible: "&:focus-visible",
					},
					host: "input",
					resolveSlot: (props) => {
						return {
							base: {
								background:
									props.tone === "strong" ? "maroon" : "red",
							},
							conditions: {
								focusVisible: {
									boxShadow: "0 0 0 2px blue",
								},
							},
						};
					},
					selectors: ["&::-webkit-slider-thumb"],
				},
			},
		});

		expect(targets.hosts.input.style).toMatchObject({
			"&:focus-visible": {
				outline: "2px solid blue",
			},
			"&:focus-visible::-webkit-slider-thumb": {
				boxShadow: "0 0 0 2px blue",
			},
			"&::-webkit-slider-thumb": {
				background: "red",
			},
			"@media (min-width: 48rem)": {
				"&::-webkit-slider-thumb": {
					background: "maroon",
				},
			},
			color: "black",
		});
	});

	it("merges consumer mix after component system mix", () => {
		const props = createComponentSlotProps({
			attrs: createComponentAnatomyAttrs("test", "root"),
			mix: "system-mix",
			props: {
				mix: "consumer-mix",
			},
		});

		expect(props.mix).toEqual(["system-mix", "consumer-mix"]);
	});

	it("emits component recipe styles through Remix css mixins", () => {
		const targets = createComponentStyleTargets({
			props: {},
			styleSystem: {
				metadata: {},
				modes: {
					light: {
						variables: {},
					},
				},
				token: {},
				variablePrefix: "test",
			},
			targets: {
				root: {
					host: "root",
					resolveSlot: () => {
						return {
							base: {
								color: "white",
							},
							conditions: {},
						};
					},
				},
			},
		});
		const result = render(
			createElement("button", {
				mix: targets.hosts.root.mix,
			}),
		);
		const style_text = document.adoptedStyleSheets
			.flatMap((sheet) => {
				return Array.from(sheet.cssRules).map((rule) => {
					return rule.cssText;
				});
			})
			.join("\n");

		expect(
			document.querySelector("style[data-vorma-component-style]"),
		).toBeNull();
		expect(result.$("button")?.className).toContain("rmxc-");
		expect(style_text).toContain("@layer rmx.rmxc-");
		expect(style_text).toContain("color: white");

		result.cleanup();
	});
});

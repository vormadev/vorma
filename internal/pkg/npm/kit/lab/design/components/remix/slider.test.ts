// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createSlider,
	sliderProgressVariable,
	type SliderStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Slider", () => {
	setup_remix_component_test_environment();

	it("applies Slider input props and mix to the range input", async () => {
		const slider_recipe = {
			defaultVariants: {
				layout: "stacked",
			},
			slots: {
				progress: {},
				root: {
					base: {
						display: "block",
					},
				},
				thumb: {},
				track: {},
			},
			variants: {
				layout: {
					stacked: {
						root: {
							base: {
								gap: "0.5rem",
							},
						},
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
					slider: slider_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SliderStyleSystem<"light", typeof slider_recipe>;
		const Slider = createSlider(style_system);
		let current_target_tag: string | undefined;
		let current_target_value: string | undefined;
		const result = render(
			createElement(Slider, {
				id: "presale-range",
				max: "100",
				min: "0",
				mix: on<HTMLInputElement, "input">("input", (event) => {
					current_target_tag = event.currentTarget.tagName;
					current_target_value = event.currentTarget.value;
				}),
				value: "25",
			}),
		);

		const input = result.$("input") as HTMLInputElement | null;
		expect(input).not.toBeNull();
		if (!input) {
			throw new Error("Expected Slider to render an input");
		}
		expect(input.style.getPropertyValue(sliderProgressVariable)).toBe(
			"25.0000%",
		);
		await result.act(() => {
			input.value = "40";
			input.dispatchEvent(new Event("input", { bubbles: true }));
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_value).toBe("40");
		expect(input.style.getPropertyValue(sliderProgressVariable)).toBe(
			"40.0000%",
		);
		expect(input.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(result.$("label")).toBe(null);
		expect(result.$("span")).toBe(null);

		result.cleanup();
	});

	it("maps Slider pseudo targets onto the input host mix", () => {
		type StyleMixNode = {
			props: {
				mix: readonly [
					unknown,
					{
						args: readonly [Record<string, unknown>];
					},
				];
			};
		};
		const slider_recipe = {
			slots: {
				progress: {
					base: {
						background: "blue",
					},
				},
				root: {},
				thumb: {
					base: {
						inlineSize: "1rem",
					},
					conditions: {
						focusVisible: {
							boxShadow: "0 0 0 2px blue",
						},
					},
				},
				track: {
					base: {
						blockSize: "0.25rem",
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
					slider: slider_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SliderStyleSystem<"light", typeof slider_recipe>;
		const Slider = createSlider(style_system, {
			targetStyles: {
				thumbWebkit: {
					marginTop: "-0.375rem",
				},
			},
		});
		const node = Slider({} as never)({
			value: "50",
		}) as unknown as StyleMixNode;
		const input_style = node.props.mix[1].args[0];

		expect(input_style).toMatchObject({
			"&::-moz-range-progress": {
				background: "blue",
			},
			"&::-moz-range-thumb": {
				inlineSize: "1rem",
			},
			"&::-moz-range-track": {
				blockSize: "0.25rem",
			},
			"&::-webkit-slider-runnable-track": {
				blockSize: "0.25rem",
			},
			"&::-webkit-slider-thumb": {
				inlineSize: "1rem",
				marginTop: "-0.375rem",
			},
			"&:focus-visible::-moz-range-thumb": {
				boxShadow: "0 0 0 2px blue",
			},
			"&:focus-visible::-webkit-slider-thumb": {
				boxShadow: "0 0 0 2px blue",
			},
		});
		expect(
			(input_style["&::-moz-range-thumb"] as Record<string, unknown>)
				.marginTop,
		).toBeUndefined();
	});
});

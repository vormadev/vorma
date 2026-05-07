// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createProgress } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Progress", () => {
	setup_remix_component_test_environment();

	it("renders accessible single value progress by default", () => {
		type ProgressNode = {
			props: {
				children: readonly [
					readonly [
						{
							props: {
								mix: readonly [
									{
										args: readonly [
											Record<string, unknown>,
										];
									},
								];
							};
						},
					],
					unknown,
				];
			};
		};
		const progress_recipe = {
			slots: { root: {}, segment: {} },
			variants: { tone: { info: { segment: {} } } },
		} as const;
		const Progress = createProgress({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { progress: progress_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(Progress, {
				"aria-label": "Upload progress",
				tone: "info",
				value: 25,
			}),
		);

		const root = result.$("[role='progressbar']");
		expect(root?.getAttribute("aria-valuemin")).toBe("0");
		expect(root?.getAttribute("aria-valuemax")).toBe("100");
		expect(root?.getAttribute("aria-valuenow")).toBe("25");
		expect(
			result.$$(
				"[data-vorma-scope='progress'][data-vorma-part='segment']",
			),
		).toHaveLength(1);
		result.cleanup();

		const node = Progress({} as never)({
			value: 150,
		}) as unknown as ProgressNode;
		const first_segment_style =
			node.props.children[0][0].props.mix[0].args[0];
		expect(first_segment_style).toMatchObject({ width: "100%" });
	});

	it("supports segmented progress as an extension", () => {
		type ProgressNode = {
			props: {
				children: readonly [
					readonly [
						{
							props: {
								mix: readonly [
									{
										args: readonly [
											Record<string, unknown>,
										];
									},
								];
							};
						},
						{
							props: {
								mix: readonly [
									{
										args: readonly [
											Record<string, unknown>,
										];
									},
								];
							};
						},
					],
					unknown,
				];
			};
		};
		const progress_recipe = {
			slots: { root: {}, segment: {} },
			variants: { tone: { info: { segment: {} } } },
		} as const;
		const Progress = createProgress({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { progress: progress_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(Progress, {
				"aria-label": "Upload progress",
				segments: [
					{ label: "Uploaded", tone: "info", value: 25 },
					{ label: "Processed", value: 30 },
				],
			}),
		);

		expect(
			result
				.$("[data-vorma-scope='progress'][data-vorma-part='segment']")
				?.getAttribute("aria-label"),
		).toBe("Uploaded");
		expect(
			result.$("[role='progressbar']")?.getAttribute("aria-valuenow"),
		).toBe("55");
		result.cleanup();

		const node = Progress({} as never)({
			segments: [{ value: -20 }, { value: 150 }],
		}) as unknown as ProgressNode;
		const first_segment_style =
			node.props.children[0][0].props.mix[0].args[0];
		const second_segment_style =
			node.props.children[0][1].props.mix[0].args[0];
		expect(first_segment_style).toMatchObject({ width: "0%" });
		expect(second_segment_style).toMatchObject({ width: "100%" });
	});
});

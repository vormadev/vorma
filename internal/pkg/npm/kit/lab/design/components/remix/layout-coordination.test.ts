// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import {
	createRecipe,
	createRecipeStyleSystem,
	type GeneratedSystem,
	type RecipeStyleDefinitions,
	type SingleRecipeStyleInput,
} from "../../core/core.ts";
import { createButton, createCodeBlock, createStack } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix layout and style-system coordination", () => {
	setup_remix_component_test_environment();

	it("applies responsive Stack structural styles", () => {
		type StyleMixNode = {
			props: {
				mix: readonly [
					{
						args: readonly [Record<string, unknown>];
					},
				];
			};
		};
		const stack_recipe = {
			defaultVariants: {
				gap: "4",
				layout: "siteFooterTop",
			},
			slots: {
				root: {
					base: {
						display: "flex",
					},
				},
			},
			variants: {
				gap: {
					"4": {
						root: {
							base: {
								gap: "1rem",
							},
						},
					},
					none: {
						root: {
							base: {
								gap: 0,
							},
						},
					},
				},
				layout: {
					siteFooterTop: {
						root: {
							base: {
								inlineSize: "100%",
							},
						},
					},
				},
			},
		} as const;
		const style_system = {
			metadata: {
				breakpoint: {
					lg: "64rem",
				},
			},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					stack: stack_recipe,
				},
			},
			variablePrefix: "test",
		} as const;
		const Stack = createStack(style_system);
		const node = Stack({} as never)({
			align: "center",
			at: {
				lg: {
					align: "start",
					direction: "row",
					gap: "none",
				},
			},
			direction: "column",
			gap: "4",
			justify: "between",
			layout: "siteFooterTop",
		}) as unknown as StyleMixNode;
		const style = node.props.mix[0].args[0];

		expect(style).toMatchObject({
			"@media (min-width: 64rem)": {
				alignItems: "flex-start",
				display: "flex",
				flexDirection: "row",
				gap: 0,
				inlineSize: "100%",
				justifyContent: "space-between",
			},
			alignItems: "center",
			display: "flex",
			flexDirection: "column",
			gap: "1rem",
			inlineSize: "100%",
			justifyContent: "space-between",
		});
	});

	it("preserves recipe specificity through generic style-system wrappers", () => {
		const recipe_system = {
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
		} satisfies GeneratedSystem<
			"light",
			{},
			{ breakpoint: { md: string } }
		>;

		function create_component_style_system<
			const TRecipes extends RecipeStyleDefinitions,
		>(recipes: TRecipes & SingleRecipeStyleInput<TRecipes>) {
			return createRecipeStyleSystem<typeof recipe_system, TRecipes>(
				recipe_system,
				recipes,
			);
		}

		const button_recipe = {
			defaultVariants: {
				layout: "control",
				size: "md",
				variant: "primary",
			},
			slots: {
				content: {},
				loadingIndicator: {},
				loadingIndicatorFrame: {},
				root: {
					base: {
						display: "inline-flex",
					},
				},
			},
			variants: {
				fluid: {
					true: {
						root: {
							base: {
								inlineSize: "100%",
							},
						},
					},
				},
				layout: {
					control: {
						root: {
							base: {
								justifyContent: "center",
							},
						},
					},
				},
				loading: {
					true: {
						content: {
							base: {
								opacity: 0,
							},
						},
					},
				},
				size: {
					md: {
						root: {
							base: {
								minHeight: "2rem",
							},
						},
					},
				},
				variant: {
					primary: {
						root: {
							base: {
								color: "white",
							},
						},
					},
				},
			},
		} as const;
		const code_block_recipe = {
			slots: {
				caption: {},
				code: {},
				header: {
					base: {
						fontWeight: 600,
					},
				},
				pre: {},
				root: {},
			},
		} as const;

		const Button = createButton(
			create_component_style_system({ button: button_recipe }),
		);
		const code_block_style_system = create_component_style_system({
			codeBlock: code_block_recipe,
		});
		const CodeBlock = createCodeBlock(code_block_style_system);
		const code_block_resolved = createRecipe(
			code_block_style_system.token.recipe.codeBlock,
		).resolve();

		expect(Button).toBeInstanceOf(Function);
		expect(CodeBlock).toBeInstanceOf(Function);
		expect(code_block_resolved.slots.header.base.fontWeight).toBe(600);
	});
});

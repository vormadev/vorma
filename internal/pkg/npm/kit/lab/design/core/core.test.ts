import { describe, expect, it } from "vitest";
import {
	createCSSVariableName,
	createCSSVariableReference,
	createRecipe,
	createRecipeStyleSystem,
	createSystem,
	defineRecipeStyles,
	defineSystem,
	mergeRecipeStyles,
	renderCSSVariables,
	type RecipeStyle,
	type RecipeVariantPropsFor,
	type RecipeVariantValue,
	type RecipeWithVariantGroups,
} from "./core.ts";
import { primitive_token_keys, token_groups } from "./types.ts";

const hover_condition = "&:hover";

type ButtonRecipeForTest<
	TVariant extends string = string,
	TSize extends string = string,
	TLayout extends string = string,
> = RecipeWithVariantGroups<
	"content" | "root",
	"hover",
	RecipeStyle,
	{
		fluid: "true";
		layout: TLayout;
		loading: "true";
		size: TSize;
		variant: TVariant;
	}
>;

type ButtonRecipeSelectionForTest<TRecipe extends ButtonRecipeForTest> =
	RecipeVariantPropsFor<
		TRecipe,
		"fluid" | "layout" | "loading" | "size" | "variant"
	>;

type ButtonRecipeVariantForTest<TRecipe extends ButtonRecipeForTest> =
	RecipeVariantValue<TRecipe, "variant">;

function resolve_button_recipe_for_test<
	const TRecipe extends ButtonRecipeForTest,
>(recipe: TRecipe, selection: ButtonRecipeSelectionForTest<TRecipe>) {
	return createRecipe(recipe).resolve(selection);
}

describe("design core", () => {
	it("creates primitive, semantic, and recipe style references", () => {
		const input = defineSystem({
			css: {
				variablePrefix: "acme",
			},
			modes: ["light", "dark"],
			primitive: {
				color: {
					source: {
						neutral: {
							chroma: 0.02,
							hue: 250,
							lightness: 0.65,
						},
					},
					palette: {
						neutral: {
							chromaLimit: 0.04,
							curve: {
								light: {
									chroma: {
										"1": 0.4,
									},
									lightness: {
										"1": 0.98,
									},
								},
								dark: {
									chroma: {
										"1": 0.5,
									},
									lightness: {
										"1": 0.2,
									},
								},
							},
							source: "neutral",
						},
					},
				},
				focus: {
					ring: {
						default: {
							color: "#38bdf8",
							style: "solid",
							width: "2px",
						},
					},
				},
				layout: {
					breakpoint: {
						md: "48rem",
					},
				},
				motion: {
					distance: {
						press: "1px",
					},
				},
				typography: {
					family: {
						sans: "Inter",
					},
					[primitive_token_keys.letter_spacing]: {
						normal: "0",
					},
					[primitive_token_keys.line_height]: {
						normal: "1.5",
					},
					size: {
						baseFontSizePx: 16,
						basePx: 16,
						ratio: 1.25,
						step: {
							body: 0,
						},
						unit: "rem",
					},
					[primitive_token_keys.text_style]: {
						body: {
							family: "sans",
							letterSpacing: "normal",
							lineHeight: "normal",
							size: "body",
							weight: "regular",
						},
					},
					weight: {
						regular: 400,
					},
				},
			},
			semantic: ({ ref, style }) => {
				return {
					light: {
						color: {
							surface: ref.primitive.color.palette.neutral["1"]!,
							text: "#111827",
						},
						typography: {
							body: style.textStyle("body"),
						},
					},
					dark: {
						color: {
							surface: ref.primitive.color.palette.neutral["1"]!,
							text: "#f9fafb",
						},
						typography: {
							body: style.textStyle("body"),
						},
					},
				};
			},
		});
		const system = createSystem(input);
		const recipes = defineRecipeStyles(input, ({ ref, style }) => {
			return {
				button: {
					defaultVariants: {
						size: "md",
						tone: "primary",
					},
					compoundVariants: [
						{
							slots: {
								root: {
									base: {
										fontWeight:
											ref.primitive.typography.weight
												.regular,
									},
									conditions: {
										[hover_condition]: {
											borderColor:
												ref.semantic.color.surface,
										},
									},
								},
							},
							variants: {
								size: "md",
								tone: "primary",
							},
						},
					],
					slots: {
						root: {
							base: {
								background: ref.semantic.color.surface,
								color: ref.semantic.color.text,
								transform: style.template`translateY(${ref.primitive.motion.distance.press})`,
								...style.focusRing("default"),
							},
							conditions: {
								[hover_condition]: {
									color: ref.semantic.color.text,
								},
							},
						},
					},
					variants: {
						size: {
							md: {
								root: {
									base: {
										minHeight:
											ref.primitive.motion.distance.press,
									},
								},
							},
						},
						tone: {
							primary: {
								root: {
									base: {},
									conditions: {
										[hover_condition]: {
											background:
												ref.semantic.color.surface,
										},
									},
								},
							},
						},
					},
				},
			};
		});
		const recipe_system = createRecipeStyleSystem(system, recipes);
		const button_recipe = createRecipe(recipe_system.token.recipe.button);
		const resolved_button = button_recipe.resolve({
			size: "md",
			tone: "primary",
		});
		const breakpoint_metadata: Record<"md", string | number> =
			system.metadata.breakpoint;
		const resolved_button_root = resolved_button.slots.root;

		expect(breakpoint_metadata).toEqual({
			md: "48rem",
		});
		expect(system.variablePrefix).toBe("acme");
		expect(system.token.primitive.color.palette.neutral["1"]).toBe(
			createCSSVariableReference("acme", [
				token_groups.primitive,
				"color",
				"palette",
				"neutral",
				"1",
			]),
		);
		expect(system.token.semantic.color.surface).toBe(
			createCSSVariableReference("acme", [
				token_groups.semantic,
				"color",
				"surface",
			]),
		);
		const recipe_root_base =
			recipe_system.token.recipe.button.slots.root.base;

		expect(recipe_system.modes.light.variables).toEqual({});
		expect(recipe_root_base?.background).toBe(
			createCSSVariableReference("acme", [
				token_groups.semantic,
				"color",
				"surface",
			]),
		);
		expect(recipe_root_base?.outline).toBe(
			createCSSVariableReference("acme", [
				token_groups.primitive,
				"focus",
				"ring",
				"default",
			]),
		);
		expect(recipe_root_base?.transform).toBe(
			`translateY(${createCSSVariableReference("acme", [
				token_groups.primitive,
				"motion",
				"distance",
				"press",
			])})`,
		);
		expect(resolved_button_root.base).toEqual({
			background: createCSSVariableReference("acme", [
				token_groups.semantic,
				"color",
				"surface",
			]),
			color: createCSSVariableReference("acme", [
				token_groups.semantic,
				"color",
				"text",
			]),
			fontWeight: createCSSVariableReference("acme", [
				token_groups.primitive,
				"typography",
				"weight",
				"regular",
			]),
			minHeight: createCSSVariableReference("acme", [
				token_groups.primitive,
				"motion",
				"distance",
				"press",
			]),
			outline: createCSSVariableReference("acme", [
				token_groups.primitive,
				"focus",
				"ring",
				"default",
			]),
			transform: `translateY(${createCSSVariableReference("acme", [
				token_groups.primitive,
				"motion",
				"distance",
				"press",
			])})`,
		});
		expect(resolved_button_root.conditions).toEqual({
			[hover_condition]: {
				background: createCSSVariableReference("acme", [
					token_groups.semantic,
					"color",
					"surface",
				]),
				borderColor: createCSSVariableReference("acme", [
					token_groups.semantic,
					"color",
					"surface",
				]),
				color: createCSSVariableReference("acme", [
					token_groups.semantic,
					"color",
					"text",
				]),
			},
		});
		expect(
			renderCSSVariables(":root", system.modes.light.variables),
		).toContain(
			`${createCSSVariableName("acme", [
				token_groups.semantic,
				"color",
				"surface",
			])}: ${createCSSVariableReference("acme", [
				token_groups.primitive,
				"color",
				"palette",
				"neutral",
				"1",
			])};`,
		);
	});

	it("requires CSS variable safe prefix and token path segments", () => {
		expect(() => {
			createCSSVariableName("Acme", [token_groups.primitive]);
		}).toThrow(/Invalid CSS variable prefix/);

		expect(() => {
			createCSSVariableName("--acme", [token_groups.primitive]);
		}).toThrow(/Invalid CSS variable prefix/);

		expect(() => {
			createCSSVariableName("acme", [
				token_groups.primitive,
				"camelCase",
			]);
		}).toThrow(/Invalid CSS variable path segment/);
	});

	it("resolves token mixes", () => {
		const input = defineSystem({
			css: {
				variablePrefix: "acme",
			},
			modes: ["light"],
			primitive: {
				color: {
					source: {
						neutral: {
							chroma: 0.02,
							hue: 250,
							lightness: 0.65,
						},
					},
				},
				content: {
					measure: {
						card: "16rem",
					},
				},
			},
			semantic: ({ mix, ref, style }) => {
				return {
					light: {
						color: {
							tint: mix({
								amount: "35%",
								color: ref.primitive.color.source.neutral,
								space: "oklch",
								with: "#fff",
							}),
						},
						layout: {
							grid: style.template`minmax(min(100%, ${ref.primitive.content.measure.card}), 1fr)`,
						},
					},
				};
			},
		});
		const system = createSystem(input);

		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.semantic,
					"color",
					"tint",
				])
			],
		).toBe(
			`color-mix(in oklch, ${createCSSVariableReference("acme", [
				token_groups.primitive,
				"color",
				"source",
				"neutral",
			])} 35%, #fff)`,
		);
		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.semantic,
					"layout",
					"grid",
				])
			],
		).toBe(
			`minmax(min(100%, ${createCSSVariableReference("acme", [
				token_groups.primitive,
				"content",
				"measure",
				"card",
			])}), 1fr)`,
		);
	});

	it("compiles border, shadow, and transition composite tokens", () => {
		const input = defineSystem({
			css: {
				variablePrefix: "acme",
			},
			modes: ["light"],
			primitive: {
				border: {
					shorthand: {
						default: {
							color: "#111",
							style: "solid",
							width: "1px",
						},
					},
				},
				effect: {
					shadow: {
						raised: {
							blur: "2px",
							color: "rgb(0 0 0 / 0.2)",
							spread: "0px",
							x: "0px",
							y: "1px",
						},
					},
				},
				motion: {
					distance: {
						press: "1px",
					},
					transition: {
						fast: {
							duration: "120ms",
							easing: "ease-out",
							property: "opacity",
						},
					},
				},
			},
			semantic: () => {
				return {};
			},
		});
		const system = createSystem(input);

		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.primitive,
					"border",
					"shorthand",
					"default",
				])
			],
		).toBe("1px solid #111");
		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.primitive,
					"effect",
					"shadow",
					"raised",
				])
			],
		).toBe("0px 1px 2px 0px rgb(0 0 0 / 0.2)");
		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.primitive,
					"motion",
					"distance",
					"press",
				])
			],
		).toBe("1px");
		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.primitive,
					"motion",
					"transition",
					"fast",
				])
			],
		).toBe("opacity 120ms ease-out");
	});

	it("emits CSS safe text style keys and omits missing linked parts", () => {
		const input = defineSystem({
			css: {
				variablePrefix: "acme",
			},
			modes: ["light"],
			primitive: {
				typography: {
					family: {
						sans: "Inter",
					},
					[primitive_token_keys.text_style]: {
						body: {
							family: "sans",
							letterSpacing: "missing",
							lineHeight: "missing",
							size: "missing",
							weight: "missing",
						},
					},
				},
			},
			semantic: ({ style }) => {
				return {
					light: {
						typography: {
							body: style.textStyle("body"),
						},
					},
				};
			},
		});
		const system = createSystem(input);

		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.semantic,
					"typography",
					"body",
					primitive_token_keys.font_family,
				])
			],
		).toBe(
			createCSSVariableReference("acme", [
				token_groups.primitive,
				"typography",
				primitive_token_keys.text_style,
				"body",
				primitive_token_keys.font_family,
			]),
		);
		expect(
			system.modes.light.variables[
				createCSSVariableName("acme", [
					token_groups.semantic,
					"typography",
					"body",
					primitive_token_keys.letter_spacing,
				])
			],
		).toBeUndefined();
	});

	it("rejects unsafe semantic token path segments during compilation", () => {
		const input = defineSystem({
			css: {
				variablePrefix: "acme",
			},
			modes: ["light"],
			primitive: {},
			semantic: () => {
				return {
					light: {
						dataVisualization: {
							color: "#0f0",
						},
					},
				};
			},
		});

		expect(() => {
			createSystem(input);
		}).toThrow(/Invalid CSS variable path segment "dataVisualization"/);
	});

	it("resolves renderer-neutral recipe slots, variants, and conditions", () => {
		const recipe = createRecipe({
			compoundVariants: [
				{
					slots: {
						root: {
							base: {
								fontWeight: 700,
							},
							conditions: {
								[hover_condition]: {
									borderColor: "crimson",
								},
							},
						},
					},
					variants: {
						size: "sm",
						tone: "danger",
					},
				},
			],
			defaultVariants: {
				size: "md",
				tone: "primary",
			},
			slots: {
				icon: {
					base: {
						inlineSize: "1rem",
					},
				},
				root: {
					base: {
						display: "inline-flex",
						gap: "0.5rem",
					},
					conditions: {
						[hover_condition]: {
							background: "white",
						},
					},
				},
			},
			variants: {
				size: {
					md: {
						root: {
							base: {
								minHeight: "2.5rem",
							},
						},
					},
					sm: {
						root: {
							base: {
								minHeight: "2rem",
							},
						},
					},
				},
				tone: {
					danger: {
						root: {
							base: {
								background: "red",
							},
							conditions: {
								[hover_condition]: {
									background: "darkred",
								},
								open: {
									boxShadow: "0 0 0 2px red",
								},
							},
						},
					},
					primary: {
						root: {
							base: {
								background: "blue",
							},
							conditions: {
								[hover_condition]: {
									background: "navy",
								},
							},
						},
					},
				},
			},
		});

		const resolved = recipe.resolve({
			size: "sm",
			tone: "danger",
		});

		expect(resolved.slots.root.base).toEqual({
			background: "red",
			display: "inline-flex",
			fontWeight: 700,
			gap: "0.5rem",
			minHeight: "2rem",
		});
		expect(resolved.slots.root.conditions).toEqual({
			[hover_condition]: {
				background: "darkred",
				borderColor: "crimson",
			},
			open: {
				boxShadow: "0 0 0 2px red",
			},
		});
		expect(resolved.slots.icon.base).toEqual({
			inlineSize: "1rem",
		});
	});

	it("resolves variant groups by recipe definition order", () => {
		const recipe = createRecipe({
			compoundVariants: [
				{
					slots: {
						root: {
							base: {
								borderColor: "selected-neutral",
							},
						},
					},
					variants: {
						selected: "true",
						variant: "neutral",
					},
				},
			],
			slots: {
				root: {
					base: {
						background: "transparent",
						color: "black",
					},
				},
			},
			variants: {
				size: {
					sm: {
						root: {
							base: {
								padding: "0.25rem",
							},
						},
					},
				},
				variant: {
					neutral: {
						root: {
							base: {
								background: "gray",
								color: "black",
							},
							conditions: {
								[hover_condition]: {
									background: "darkgray",
								},
							},
						},
					},
				},
				selected: {
					true: {
						root: {
							base: {
								background: "blue",
								color: "white",
							},
							conditions: {
								[hover_condition]: {
									background: "darkblue",
								},
							},
						},
					},
				},
			},
		});

		const resolved = recipe.resolve({
			selected: "true",
			size: "sm",
			variant: "neutral",
		});

		expect(resolved.slots.root.base).toEqual({
			background: "blue",
			borderColor: "selected-neutral",
			color: "white",
			padding: "0.25rem",
		});
		expect(resolved.slots.root.conditions).toEqual({
			[hover_condition]: {
				background: "darkblue",
			},
		});
	});

	it("provides variant-group helper types for generic component factories", () => {
		const recipe: ButtonRecipeForTest<"primary", "md", "control"> = {
			slots: {
				content: {
					base: {
						display: "inline-flex",
					},
				},
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
								minWidth: "8rem",
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
								minHeight: "2.5rem",
							},
						},
					},
				},
				variant: {
					primary: {
						root: {
							base: {
								background: "blue",
							},
							conditions: {
								hover: {
									background: "navy",
								},
							},
						},
					},
				},
			},
		};
		const variant: ButtonRecipeVariantForTest<typeof recipe> = "primary";
		const resolved = resolve_button_recipe_for_test(recipe, {
			fluid: "true",
			layout: "control",
			loading: "true",
			size: "md",
			variant,
		});

		expect(resolved.slots.root.base).toEqual({
			background: "blue",
			display: "inline-flex",
			inlineSize: "100%",
			minHeight: "2.5rem",
			minWidth: "8rem",
		});
		expect(resolved.slots.content.base).toEqual({
			display: "inline-flex",
			opacity: 0,
		});
	});

	it("merges recipe styles without mutating inputs", () => {
		const base = {
			color: "black",
		};
		const override = {
			color: "white",
			padding: "1rem",
		};

		expect(mergeRecipeStyles(base, undefined, override)).toEqual({
			color: "white",
			padding: "1rem",
		});
		expect(base).toEqual({
			color: "black",
		});
	});
});

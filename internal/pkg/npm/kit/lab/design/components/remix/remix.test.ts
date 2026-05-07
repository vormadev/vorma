// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import * as remixSelect from "remix/ui/select";
import { render } from "remix/ui/test";
import { beforeEach, describe, expect, it } from "vitest";
import {
	createRecipe,
	createRecipeStyleSystem,
	type GeneratedSystem,
	type RecipeStyleDefinitions,
	type SingleRecipeStyleInput,
} from "../../core/core.ts";
import {
	componentAnatomyAttrs,
	createAspectRatio,
	createBadge,
	createBox,
	createButton,
	createCallout,
	createChip,
	createCodeBlock,
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
	createDescriptionList,
	createDescriptionListItem,
	createDialog,
	createEmptyState,
	createFieldParts,
	createGrid,
	createIcon,
	createInput,
	createList,
	createListItem,
	createPopover,
	createProgress,
	createRangeField,
	createRecipeStyle,
	createResponsiveRecipeStyle,
	createSelect,
	createSelectableTile,
	createSeparator,
	createSpinner,
	createStack,
	createStat,
	createSurface,
	createTable,
	createTableBody,
	createTableCell,
	createTableHead,
	createTableHeaderCell,
	createTableRow,
	createText,
	popoverAnatomy,
	type CodeBlockStyleSystem,
	type DialogStyleSystem,
	type FieldStyleSystem,
	type PopoverStyleSystem,
	type RangeFieldStyleSystem,
	type SelectStyleSystem,
} from "./remix.ts";

describe("design components remix", () => {
	beforeEach(() => {
		Object.defineProperty(document, "adoptedStyleSheets", {
			configurable: true,
			value: [],
			writable: true,
		});
		Object.defineProperty(HTMLDialogElement.prototype, "show", {
			configurable: true,
			value: function show(this: HTMLDialogElement): void {
				this.open = true;
			},
		});
		Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
			configurable: true,
			value: function showModal(this: HTMLDialogElement): void {
				this.open = true;
			},
		});
		Object.defineProperty(HTMLDialogElement.prototype, "close", {
			configurable: true,
			value: function close(this: HTMLDialogElement): void {
				if (!this.open) {
					return;
				}
				this.open = false;
				this.dispatchEvent(new Event("close"));
			},
		});
	});

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

	it("emits component recipe styles without cascade layers", () => {
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
		const style_text =
			document.querySelector("style[data-vorma-component-style]")
				?.textContent ?? "";

		expect(result.$("button")?.className).toContain("vmxc-");
		expect(style_text).toContain("color: white;");
		expect(style_text).not.toContain("@layer");

		result.cleanup();
	});

	it("creates a Remix button from a Vorma recipe", () => {
		const button_recipe = {
			defaultVariants: {
				size: "md",
				variant: "primary",
			},
			slots: {
				content: {
					base: {
						display: "inline-flex",
					},
				},
				loadingIndicator: {
					base: {
						blockSize: "1em",
						inlineSize: "1em",
					},
					conditions: {
						reducedMotion: {
							animation: "none",
						},
					},
				},
				loadingIndicatorFrame: {
					base: {
						position: "absolute",
					},
				},
				root: {
					base: {
						alignItems: "center",
						display: "inline-flex",
					},
					conditions: {
						hover: {
							background: "blue",
						},
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
								minHeight: "2.5rem",
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
		const base_system = {
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
		const style_system = createRecipeStyleSystem(base_system, {
			button: button_recipe,
		});
		const Button = createButton(style_system);
		const result = render(
			createElement(
				Button,
				{
					at: {
						md: {
							size: "md",
						},
					},
					loading: true,
					loadingLabel: "Saving",
				},
				"Save",
			),
		);

		const button = result.$("button");
		expect(button?.getAttribute("aria-busy")).toBe("true");
		expect(button?.getAttribute("aria-label")).toBe("Saving");
		expect(button?.getAttribute("type")).toBe("button");
		expect(button?.hasAttribute("disabled")).toBe(true);
		expect(button?.textContent).toBe("Save");

		result.cleanup();
	});

	it("creates a Remix code block with a summary slot", () => {
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
				summary: {
					base: {
						fontWeight: 600,
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
					preProps: {
						"data-pre": "example",
					},
					summary: "Install",
					summaryProps: {
						"data-summary": "example",
					},
				},
				"pnpm add vorma",
			),
		);

		const captions = result.$$("figcaption");
		expect(captions).toHaveLength(2);
		expect(captions[0]?.textContent).toBe("Install");
		expect(captions[0]?.getAttribute("data-summary")).toBe("example");
		expect(captions[1]?.textContent).toBe("Use it anywhere");
		expect(result.$("pre")?.getAttribute("data-pre")).toBe("example");
		expect(result.$("code")?.getAttribute("data-code")).toBe("example");
		expect(result.$("code")?.textContent).toBe("pnpm add vorma");

		result.cleanup();
	});

	it("renders single-host primitives with semantic hosts and anatomy", () => {
		const base_system = {
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
		} as const;
		const badge_recipe = {
			slots: {
				root: {},
			},
			variants: {
				size: {
					sm: {
						root: {},
					},
				},
				tone: {
					info: {
						root: {},
					},
				},
			},
		} as const;
		const box_recipe = {
			slots: {
				root: {},
			},
			variants: {
				layout: {
					panel: {
						root: {},
					},
				},
			},
		} as const;
		const chip_recipe = {
			slots: {
				root: {},
			},
			variants: {
				selected: {
					false: {
						root: {},
					},
					true: {
						root: {},
					},
				},
				size: {
					md: {
						root: {},
					},
				},
				variant: {
					neutral: {
						root: {},
					},
				},
			},
		} as const;
		const grid_recipe = {
			slots: {
				root: {},
			},
			variants: {
				columns: {
					two: {
						root: {},
					},
				},
				gap: {
					md: {
						root: {},
					},
				},
				layout: {
					dashboard: {
						root: {},
					},
				},
			},
		} as const;
		const icon_recipe = {
			slots: {
				root: {},
			},
			variants: {
				layout: {
					inline: {
						root: {},
					},
				},
				size: {
					md: {
						root: {},
					},
				},
				tone: {
					neutral: {
						root: {},
					},
				},
			},
		} as const;
		const input_recipe = {
			slots: {
				root: {},
			},
			variants: {
				size: {
					md: {
						root: {},
					},
				},
				variant: {
					default: {
						root: {},
					},
				},
			},
		} as const;
		const selectable_tile_recipe = {
			slots: {
				root: {},
			},
			variants: {
				density: {
					compact: {
						root: {},
					},
				},
				selected: {
					false: {
						root: {},
					},
					true: {
						root: {},
					},
				},
				variant: {
					card: {
						root: {},
					},
				},
			},
		} as const;
		const spinner_recipe = {
			slots: {
				root: {},
			},
			variants: {
				size: {
					sm: {
						root: {},
					},
				},
				tone: {
					neutral: {
						root: {},
					},
				},
			},
		} as const;
		const surface_recipe = {
			slots: {
				root: {},
			},
			variants: {
				density: {
					cozy: {
						root: {},
					},
				},
				layout: {
					panel: {
						root: {},
					},
				},
				variant: {
					raised: {
						root: {},
					},
				},
			},
		} as const;
		const text_recipe = {
			slots: {
				root: {},
			},
			variants: {
				tone: {
					muted: {
						root: {},
					},
				},
				variant: {
					heading: {
						root: {},
					},
				},
			},
		} as const;
		const Badge = createBadge(
			{
				...base_system,
				token: {
					recipe: {
						badge: badge_recipe,
					},
				},
			},
			{ defaultSize: "sm", defaultTone: "info" },
		);
		const Box = createBox({
			...base_system,
			token: {
				recipe: {
					box: box_recipe,
				},
			},
		});
		const Chip = createChip(
			{
				...base_system,
				token: {
					recipe: {
						chip: chip_recipe,
					},
				},
			},
			{ defaultSize: "md", defaultVariant: "neutral" },
		);
		const Grid = createGrid({
			...base_system,
			token: {
				recipe: {
					grid: grid_recipe,
				},
			},
		});
		const Icon = createIcon({
			...base_system,
			token: {
				recipe: {
					icon: icon_recipe,
				},
			},
		});
		const Input = createInput(
			{
				...base_system,
				token: {
					recipe: {
						input: input_recipe,
					},
				},
			},
			{ defaultSize: "md", defaultVariant: "default" },
		);
		const SelectableTile = createSelectableTile(
			{
				...base_system,
				token: {
					recipe: {
						selectableTile: selectable_tile_recipe,
					},
				},
			},
			{ defaultDensity: "compact", defaultVariant: "card" },
		);
		const Separator = createSeparator({
			...base_system,
			token: {
				recipe: {
					separator: {
						slots: {
							root: {},
						},
					},
				},
			},
		});
		const Spinner = createSpinner(
			{
				...base_system,
				token: {
					recipe: {
						spinner: spinner_recipe,
					},
				},
			},
			{ defaultSize: "sm", defaultTone: "neutral" },
		);
		const Surface = createSurface(
			{
				...base_system,
				token: {
					recipe: {
						surface: surface_recipe,
					},
				},
			},
			{ defaultDensity: "cozy", defaultVariant: "raised" },
		);
		const Text = createText({
			...base_system,
			token: {
				recipe: {
					text: text_recipe,
				},
			},
		});
		const AspectRatio = createAspectRatio(base_system);
		const result = render(
			createElement(
				"div",
				{},
				createElement(Badge, { "data-testid": "badge" }, "Beta"),
				createElement(
					Box,
					{
						as: "section",
						"data-testid": "box",
						layout: "panel",
					},
					"Box",
				),
				createElement(
					Chip,
					{ "data-testid": "chip", selected: true },
					"Chip",
				),
				createElement(
					Grid,
					{
						as: "section",
						columns: "two",
						"data-testid": "grid",
						gap: "md",
						layout: "dashboard",
					},
					"Grid",
				),
				createElement(Icon, { "data-testid": "icon" }, "*"),
				createElement(Icon, {
					"aria-label": "Search",
					"data-testid": "named-icon",
				}),
				createElement(Input, {
					"data-testid": "input",
					placeholder: "Name",
				}),
				createElement(
					SelectableTile,
					{ "data-testid": "tile", selected: true },
					"Tile",
				),
				createElement(Separator, { "data-testid": "separator" }),
				createElement(Spinner, {
					"data-testid": "spinner",
					idle: true,
				}),
				createElement(
					Surface,
					{
						"data-testid": "surface",
						layout: "panel",
					},
					"Surface",
				),
				createElement(
					Text,
					{
						as: "h2",
						"data-testid": "text",
						tone: "muted",
						variant: "heading",
					},
					"Heading",
				),
				createElement(AspectRatio, {
					"data-testid": "aspect",
					ratio: "16 / 9",
				}),
			),
		);

		const badge = result.$("[data-testid='badge']");
		const box = result.$("[data-testid='box']");
		const chip = result.$("[data-testid='chip']");
		const grid = result.$("[data-testid='grid']");
		const icon = result.$("[data-testid='icon']");
		const named_icon = result.$("[data-testid='named-icon']");
		const input = result.$("[data-testid='input']");
		const separator = result.$("[data-testid='separator']");
		const spinner = result.$("[data-testid='spinner']");
		const surface = result.$("[data-testid='surface']");
		const text = result.$("[data-testid='text']");
		expect(badge?.tagName).toBe("SPAN");
		expect(badge?.getAttribute(componentAnatomyAttrs.scope)).toBe("badge");
		expect(box?.tagName).toBe("SECTION");
		expect(box?.hasAttribute("layout")).toBe(false);
		expect(chip?.getAttribute("aria-pressed")).toBe("true");
		expect(chip?.getAttribute("data-selected")).toBe("true");
		expect(chip?.getAttribute("type")).toBe("button");
		expect(grid?.tagName).toBe("SECTION");
		expect(grid?.hasAttribute("columns")).toBe(false);
		expect(icon?.getAttribute("aria-hidden")).toBe("true");
		expect(named_icon?.getAttribute("aria-hidden")).toBeNull();
		expect(input?.tagName).toBe("INPUT");
		expect(input?.getAttribute("placeholder")).toBe("Name");
		expect(input?.getAttribute(componentAnatomyAttrs.scope)).toBe("input");
		expect(
			result.$("[data-testid='tile']")?.getAttribute("aria-pressed"),
		).toBe("true");
		expect(separator?.tagName).toBe("HR");
		expect(spinner?.getAttribute("data-idle")).toBe("true");
		expect(surface?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"surface",
		);
		expect(text?.tagName).toBe("H2");
		expect(
			result
				.$("[data-testid='aspect']")
				?.getAttribute(componentAnatomyAttrs.scope),
		).toBe("aspectRatio");

		result.cleanup();
	});

	it("renders content primitives with named slot ownership", () => {
		const callout_recipe = {
			slots: {
				content: {},
				icon: {},
				root: {},
			},
			variants: {
				tone: {
					info: {
						root: {},
					},
				},
			},
		} as const;
		const description_list_recipe = {
			slots: {
				description: {},
				item: {},
				root: {},
				term: {},
			},
			variants: {
				descriptionTone: {
					muted: {
						description: {},
					},
				},
				layout: {
					stacked: {
						item: {},
					},
				},
				tone: {
					neutral: {
						term: {},
					},
				},
			},
		} as const;
		const empty_state_recipe = {
			slots: {
				body: {},
				indicator: {},
				root: {},
				title: {},
			},
		} as const;
		const list_recipe = {
			slots: {
				item: {},
				root: {},
			},
		} as const;
		const stat_recipe = {
			slots: {
				root: {},
				title: {},
				value: {},
			},
			variants: {
				valueTone: {
					positive: {
						value: {},
					},
				},
			},
		} as const;
		const Callout = createCallout({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					callout: callout_recipe,
				},
			},
			variablePrefix: "test",
		});
		const DescriptionList = createDescriptionList({
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
			token: {
				recipe: {
					descriptionList: description_list_recipe,
				},
			},
			variablePrefix: "test",
		});
		const DescriptionListItem = createDescriptionListItem({
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
			token: {
				recipe: {
					descriptionList: description_list_recipe,
				},
			},
			variablePrefix: "test",
		});
		const EmptyState = createEmptyState({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					emptyState: empty_state_recipe,
				},
			},
			variablePrefix: "test",
		});
		const List = createList({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					list: list_recipe,
				},
			},
			variablePrefix: "test",
		});
		const ListItem = createListItem({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					list: list_recipe,
				},
			},
			variablePrefix: "test",
		});
		const Stat = createStat({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					stat: stat_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				"div",
				{},
				createElement(
					Callout,
					{
						"data-testid": "callout",
						icon: "!",
						tone: "info",
					},
					"Heads up",
				),
				createElement(
					DescriptionList,
					{ "data-testid": "description-list" },
					createElement(
						DescriptionListItem,
						{
							descriptionTone: "muted",
							layout: "stacked",
							term: "Plan",
							tone: "neutral",
						},
						"Pro",
					),
				),
				createElement(
					EmptyState,
					{
						"data-testid": "empty-state",
						indicator: "*",
						title: "No projects",
					},
					"Create one to get started.",
				),
				createElement(
					List,
					{ as: "ol", "data-testid": "list" },
					createElement(ListItem, {}, "First"),
				),
				createElement(Stat, {
					"data-testid": "stat",
					title: "Revenue",
					value: "$100",
					valueTone: "positive",
				}),
			),
		);

		expect(
			result
				.$("[data-testid='callout']")
				?.getAttribute(componentAnatomyAttrs.scope),
		).toBe("callout");
		expect(
			result
				.$("[data-vorma-scope='callout'][data-vorma-part='icon']")
				?.getAttribute("aria-hidden"),
		).toBe("true");
		expect(result.$("[data-testid='description-list']")?.tagName).toBe(
			"DL",
		);
		expect(result.$("dt")?.textContent).toBe("Plan");
		expect(result.$("dd")?.textContent).toBe("Pro");
		expect(
			result
				.$(
					"[data-vorma-scope='emptyState'][data-vorma-part='indicator']",
				)
				?.getAttribute("aria-hidden"),
		).toBe("true");
		expect(
			result.$("[data-vorma-scope='emptyState'][data-vorma-part='title']")
				?.textContent,
		).toBe("No projects");
		expect(result.$("[data-testid='list']")?.tagName).toBe("OL");
		expect(result.$("li")?.textContent).toBe("First");
		expect(
			result.$("[data-vorma-scope='stat'][data-vorma-part='title']")
				?.textContent,
		).toBe("Revenue");
		expect(
			result.$("[data-vorma-scope='stat'][data-vorma-part='value']")
				?.textContent,
		).toBe("$100");

		result.cleanup();
	});

	it("renders table primitives with native header scope semantics", () => {
		const table_recipe = {
			slots: {
				body: {},
				cell: {},
				head: {},
				root: {},
				row: {},
			},
			variants: {
				cellRole: {
					body: {
						cell: {},
					},
					columnHeader: {
						cell: {},
					},
					rowHeader: {
						cell: {},
					},
				},
				cellVariant: {
					default: {
						cell: {},
					},
				},
				density: {
					compact: {
						root: {},
					},
				},
				layout: {
					data: {
						root: {},
					},
				},
				rowVariant: {
					striped: {
						row: {},
					},
				},
			},
		} as const;
		const style_system = {
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
			token: {
				recipe: {
					table: table_recipe,
				},
			},
			variablePrefix: "test",
		} as const;
		const Table = createTable(style_system);
		const TableHead = createTableHead(style_system);
		const TableBody = createTableBody(style_system);
		const TableRow = createTableRow(style_system);
		const TableCell = createTableCell(style_system);
		const TableHeaderCell = createTableHeaderCell(style_system);
		const result = render(
			createElement(
				Table,
				{
					density: "compact",
					layout: "data",
				},
				createElement(
					TableHead,
					{},
					createElement(
						TableRow,
						{ variant: "striped" },
						createElement(
							TableHeaderCell,
							{ variant: "default" },
							"Name",
						),
					),
				),
				createElement(
					TableBody,
					{},
					createElement(
						TableRow,
						{},
						createElement(
							TableHeaderCell,
							{ scope: "row", variant: "default" },
							"Ada",
						),
						createElement(TableCell, { variant: "default" }, "42"),
					),
				),
			),
		);

		const table = result.$("table");
		const column_header = result.$("thead th");
		const row_header = result.$("tbody th");
		const body_cell = result.$("td");
		expect(table?.getAttribute(componentAnatomyAttrs.scope)).toBe("table");
		expect(column_header?.getAttribute("scope")).toBe("col");
		expect(row_header?.getAttribute("scope")).toBe("row");
		expect(body_cell?.textContent).toBe("42");

		result.cleanup();
	});

	it("renders Progress with accessible value defaults", () => {
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
			slots: {
				root: {},
				segment: {},
			},
			variants: {
				tone: {
					info: {
						segment: {},
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
					progress: progress_recipe,
				},
			},
			variablePrefix: "test",
		} as const;
		const Progress = createProgress(style_system);
		const result = render(
			createElement(Progress, {
				"aria-label": "Upload progress",
				segments: [
					{
						label: "Uploaded",
						tone: "info",
						value: 25,
					},
					{
						label: "Processed",
						value: 30,
					},
				],
			}),
		);

		const root = result.$("[role='progressbar']");
		expect(root?.getAttribute("aria-valuemin")).toBe("0");
		expect(root?.getAttribute("aria-valuemax")).toBe("100");
		expect(root?.getAttribute("aria-valuenow")).toBe("55");
		expect(
			result.$$(
				"[data-vorma-scope='progress'][data-vorma-part='segment']",
			),
		).toHaveLength(2);
		expect(
			result
				.$("[data-vorma-scope='progress'][data-vorma-part='segment']")
				?.getAttribute("aria-label"),
		).toBe("Uploaded");
		result.cleanup();

		const node = Progress({} as never)({
			segments: [
				{
					value: -20,
				},
				{
					value: 150,
				},
			],
		}) as unknown as ProgressNode;
		const first_segment_style =
			node.props.children[0][0].props.mix[0].args[0];
		const second_segment_style =
			node.props.children[0][1].props.mix[0].args[0];
		expect(first_segment_style).toMatchObject({
			width: "0%",
		});
		expect(second_segment_style).toMatchObject({
			width: "100%",
		});
	});

	it("applies RangeField input props and mix to the range input", async () => {
		const range_field_recipe = {
			defaultVariants: {
				layout: "stacked",
			},
			slots: {
				input: {},
				label: {},
				progress: {},
				root: {
					base: {
						display: "grid",
					},
				},
				thumb: {},
				track: {},
				value: {},
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
					rangeField: range_field_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies RangeFieldStyleSystem<"light", typeof range_field_recipe>;
		const RangeField = createRangeField(style_system);
		let current_target_tag: string | undefined;
		let current_target_value: string | undefined;
		let root_current_target_tag: string | undefined;
		const result = render(
			createElement(RangeField, {
				displayValue: "25",
				id: "presale-range",
				label: "Presale raised",
				labelProps: {
					"data-slot-owner": "label",
				},
				max: "100",
				min: "0",
				mix: on<HTMLInputElement, "input">("input", (event) => {
					current_target_tag = event.currentTarget.tagName;
					current_target_value = event.currentTarget.value;
				}),
				rootProps: {
					"data-slot-owner": "root",
					mix: on<HTMLDivElement, "click">("click", (event) => {
						root_current_target_tag = event.currentTarget.tagName;
					}),
				},
				value: "25",
				valueProps: {
					"data-slot-owner": "value",
				},
			}),
		);

		const input = result.$("input") as HTMLInputElement | null;
		expect(input).not.toBeNull();
		if (!input) {
			throw new Error("Expected RangeField to render an input");
		}
		await result.act(() => {
			input.value = "40";
			input.dispatchEvent(new Event("input", { bubbles: true }));
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_value).toBe("40");
		const root = result.$("div");
		expect(root?.getAttribute("data-slot-owner")).toBe("root");
		if (!root) {
			throw new Error("Expected RangeField to render a root");
		}
		await result.act(() => {
			root.dispatchEvent(new Event("click", { bubbles: true }));
		});
		expect(root_current_target_tag).toBe("DIV");
		expect(result.$("label")?.getAttribute("data-slot-owner")).toBe(
			"label",
		);
		expect(result.$("span")?.getAttribute("data-slot-owner")).toBe("value");
		expect(input?.getAttribute(componentAnatomyAttrs.part)).toBe("input");

		result.cleanup();
	});

	it("maps RangeField pseudo targets onto the input host mix", () => {
		type StyleMixNode = {
			props: {
				children: readonly [
					unknown,
					{
						props: {
							mix: readonly [
								{
									args: readonly [Record<string, unknown>];
								},
							];
						};
					},
				];
			};
		};
		const range_field_recipe = {
			slots: {
				input: {},
				label: {},
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
				value: {},
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
					rangeField: range_field_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies RangeFieldStyleSystem<"light", typeof range_field_recipe>;
		const RangeField = createRangeField(style_system);
		const node = RangeField({} as never)({
			label: "Volume",
			value: "50",
		}) as unknown as StyleMixNode;
		const input_style = node.props.children[1].props.mix[0].args[0];

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
			},
			"&:focus-visible::-moz-range-thumb": {
				boxShadow: "0 0 0 2px blue",
			},
			"&:focus-visible::-webkit-slider-thumb": {
				boxShadow: "0 0 0 2px blue",
			},
		});
	});

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
				pre: {},
				root: {},
				summary: {
					base: {
						fontWeight: 600,
					},
				},
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
		expect(code_block_resolved.slots.summary.base.fontWeight).toBe(600);
	});

	it("creates field parts without requiring factory call order", () => {
		const field_recipe = {
			slots: {
				description: {
					base: {
						color: "gray",
					},
				},
				error: {
					base: {
						color: "red",
					},
				},
				label: {
					base: {
						color: "black",
					},
				},
				root: {
					base: {
						display: "grid",
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
					field: field_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies FieldStyleSystem<"light", typeof field_recipe>;
		const Field = createFieldParts(style_system);
		const result = render(
			createElement(
				Field.Root,
				{
					invalid: true,
				},
				createElement(Field.Label, {}, "Name"),
				createElement(Field.Error, { match: true }, "Required"),
			),
		);

		const root = result.$("div");
		const label = result.$("label");
		const error = result.$("p");
		expect(root?.getAttribute("data-invalid")).toBe("true");
		expect(label?.getAttribute("data-invalid")).toBe("true");
		expect(error?.textContent).toBe("Required");

		result.cleanup();
	});

	it("does not require dialog recipes to define an overlay slot", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				popup: {
					base: {
						background: "white",
					},
				},
				title: {},
				trigger: {},
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
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		expect(result.$("dialog")?.textContent).toBe("Panel");

		result.cleanup();
	});

	it("creates a dialog overlay backdrop and closes on outside click", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				overlay: {
					base: {
						background: "rgb(0 0 0 / 0.4)",
					},
					conditions: {
						open: {
							opacity: 1,
						},
					},
				},
				popup: {
					base: {
						background: "white",
					},
				},
				title: {},
				trigger: {},
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
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		const popup = result.$("dialog");
		if (!(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog popup host");
		}
		Object.defineProperty(popup, "getBoundingClientRect", {
			configurable: true,
			value: () => {
				return {
					bottom: 100,
					height: 90,
					left: 10,
					right: 100,
					top: 10,
					width: 90,
					x: 10,
					y: 10,
					toJSON: () => {
						return {};
					},
				};
			},
		});

		expect(popup.open).toBe(true);
		popup.dispatchEvent(
			new MouseEvent("click", {
				bubbles: true,
				clientX: 0,
				clientY: 0,
			}),
		);

		expect(open_values).toEqual([false]);

		result.cleanup();
	});

	it("respects Dialog closeOnOutsideClick", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				popup: {},
				title: {},
				trigger: {},
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
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					closeOnOutsideClick: false,
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		const popup = result.$("dialog");
		if (!(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog popup host");
		}
		Object.defineProperty(popup, "getBoundingClientRect", {
			configurable: true,
			value: () => {
				return {
					bottom: 100,
					height: 90,
					left: 10,
					right: 100,
					top: 10,
					width: 90,
					x: 10,
					y: 10,
					toJSON: () => {
						return {};
					},
				};
			},
		});

		popup.dispatchEvent(
			new MouseEvent("click", {
				bubbles: true,
				clientX: 0,
				clientY: 0,
			}),
		);

		expect(open_values).toEqual([]);
		expect(popup.open).toBe(true);

		result.cleanup();
	});

	it("creates Remix popover parts from lower-level popover mixins", () => {
		const popover_recipe = {
			defaultVariants: {
				size: "md",
			},
			slots: {
				content: {
					base: {
						padding: "0.75rem",
					},
				},
				surface: {
					base: {
						background: "white",
					},
					conditions: {
						closed: {
							pointerEvents: "none",
						},
						open: {
							opacity: 1,
						},
					},
				},
				trigger: {
					base: {
						display: "inline-flex",
					},
					conditions: {
						focusVisible: {
							outline: "2px solid blue",
						},
					},
				},
			},
			variants: {
				size: {
					md: {
						surface: {
							base: {
								minWidth: "12rem",
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
					popover: popover_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies PopoverStyleSystem<"light", typeof popover_recipe>;
		const Popover = createPopover(style_system);
		const result = render(
			createElement(
				Popover.Root,
				{},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(
					Popover.Surface,
					{},
					createElement(Popover.Content, {}, "Panel"),
				),
			),
		);

		const trigger = result.$("button");
		const surface = result.$("div[popover='manual']");
		expect(trigger?.getAttribute("aria-expanded")).toBe("false");
		expect(trigger?.getAttribute(componentAnatomyAttrs.part)).toBe(
			popoverAnatomy.parts.trigger,
		);
		expect(trigger?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			popoverAnatomy.scope,
		);
		expect(surface?.getAttribute(componentAnatomyAttrs.part)).toBe(
			popoverAnatomy.parts.surface,
		);
		expect(surface?.textContent).toBe("Panel");

		result.cleanup();
	});

	it("creates a Remix select popup with its required popover context", () => {
		const select_recipe = {
			slots: {
				list: {},
				option: {},
				popup: {},
				trigger: {},
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
					select: select_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SelectStyleSystem<"light", typeof select_recipe>;
		const Select = createSelect(style_system);
		const result = render(
			createElement(
				Select.Root,
				{
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(Select.Option, {
						label: "Light",
						value: "light",
					}),
				),
			),
		);

		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		const option = result.$("[role='option']");
		expect(trigger?.getAttribute("aria-expanded")).toBe("false");
		expect(popup?.getAttribute(componentAnatomyAttrs.part)).toBe("popup");
		expect(option?.getAttribute(componentAnatomyAttrs.part)).toBe("option");

		result.cleanup();
	});

	it("forwards value-change close events through Select hosts", () => {
		const select_recipe = {
			slots: {
				list: {},
				option: {},
				popup: {},
				trigger: {},
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
					select: select_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SelectStyleSystem<"light", typeof select_recipe>;
		const Select = createSelect(style_system);
		const close_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChangeClose: (value: string | null) => {
						close_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(Select.Option, {
						label: "Light",
						value: "light",
					}),
				),
			),
		);

		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		if (!trigger || !popup) {
			throw new Error("Expected Select trigger and popup hosts");
		}
		trigger.dispatchEvent(
			new remixSelect.SelectChangeEvent({
				label: "Light",
				optionId: "option-light",
				value: "light",
			}),
		);
		popup.dispatchEvent(
			new remixSelect.SelectChangeEvent({
				label: "System",
				optionId: "option-system",
				value: "system",
			}),
		);

		expect(close_values).toEqual(["light", "system"]);

		result.cleanup();
	});

	it("emits Select onValueChange immediately from option click", () => {
		const select_recipe = {
			slots: {
				list: {},
				option: {},
				popup: {},
				trigger: {},
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
					select: select_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SelectStyleSystem<"light", typeof select_recipe>;
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const close_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					onValueChangeClose: (value: string | null) => {
						close_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(Select.Option, {
						label: "Light",
						value: "light",
					}),
				),
			),
		);

		const option = result.$("[role='option']");
		const trigger = result.$("button");
		if (!option || !trigger) {
			throw new Error("Expected Select option and trigger hosts");
		}
		option.dispatchEvent(new MouseEvent("click", { bubbles: true }));

		expect(changed_values).toEqual(["light"]);
		expect(close_values).toEqual([]);

		trigger.dispatchEvent(
			new remixSelect.SelectChangeEvent({
				label: "Light",
				optionId: "option-light",
				value: "light",
			}),
		);

		expect(changed_values).toEqual(["light"]);
		expect(close_values).toEqual(["light"]);

		result.cleanup();
	});

	it("does not emit Select immediate value changes for disabled options", () => {
		const select_recipe = {
			slots: {
				list: {},
				option: {},
				popup: {},
				trigger: {},
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
					select: select_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SelectStyleSystem<"light", typeof select_recipe>;
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(Select.Option, {
						disabled: true,
						label: "Light",
						value: "light",
					}),
				),
			),
		);

		const option = result.$("[role='option']");
		if (!option) {
			throw new Error("Expected Select option host");
		}
		option.dispatchEvent(new MouseEvent("click", { bubbles: true }));

		expect(changed_values).toEqual([]);

		result.cleanup();
	});

	it("emits Select onValueChange immediately from keyboard selection", () => {
		const select_recipe = {
			slots: {
				list: {},
				option: {},
				popup: {},
				trigger: {},
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
					select: select_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SelectStyleSystem<"light", typeof select_recipe>;
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(Select.Option, {
						label: "Light",
						value: "light",
					}),
				),
			),
		);

		const list = result.$("[role='listbox']");
		const option = result.$("[role='option']");
		if (!list || !option || !option.id) {
			throw new Error("Expected Select list and option hosts");
		}
		list.setAttribute("aria-activedescendant", option.id);
		list.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));

		expect(changed_values).toEqual(["light"]);

		result.cleanup();
	});
});

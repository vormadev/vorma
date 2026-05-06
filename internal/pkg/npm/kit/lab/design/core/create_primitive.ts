import { createPalette, formatOKLCH, is_oklch, type OKLCH } from "./color.ts";
import { createCSSVariableReference } from "./css.ts";
import { format_px, format_rem } from "./math.ts";
import { compact_object } from "./object.ts";
import {
	is_token_spec,
	resolve_raw_record,
	resolve_raw_value,
	resolve_spec_record,
	resolve_token_spec,
} from "./resolve_token.ts";
import {
	primitive_token_keys,
	token_groups,
	type BorderCompositeInput,
	type LengthScaleInput,
	type ModeName,
	type ModularScaleInput,
	type NonEmptyReadonlyArray,
	type PrimitiveTokens,
	type RawTokenValue,
	type ShadowCompositeInput,
	type ShadowLayerInput,
	type SystemInput,
	type TextStyleInput,
	type TokenSpec,
	type TransitionCompositeInput,
	type TransitionInput,
} from "./types.ts";

/////////////////////////////////////////////////////////////////////
/////// Composite Values
/////////////////////////////////////////////////////////////////////

// Composite tokens keep authoring ergonomic while still compiling to ordinary token strings.
function resolve_composite_value(
	value: RawTokenValue | TokenSpec,
	variable_prefix: string,
): string | number {
	return resolve_token_spec(value, variable_prefix);
}

function resolve_border_composite(
	input: BorderCompositeInput,
	variable_prefix: string,
): string {
	return [
		resolve_composite_value(input.width, variable_prefix),
		resolve_composite_value(input.style, variable_prefix),
		resolve_composite_value(input.color, variable_prefix),
	].join(" ");
}

function resolve_shadow_layer(
	input: ShadowLayerInput,
	variable_prefix: string,
): string {
	return [
		resolve_composite_value(input.x, variable_prefix),
		resolve_composite_value(input.y, variable_prefix),
		resolve_composite_value(input.blur, variable_prefix),
		input.spread === undefined
			? undefined
			: resolve_composite_value(input.spread, variable_prefix),
		resolve_composite_value(input.color, variable_prefix),
	]
		.filter((value) => {
			return value !== undefined;
		})
		.join(" ");
}

function is_shadow_layer_input(input: object): input is ShadowLayerInput {
	return "x" in input && "y" in input && "blur" in input && "color" in input;
}

function resolve_shadow_composite(
	input: RawTokenValue | ShadowCompositeInput,
	variable_prefix: string,
): string | number {
	if (typeof input === "object" && input !== null && is_token_spec(input)) {
		return resolve_token_spec(input, variable_prefix);
	}

	if (typeof input === "object" && input !== null && !is_oklch(input)) {
		const layers = is_shadow_layer_input(input) ? [input] : input;

		return layers
			.map((layer) => {
				return resolve_shadow_layer(layer, variable_prefix);
			})
			.join(", ");
	}

	return resolve_raw_value(input);
}

function resolve_transition(
	input: TransitionInput,
	variable_prefix: string,
): string {
	return [
		resolve_composite_value(input.property, variable_prefix),
		resolve_composite_value(input.duration, variable_prefix),
		resolve_composite_value(input.easing, variable_prefix),
		input.delay === undefined
			? undefined
			: resolve_composite_value(input.delay, variable_prefix),
	]
		.filter((value) => {
			return value !== undefined;
		})
		.join(" ");
}

function is_transition_input(input: object): input is TransitionInput {
	return "property" in input && "duration" in input && "easing" in input;
}

function resolve_transition_composite(
	input: RawTokenValue | TransitionCompositeInput,
	variable_prefix: string,
): string | number {
	if (typeof input === "object" && input !== null && is_token_spec(input)) {
		return resolve_token_spec(input, variable_prefix);
	}

	if (typeof input === "object" && input !== null && !is_oklch(input)) {
		const transitions = is_transition_input(input) ? [input] : input;

		return transitions
			.map((transition) => {
				return resolve_transition(transition, variable_prefix);
			})
			.join(", ");
	}

	return resolve_raw_value(input);
}

/////////////////////////////////////////////////////////////////////
/////// Scales
/////////////////////////////////////////////////////////////////////

function format_length(
	px_value: number,
	base_font_size_px: number,
	unit: "px" | "rem",
): string {
	if (unit === "px") {
		return format_px(px_value);
	}

	return format_rem(px_value, base_font_size_px);
}

function create_required_length_scale<TKey extends string>(
	input: LengthScaleInput<TKey>,
): Record<TKey, string> {
	return Object.fromEntries(
		(Object.entries(input.multiplier) as [TKey, number][]).map(
			([key, multiplier]) => {
				return [
					key,
					format_length(
						input.basePx * multiplier,
						input.baseFontSizePx,
						input.unit,
					),
				];
			},
		),
	) as Record<TKey, string>;
}

function create_length_scale<TKey extends string>(
	input: LengthScaleInput<TKey> | undefined,
): Record<TKey, string> | undefined {
	if (!input) {
		return undefined;
	}

	return create_required_length_scale(input);
}

function create_modular_scale<TKey extends string>(
	input: ModularScaleInput<TKey> | undefined,
): Record<TKey, string> | undefined {
	if (!input) {
		return undefined;
	}

	return Object.fromEntries(
		(Object.entries(input.step) as [TKey, number][]).map(([key, step]) => {
			const px_value = input.basePx * input.ratio ** step;
			return [
				key,
				format_length(px_value, input.baseFontSizePx, input.unit),
			];
		}),
	) as Record<TKey, string>;
}

/////////////////////////////////////////////////////////////////////
/////// Typography
/////////////////////////////////////////////////////////////////////

type PrimitiveTypographyInput = {
	family?: Record<string, RawTokenValue>;
	[primitive_token_keys.letter_spacing]?: Record<string, RawTokenValue>;
	[primitive_token_keys.line_height]?: Record<string, RawTokenValue>;
	size?: ModularScaleInput;
	[primitive_token_keys.text_style]?: Record<string, TextStyleInput>;
	weight?: Record<string, RawTokenValue>;
};

function create_primitive_typography(
	input: PrimitiveTypographyInput | undefined,
	variable_prefix: string,
): PrimitiveTokens["typography"] | undefined {
	if (!input) {
		return undefined;
	}

	const letter_spacing = input[primitive_token_keys.letter_spacing];
	const line_height = input[primitive_token_keys.line_height];
	const text_styles = input[primitive_token_keys.text_style];
	const typography = compact_object({
		family: resolve_raw_record(input.family),
		[primitive_token_keys.letter_spacing]:
			resolve_raw_record(letter_spacing),
		[primitive_token_keys.line_height]: resolve_raw_record(line_height),
		size: create_modular_scale(input.size),
		weight: resolve_raw_record(input.weight),
	});

	return compact_object({
		...typography,
		[primitive_token_keys.text_style]: text_styles
			? Object.fromEntries(
					Object.entries(text_styles).map(([name, style]) => {
						const text_style = compact_object({
							[primitive_token_keys.font_family]:
								input.family && style.family in input.family
									? createCSSVariableReference(
											variable_prefix,
											[
												token_groups.primitive,
												"typography",
												"family",
												style.family,
											],
										)
									: undefined,
							[primitive_token_keys.font_size]:
								input.size && style.size in input.size.step
									? createCSSVariableReference(
											variable_prefix,
											[
												token_groups.primitive,
												"typography",
												"size",
												style.size,
											],
										)
									: undefined,
							[primitive_token_keys.font_weight]:
								input.weight && style.weight in input.weight
									? createCSSVariableReference(
											variable_prefix,
											[
												token_groups.primitive,
												"typography",
												"weight",
												style.weight,
											],
										)
									: undefined,
							[primitive_token_keys.letter_spacing]:
								letter_spacing &&
								style.letterSpacing in letter_spacing
									? createCSSVariableReference(
											variable_prefix,
											[
												token_groups.primitive,
												"typography",
												primitive_token_keys.letter_spacing,
												style.letterSpacing,
											],
										)
									: undefined,
							[primitive_token_keys.line_height]:
								line_height && style.lineHeight in line_height
									? createCSSVariableReference(
											variable_prefix,
											[
												token_groups.primitive,
												"typography",
												primitive_token_keys.line_height,
												style.lineHeight,
											],
										)
									: undefined,
						});

						return [name, text_style];
					}),
				)
			: undefined,
	}) as PrimitiveTokens["typography"];
}

/////////////////////////////////////////////////////////////////////
/////// Primitive Tokens
/////////////////////////////////////////////////////////////////////

export function create_primitive_tokens<
	TModes extends NonEmptyReadonlyArray<ModeName>,
	TColorSource extends string,
>(
	input: SystemInput<TModes, TColorSource>,
	mode: TModes[number],
): PrimitiveTokens | undefined {
	if (!input.primitive) {
		return undefined;
	}

	const variable_prefix = input.css.variablePrefix;
	const primitive = input.primitive;
	const color_input = primitive.color;
	const data_visualization =
		primitive[primitive_token_keys.data_visualization];
	const color = color_input
		? compact_object({
				palette: color_input.palette
					? Object.fromEntries(
							Object.entries(color_input.palette).map(
								([name, palette]) => {
									const source =
										color_input.source[palette.source]!;

									return [
										name,
										createPalette({
											chromaLimit: palette.chromaLimit,
											curve: palette.curve[mode],
											source,
										}),
									];
								},
							),
						)
					: undefined,
				source: Object.fromEntries(
					(
						Object.entries(color_input.source) as [string, OKLCH][]
					).map(([name, value]) => {
						return [name, formatOKLCH(value)];
					}),
				),
			})
		: undefined;
	const typography = create_primitive_typography(
		primitive.typography,
		variable_prefix,
	);

	return compact_object({
		border: primitive.border
			? compact_object({
					radius: create_length_scale(primitive.border.radius),
					shorthand: resolve_spec_record(
						primitive.border.shorthand,
						(value) => {
							return resolve_border_composite(
								value,
								variable_prefix,
							);
						},
					),
					style: resolve_raw_record(primitive.border.style),
					width: create_length_scale(primitive.border.width),
				})
			: undefined,
		color,
		content: primitive.content
			? compact_object({
					[primitive_token_keys.aspect_ratio]: resolve_raw_record(
						primitive.content[primitive_token_keys.aspect_ratio],
					),
					measure: resolve_raw_record(primitive.content.measure),
				})
			: undefined,
		[primitive_token_keys.data_visualization]: data_visualization
			? compact_object({
					color: resolve_raw_record(data_visualization.color),
					shape: resolve_raw_record(data_visualization.shape),
					stroke: resolve_raw_record(data_visualization.stroke),
				})
			: undefined,
		dimension: primitive.dimension
			? compact_object({
					[primitive_token_keys.aspect_ratio]: resolve_raw_record(
						primitive.dimension[primitive_token_keys.aspect_ratio],
					),
					measure: resolve_raw_record(primitive.dimension.measure),
					size: primitive.dimension.size
						? Object.fromEntries(
								Object.entries(primitive.dimension.size).map(
									([name, scale]) => {
										return [
											name,
											create_required_length_scale(scale),
										];
									},
								),
							)
						: undefined,
					space: create_length_scale(primitive.dimension.space),
				})
			: undefined,
		effect: primitive.effect
			? compact_object({
					blur: resolve_raw_record(primitive.effect.blur),
					opacity: resolve_raw_record(primitive.effect.opacity),
					shadow: resolve_spec_record(
						primitive.effect.shadow,
						(value) => {
							return resolve_shadow_composite(
								value,
								variable_prefix,
							);
						},
					),
				})
			: undefined,
		focus: primitive.focus
			? compact_object({
					ring: resolve_spec_record(primitive.focus.ring, (value) => {
						if (
							typeof value === "object" &&
							value !== null &&
							!is_oklch(value)
						) {
							return resolve_border_composite(
								value,
								variable_prefix,
							);
						}

						return resolve_raw_value(value);
					}),
				})
			: undefined,
		icon: primitive.icon
			? compact_object({
					asset: resolve_raw_record(primitive.icon.asset),
					size: create_length_scale(primitive.icon.size),
					[primitive_token_keys.stroke_width]: create_length_scale(
						primitive.icon[primitive_token_keys.stroke_width],
					),
				})
			: undefined,
		layout: primitive.layout
			? compact_object({
					breakpoint: resolve_raw_record(primitive.layout.breakpoint),
					container: resolve_raw_record(primitive.layout.container),
					grid: resolve_raw_record(primitive.layout.grid),
					layer: resolve_raw_record(primitive.layout.layer),
				})
			: undefined,
		motion: primitive.motion
			? compact_object({
					distance: resolve_raw_record(primitive.motion.distance),
					duration: resolve_raw_record(primitive.motion.duration),
					easing: resolve_raw_record(primitive.motion.easing),
					transition: resolve_spec_record(
						primitive.motion.transition,
						(value) => {
							return resolve_transition_composite(
								value,
								variable_prefix,
							);
						},
					),
				})
			: undefined,
		outline: primitive.outline
			? compact_object({
					offset: create_length_scale(primitive.outline.offset),
					shorthand: resolve_spec_record(
						primitive.outline.shorthand,
						(value) => {
							return resolve_border_composite(
								value,
								variable_prefix,
							);
						},
					),
					style: resolve_raw_record(primitive.outline.style),
					width: create_length_scale(primitive.outline.width),
				})
			: undefined,
		typography,
	});
}

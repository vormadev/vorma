import { round_number } from "./math.ts";

export type OKLCH = {
	chroma: number;
	hue: number;
	lightness: number;
};

export type PaletteStep = string;
export type PaletteCurve<TStep extends string = string> = Record<TStep, number>;
export type Palette<TStep extends string = string> = Record<TStep, string>;

export function formatOKLCH(color: OKLCH): string {
	return `oklch(${round_number(color.lightness)} ${round_number(
		color.chroma,
	)} ${round_number(color.hue)})`;
}

export function is_oklch(value: object): value is OKLCH {
	return "lightness" in value && "chroma" in value && "hue" in value;
}

export function createPalette<TStep extends string>(input: {
	chromaLimit: number;
	curve: {
		chroma: PaletteCurve<TStep>;
		lightness: PaletteCurve<TStep>;
	};
	source: OKLCH;
}): Palette<TStep> {
	const chroma = Math.min(input.source.chroma, input.chromaLimit);
	const steps = Object.keys(input.curve.lightness) as TStep[];

	return Object.fromEntries(
		steps.map((step) => {
			return [
				step,
				formatOKLCH({
					chroma: chroma * input.curve.chroma[step],
					hue: input.source.hue,
					lightness: input.curve.lightness[step],
				}),
			];
		}),
	) as Palette<TStep>;
}

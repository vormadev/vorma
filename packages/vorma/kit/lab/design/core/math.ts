export function round_number(value: number, precision = 4): number {
	const scale = 10 ** precision;
	return Math.round(value * scale) / scale;
}

export function format_px(value: number): string {
	return `${round_number(value)}px`;
}

export function format_rem(px_value: number, base_font_size_px: number): string {
	return `${round_number(px_value / base_font_size_px)}rem`;
}

export type RecipeStyleValue = string | number | null | undefined;
export type RecipeStyle = {
	readonly [property: string]: unknown;
};

const empty_recipe_style = Object.freeze({}) as RecipeStyle;

/////////////////////////////////////////////////////////////////////
/////// Style Objects
/////////////////////////////////////////////////////////////////////

function has_recipe_style_entries(style: object): boolean {
	return Object.keys(style).length > 0;
}

function is_mergeable_recipe_style(value: unknown): value is RecipeStyle {
	if (value === null || typeof value !== "object" || Array.isArray(value)) {
		return false;
	}

	return Object.getOwnPropertySymbols(value).length === 0;
}

function merge_recipe_style_value(current: unknown, value: unknown): unknown {
	if (is_mergeable_recipe_style(current) && is_mergeable_recipe_style(value)) {
		return mergeRecipeStyles(current, value);
	}

	return value;
}

function merge_recipe_style_into(
	merged: Record<string, unknown>,
	style: RecipeStyle,
): void {
	for (const [key, value] of Object.entries(style)) {
		merged[key] = merge_recipe_style_value(merged[key], value);
	}
}

export function mergeRecipeStyles<TStyle extends RecipeStyle>(
	...styles: ReadonlyArray<TStyle | undefined>
): TStyle {
	const non_empty_styles = styles.filter((style): style is TStyle => {
		return style !== undefined && has_recipe_style_entries(style);
	});

	if (non_empty_styles.length === 0) {
		return empty_recipe_style as TStyle;
	}

	if (non_empty_styles.length === 1) {
		return non_empty_styles[0]!;
	}

	const merged: Record<string, unknown> = {};
	for (const style of non_empty_styles) {
		merge_recipe_style_into(merged, style);
	}

	return merged as TStyle;
}

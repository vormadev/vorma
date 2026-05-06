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

	return Object.assign({}, ...non_empty_styles) as TStyle;
}

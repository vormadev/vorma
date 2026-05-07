import type { RecipeConditionSelectorMap } from "./recipe.ts";

export type CommonRecipeCondition =
	| "active"
	| "disabled"
	| "focusVisible"
	| "hover"
	| "idle"
	| "invalid"
	| "placeholder"
	| "reducedMotion"
	| "selected";

export const commonConditions = {
	active: "&:active",
	disabled: "&:disabled, &[aria-disabled='true']",
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	idle: "&[data-idle='true']",
	invalid: "&[aria-invalid='true'], &[data-invalid='true']",
	placeholder: "&::placeholder",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
	selected: "&[aria-selected='true'], &[data-selected='true']",
} as const satisfies RecipeConditionSelectorMap<CommonRecipeCondition>;

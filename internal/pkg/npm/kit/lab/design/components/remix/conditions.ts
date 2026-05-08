import { componentDataAttribute } from "./component-state.ts";
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
	disabled: `&:disabled, &[aria-disabled='true'], &[${componentDataAttribute.disabled}]`,
	focusVisible: "&:focus-visible",
	hover: "&:hover",
	idle: "&[data-idle='true']",
	invalid: `&[aria-invalid='true'], &[${componentDataAttribute.invalid}]`,
	placeholder: "&::placeholder",
	reducedMotion: "@media (prefers-reduced-motion: reduce)",
	selected: `&[aria-selected='true'], &[${componentDataAttribute.selected}]`,
} as const satisfies RecipeConditionSelectorMap<CommonRecipeCondition>;

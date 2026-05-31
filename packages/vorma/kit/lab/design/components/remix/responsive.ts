import {
	mergeRecipeStyles,
	type RecipeStyle,
	type ResolvedRecipeSlot,
} from "../../core/core.ts";
import { createRecipeStyle, type RecipeConditionSelectorMap } from "./recipe.ts";
import type { ComponentStyleSystem } from "./types.ts";

type MetadataBreakpoint<TMetadata> = TMetadata extends {
	breakpoint: Record<infer TBreakpoint, string | number>;
}
	? TBreakpoint & string
	: never;

export type ResponsiveProps<TProps extends object, TBreakpoint extends string> = {
	at?: Partial<Record<TBreakpoint, Partial<TProps>>>;
};

export type BreakpointForStyleSystem<TStyleSystem> =
	TStyleSystem extends ComponentStyleSystem<string, unknown, infer TMetadata>
		? MetadataBreakpoint<TMetadata>
		: never;

export function createResponsiveStyle<
	TProps extends object,
	TMode extends string,
	TToken,
	TMetadata extends {
		breakpoint?: Record<string, string | number>;
	},
>(input: {
	at: Partial<Record<string, Partial<TProps>>> | undefined;
	resolve: (props: Partial<TProps>) => RecipeStyle | undefined;
	styleSystem: ComponentStyleSystem<TMode, TToken, TMetadata>;
}): RecipeStyle | undefined {
	const breakpoints = input.styleSystem.metadata.breakpoint;
	if (!breakpoints || !input.at) {
		return undefined;
	}

	let style: RecipeStyle | undefined;
	for (const [name, props] of Object.entries(input.at)) {
		if (!props) {
			continue;
		}

		const breakpoint = breakpoints[name];
		if (breakpoint === undefined) {
			continue;
		}

		style = mergeRecipeStyles(style, {
			[`@media (min-width: ${breakpoint})`]: input.resolve(props),
		});
	}

	return style;
}

export function createResponsiveRecipeStyle<
	TProps extends object,
	TCondition extends string,
	TMode extends string,
	TToken,
	TMetadata extends {
		breakpoint?: Record<string, string | number>;
	},
>(input: {
	at: Partial<Record<string, Partial<TProps>>> | undefined;
	conditions?: RecipeConditionSelectorMap;
	resolve: (
		props: Partial<TProps>,
	) => ResolvedRecipeSlot<TCondition, RecipeStyle> | undefined;
	styleSystem: ComponentStyleSystem<TMode, TToken, TMetadata>;
}): RecipeStyle | undefined {
	const breakpoints = input.styleSystem.metadata.breakpoint;
	if (!breakpoints || !input.at) {
		return undefined;
	}

	let style: RecipeStyle | undefined;
	for (const [name, props] of Object.entries(input.at)) {
		if (!props) {
			continue;
		}

		const breakpoint = breakpoints[name];
		if (breakpoint === undefined) {
			continue;
		}

		const slot = input.resolve(props);
		if (!slot) {
			continue;
		}

		style = mergeRecipeStyles(style, {
			[`@media (min-width: ${breakpoint})`]: createRecipeStyle({
				conditions: input.conditions,
				slot,
			}),
		});
	}

	return style;
}

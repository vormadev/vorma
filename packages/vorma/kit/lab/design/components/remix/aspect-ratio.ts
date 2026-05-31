import { createElement, type Props, type RemixNode } from "remix/ui";
import { mergeRecipeStyles, type RecipeStyle } from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import { type BreakpointForStyleSystem, type ResponsiveProps } from "./responsive.ts";
import type { ComponentStyleSystem, RemixComponent } from "./types.ts";

export type AspectRatioOverflow = "auto" | "clip" | "hidden" | "visible";

export type AspectRatioStyleProps = {
	height?: number | string;
	minHeight?: number | string;
	overflow?: AspectRatioOverflow;
	ratio?: string;
	width?: number | string;
};

export type AspectRatioProps<TBreakpoint extends string = string> = Omit<
	Props<"div">,
	"style"
> &
	AspectRatioStyleProps &
	ResponsiveProps<AspectRatioStyleProps, TBreakpoint> & {
		style?: never;
	};

const root_style = {
	boxSizing: "border-box",
	display: "block",
	minWidth: 0,
	width: "100%",
} as const satisfies RecipeStyle;

function style_for_props(props: Partial<AspectRatioStyleProps>): RecipeStyle {
	return {
		aspectRatio: props.ratio,
		height: props.height,
		minHeight: props.minHeight,
		overflow: props.overflow,
		width: props.width,
	};
}

const aspect_ratio_scope = "aspectRatio";

export function createAspectRatio<
	TMode extends string,
	TMetadata extends { breakpoint?: Record<string, string | number> },
>(
	style_system: ComponentStyleSystem<TMode, unknown, TMetadata>,
): RemixComponent<AspectRatioProps<BreakpointForStyleSystem<typeof style_system>>> {
	type TBreakpoint = BreakpointForStyleSystem<typeof style_system>;

	return () => {
		return (props: AspectRatioProps<TBreakpoint>): RemixNode => {
			const {
				at,
				children,
				height,
				minHeight,
				mix,
				overflow,
				ratio,
				width,
				...div_props
			} = props;
			const base_props = {
				height,
				minHeight,
				overflow,
				ratio,
				width,
			};
			const parts = createComponentStyleTargets({
				at,
				targets: {
					root: {
						host: "root",
						resolveSlot: (style_props) => {
							return {
								base: mergeRecipeStyles(
									root_style,
									style_for_props(style_props),
								),
								conditions: {},
							};
						},
					},
				},
				props: base_props,
				styleSystem: style_system,
			});

			return createElement(
				"div",
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(aspect_ratio_scope, "root"),
					mix: parts.hosts.root.mix,
					props: {
						...div_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

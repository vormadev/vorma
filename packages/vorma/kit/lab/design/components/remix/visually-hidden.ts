import { createElement, type Props, type RemixNode } from "remix/ui";
import { type RecipeStyle } from "../../core/core.ts";
import {
	createComponentAnatomyAttrs,
	createComponentSlotProps,
	createComponentStyleTargets,
} from "./component-style.ts";
import type { ComponentStyleSystem, RemixComponent } from "./types.ts";

export const visuallyHiddenDefaultElement = "span";
export const visuallyHiddenFocusableSelector = "&:not(:focus):not(:active)";
export const visuallyHiddenPart = {
	root: "root",
} as const;
export const visuallyHiddenScope = "visuallyHidden";
export const visuallyHiddenStyle = {
	border: 0,
	clip: "rect(0 0 0 0)",
	clipPath: "inset(50%)",
	height: "1px",
	margin: "-1px",
	overflow: "hidden",
	padding: 0,
	position: "absolute",
	whiteSpace: "nowrap",
	width: "1px",
} as const satisfies RecipeStyle;

export type VisuallyHiddenElement = keyof HTMLElementTagNameMap;

export type VisuallyHiddenProps<TElement extends VisuallyHiddenElement = "span"> = Omit<
	Props<TElement>,
	"style"
> & {
	as?: TElement;
	isFocusable?: boolean;
	style?: never;
};

function style_for_props(props: { isFocusable?: boolean }): RecipeStyle {
	if (props.isFocusable) {
		return {
			[visuallyHiddenFocusableSelector]: visuallyHiddenStyle,
		};
	}

	return visuallyHiddenStyle;
}

export function createVisuallyHidden<
	TMode extends string,
	TMetadata extends { breakpoint?: Record<string, string | number> },
>(
	style_system: ComponentStyleSystem<TMode, unknown, TMetadata>,
): RemixComponent<VisuallyHiddenProps> {
	return () => {
		return (props: VisuallyHiddenProps): RemixNode => {
			const {
				as = visuallyHiddenDefaultElement,
				children,
				isFocusable = false,
				mix,
				...host_props
			} = props;
			const parts = createComponentStyleTargets({
				props: {
					isFocusable,
				},
				styleSystem: style_system,
				targets: {
					root: {
						host: "root",
						resolveSlot: (style_props) => {
							return {
								base: style_for_props(style_props),
								conditions: {},
							};
						},
					},
				},
			});

			return createElement(
				as,
				createComponentSlotProps({
					attrs: createComponentAnatomyAttrs(
						visuallyHiddenScope,
						visuallyHiddenPart.root,
					),
					mix: parts.hosts.root.mix,
					props: {
						...host_props,
						mix,
					},
				}),
				children,
			);
		};
	};
}

import type { Handle, RemixNode } from "remix/ui";
import type {
	AnyGeneratedSystemMetadata,
	GeneratedSystem,
	RecipeStyle,
} from "../../core/core.ts";

export type ComponentStyle = RecipeStyle;

export type RemixComponent<
	TProps extends object = Record<string, unknown>,
	TContext = Record<string, never>,
> = (handle: Handle<TProps, TContext>) => (props: TProps) => RemixNode;

export type ComponentStyleSystem<
	TMode extends string = string,
	TToken = unknown,
	TMetadata extends AnyGeneratedSystemMetadata = AnyGeneratedSystemMetadata,
> = GeneratedSystem<TMode, TToken, TMetadata>;

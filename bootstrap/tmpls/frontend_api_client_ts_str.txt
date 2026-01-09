import {
	buildMutationURL,
	buildQueryURL,
	resolveBody,
	submit,
} from "vorma/client";
import {
	vormaAppConfig,
	type MutationOutput,
	type MutationPattern,
	type MutationProps,
	type QueryOutput,
	type QueryPattern,
	type QueryProps,
} from "./vorma.gen/index.ts";

export const api = { query, mutate };

async function query<P extends QueryPattern>(props: QueryProps<P>) {
	return await submit<QueryOutput<P>>(
		buildQueryURL(vormaAppConfig, props),
		{
			method: "GET",
			...props.requestInit,
		},
		props.options,
	);
}

async function mutate<P extends MutationPattern>(props: MutationProps<P>) {
	return await submit<MutationOutput<P>>(
		buildMutationURL(vormaAppConfig, props),
		{
			method: "POST",
			...props.requestInit,
			body: resolveBody(props),
		},
		props.options,
	);
}

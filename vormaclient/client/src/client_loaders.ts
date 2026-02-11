import { registerPattern } from "vorma/kit/matcher/register";
import {
	executeClientLoaders,
	type ClientLoadersResult,
	type PartialWaitFnJSON,
} from "./client_loader_execution.ts";
import { buildClientLoaderSnapshotFromGlobal } from "./client_loader_snapshot.ts";
import { findClientLoaderPartialMatches } from "./client_loader_partial_matches.ts";
import {
	deriveAndSetErrorState,
	setClientLoadersState,
} from "./client_loader_state.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export type { ClientLoadersResult } from "./client_loader_execution.ts";
export {
	deriveAndSetErrorState,
	setClientLoadersState,
} from "./client_loader_state.ts";

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await executeClientLoaders(
		buildClientLoaderSnapshotFromGlobal(),
		__vormaClientGlobal.get("buildID"),
		new AbortController().signal,
	);

	setClientLoadersState(clientLoadersResult);
	deriveAndSetErrorState();
}

export async function __registerClientLoaderPattern(
	pattern: string,
): Promise<void> {
	registerPattern(__vormaClientGlobal.get("patternRegistry"), pattern);
}

// This is needed because the matcher, by definition, will only
// match when you have a full path match. If the path you are
// testing is longer than the registered patterns, you will get
// no match, even if some registered patterns would potentially
// be in the parent segments. This fixes that.
export async function findPartialMatchesOnClient(pathname: string) {
	return findClientLoaderPartialMatches({
		pathname,
		patternRegistry: __vormaClientGlobal.get("patternRegistry"),
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap"),
	});
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<any>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

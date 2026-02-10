import { registerPattern } from "vorma/kit/matcher/register";
import { getEffectiveErrorData } from "./component_loader_error_data.ts";
import {
	executeClientLoaders,
	type ClientLoadersResult,
	type PartialWaitFnJSON,
} from "./client_loader_execution.ts";
import { buildClientLoaderSnapshotFromGlobal } from "./client_loader_snapshot.ts";
import { findClientLoaderPartialMatches } from "./client_loader_partial_matches.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export type { ClientLoadersResult } from "./client_loader_execution.ts";

export function setClientLoadersState(
	clr: ClientLoadersResult | undefined,
): void {
	if (clr) {
		__vormaClientGlobal.set("clientLoadersData", clr.data ?? []);
		__vormaClientGlobal.set(
			"outermostClientErrorIdx",
			clr.errorMessage ? clr.data.length - 1 : undefined,
		);
		__vormaClientGlobal.set("outermostClientError", clr.errorMessage);
	}
}

export function deriveAndSetErrorState(): void {
	const effectiveErrData = getEffectiveErrorData();
	__vormaClientGlobal.set("outermostErrorIdx", effectiveErrData.index);
	__vormaClientGlobal.set("outermostError", effectiveErrData.error);
}

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await runWaitFns(
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

async function runWaitFns(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal);
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<any>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

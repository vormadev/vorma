import { route_hrefs_share_document } from "./href.ts";
import type {
	CoreModel,
	CoreOperation,
	OperationID,
	OperationRight,
} from "./model.ts";

export type OperationGuardModel = {
	operations: Readonly<
		Record<OperationID, { rights: readonly OperationRight[] } | undefined>
	>;
	publication_owner_id: OperationID | null;
};

export type RouteOwnershipModel = Pick<
	CoreModel,
	"active_route_operation_id" | "browser" | "refresh"
>;

export type RouteOperation = Extract<
	CoreOperation,
	{ kind: "boot" | "navigation" | "popstate" | "route_revalidation" }
>;

export type RunningRefresh = Extract<CoreModel["refresh"], { kind: "running" }>;

export function is_route_operation(
	operation: CoreOperation | undefined,
): operation is RouteOperation {
	if (!operation) {
		return false;
	}
	return (
		operation.kind === "boot" ||
		operation.kind === "navigation" ||
		operation.kind === "popstate" ||
		operation.kind === "route_revalidation"
	);
}

export function can_publish_route(
	model: OperationGuardModel,
	operation_id: OperationID,
): boolean {
	const operation = model.operations[operation_id];
	return (
		model.publication_owner_id === operation_id &&
		operation?.rights.includes("publish_route") === true
	);
}

export function owns_route_completion(
	model: RouteOwnershipModel,
	operation: RouteOperation,
): boolean {
	if (operation.kind === "route_revalidation") {
		return (
			!!satisfiable_refresh_for_operation(model, operation) &&
			model.active_route_operation_id === null &&
			!!model.browser &&
			route_hrefs_share_document(operation.href, model.browser.href)
		);
	}
	return model.active_route_operation_id === operation.id;
}

export function satisfiable_refresh_for_operation(
	model: Pick<CoreModel, "refresh">,
	operation: CoreOperation,
): RunningRefresh | undefined {
	if (!operation.rights.includes("satisfy_refresh_demand")) {
		return undefined;
	}
	const refresh = model.refresh;
	if (refresh.kind !== "running" || refresh.operation_id !== operation.id) {
		return undefined;
	}
	return refresh;
}

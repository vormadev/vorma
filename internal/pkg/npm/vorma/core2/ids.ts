import type { OperationID, PublicCallID, SubmissionID } from "./model.ts";

const id_kind = {
	browser: "browser",
	operation: "operation",
	public: "public",
	submission: "submission",
} as const;

export type IDSource = {
	next_browser_key: () => string;
	next_operation_id: () => OperationID;
	next_public_call_id: () => PublicCallID;
	next_submission_id: () => SubmissionID;
};

export function create_id_source(prefix = "core2"): IDSource {
	let next = 0;

	function next_id(kind: string): string {
		next += 1;
		return `${prefix}:${kind}:${next}`;
	}

	return {
		next_browser_key: () => {
			return next_id(id_kind.browser);
		},
		next_operation_id: () => {
			return next_id(id_kind.operation);
		},
		next_public_call_id: () => {
			return next_id(id_kind.public);
		},
		next_submission_id: () => {
			return next_id(id_kind.submission);
		},
	};
}

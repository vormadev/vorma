/**
 * A success-or-string-error result — check `ok` to discriminate. String
 * errors (not `Error` objects) are the deliberate contract: this type is
 * for internal, expected-to-fail operations (parsing, validation) whose
 * failure is a plain message, not a stack-trace-bearing exception. Kit's
 * own modules (`jsonStringifyStable`, `create_client_core`'s boot
 * sequence) use this shape internally; it is exported because an app may
 * find the same shape useful for its own fallible operations.
 */
export type Result<T> = { ok: true; val: T } | { ok: false; err: string };

function ok<T>(val: T): Result<T> {
	return { ok: true, val };
}

function err<T extends string>(err: T): Result<any> {
	return { ok: false, err };
}

/** Constructors for {@link Result}: `R.ok(val)` / `R.err(message)`. */
export const R = { ok, err } as const;

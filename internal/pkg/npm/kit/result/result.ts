export type Result<T> = { ok: true; val: T } | { ok: false; err: string };

function ok<T>(val: T): Result<T> {
	return { ok: true, val };
}

function err<T extends string>(err: T): Result<any> {
	return { ok: false, err };
}

export const R = { ok, err } as const;

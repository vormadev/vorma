/** `JSON.stringify(obj, null, "\t")` — tab-indented pretty-printed JSON, for debug output/logging. */
export function prettyJson(obj: any): string {
	return JSON.stringify(obj, null, "\t");
}

export function prettyJson(obj: any): string {
	return JSON.stringify(obj, null, "\t");
}

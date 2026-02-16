export function prettyJSON(obj: any): string {
	return JSON.stringify(obj, null, 2);
}

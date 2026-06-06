export function escape_regex_literal(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

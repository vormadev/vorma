export function createElementFingerprint(element: Element): string {
	const attributes: Array<string> = [];
	for (let i = 0; i < element.attributes.length; i++) {
		const attr = element.attributes[i];
		if (!attr) {
			continue;
		}
		const value =
			element.hasAttribute(attr.name) && attr.value === ""
				? ""
				: attr.value;
		attributes.push(`${attr.name}="${value}"`);
	}
	attributes.sort();
	return `${element.tagName.toUpperCase()}|${attributes.join(",")}|${(element.innerHTML || "").trim()}`;
}

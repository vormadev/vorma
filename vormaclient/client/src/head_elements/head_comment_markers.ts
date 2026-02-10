export function getStartAndEndComments(type: "meta" | "rest"): {
	startComment: Comment | null;
	endComment: Comment | null;
} {
	const startMarker = `data-vorma="${type}-start"`;
	const endMarker = `data-vorma="${type}-end"`;
	const start = findComment(startMarker);
	const end = findComment(endMarker);
	return { startComment: start, endComment: end };
}

function findComment(matchingText: string): Comment | null {
	const walker = document.createTreeWalker(
		document.head,
		NodeFilter.SHOW_COMMENT,
		{
			acceptNode(node: Comment) {
				return node.nodeValue?.trim() === matchingText.trim()
					? NodeFilter.FILTER_ACCEPT
					: NodeFilter.FILTER_REJECT;
			},
		},
	);
	return walker.nextNode() as Comment | null;
}

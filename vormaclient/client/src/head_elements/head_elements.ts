import type { HeadEl } from "../vorma_ctx/vorma_ctx.ts";
import { buildDedupedElementsFromBlocks } from "./head_element_candidates.ts";
import { getStartAndEndComments } from "./head_comment_markers.ts";
import {
	placeReconciledHeadElements,
	reconcileHeadElements,
	removeStaleManagedNodes,
} from "./head_element_reconcile.ts";

export { getStartAndEndComments } from "./head_comment_markers.ts";

export function updateHeadEls(type: "meta" | "rest", blocks: Array<HeadEl>) {
	const { startComment, endComment } = getStartAndEndComments(type);
	if (!startComment || !endComment || !endComment.parentNode) {
		return;
	}
	const parent = endComment.parentNode;

	// Collect all current nodes between start and end comments
	const currentNodes: Array<Node> = [];
	let nodePtr = startComment.nextSibling;
	while (nodePtr != null && nodePtr !== endComment) {
		currentNodes.push(nodePtr);
		nodePtr = nodePtr.nextSibling;
	}
	const currentElements = currentNodes.filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);

	const newElements = buildDedupedElementsFromBlocks(blocks);
	const { finalElements, usedCurrentElements } = reconcileHeadElements(
		currentElements,
		newElements,
	);
	removeStaleManagedNodes(parent, currentNodes, usedCurrentElements);
	placeReconciledHeadElements(
		parent,
		startComment,
		endComment,
		finalElements,
		usedCurrentElements,
	);
}

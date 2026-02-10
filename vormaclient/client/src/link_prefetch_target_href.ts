export function buildPrefetchTargetHref(props: {
	relativeURL: string;
	search?: string;
	hash?: string;
}): string {
	const fullUrl = new URL(props.relativeURL, window.location.href);
	if (props.search !== undefined) fullUrl.search = props.search;
	if (props.hash !== undefined) fullUrl.hash = props.hash;
	return fullUrl.href;
}

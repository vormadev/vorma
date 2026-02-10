type HashCheckAnchorDetails =
	| {
			anchor: HTMLAnchorElement;
	  }
	| null
	| undefined;

export function isJustAHashChange(
	anchorDetails: HashCheckAnchorDetails,
): boolean {
	if (!anchorDetails) return false;

	const { pathname, search, hash } = new URL(
		anchorDetails.anchor.href,
		window.location.href,
	);

	return !!(
		hash &&
		pathname === window.location.pathname &&
		search === window.location.search
	);
}

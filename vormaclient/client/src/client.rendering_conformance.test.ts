import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	beginNavigation,
	getHistoryInstance,
	navigationStateManager,
	revalidate,
	vormaNavigate,
} from "./client";
import {
	addRouteChangeListener,
} from "./events.ts";
import {
	getStartAndEndComments,
} from "./head_elements/head_elements.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

function baseRouteData(overrides?: Record<string, any>) {
	return {
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		errorExportKeys: [],
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		deps: [],
		cssBundles: [],
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		...(overrides || {}),
	};
}

function ensureHeadMarkers(type: "meta" | "rest") {
	const existing = getStartAndEndComments(type);
	if (!existing.startComment) {
		document.head.appendChild(
			document.createComment(`data-vorma="${type}-start"`),
		);
	}
	if (!existing.endComment) {
		document.head.appendChild(
			document.createComment(`data-vorma="${type}-end"`),
		);
	}
}

function removeHeadMarkers(type: "meta" | "rest") {
	const comments = getStartAndEndComments(type);
	comments.startComment?.remove();
	comments.endComment?.remove();
}

function insertIntoSection(type: "meta" | "rest", element: Element) {
	const comments = getStartAndEndComments(type);
	if (!comments.endComment) {
		throw new Error(`Missing ${type} end comment`);
	}
	document.head.insertBefore(element, comments.endComment);
}

function getSectionElementTags(type: "meta" | "rest"): string[] {
	const comments = getStartAndEndComments(type);
	if (!comments.startComment || !comments.endComment) {
		return [];
	}

	const tags: string[] = [];
	let current = comments.startComment.nextSibling;
	while (current && current !== comments.endComment) {
		if (current.nodeType === Node.ELEMENT_NODE) {
			tags.push((current as Element).tagName.toLowerCase());
		}
		current = current.nextSibling;
	}
	return tags;
}

function hasTextNodesBetweenMarkers(type: "meta" | "rest"): boolean {
	const comments = getStartAndEndComments(type);
	if (!comments.startComment || !comments.endComment) {
		return false;
	}

	let current = comments.startComment.nextSibling;
	while (current && current !== comments.endComment) {
		if (current.nodeType === Node.TEXT_NODE) {
			return true;
		}
		current = current.nextSibling;
	}
	return false;
}

describeNavigationTestSuite(({ addListener }) => {
	describe("Rendering/history/head conformance", () => {
		it("FEC-REN-001_FE-REN-001_view_transitions_apply_for_navigate_like_paths_but_not_revalidation_or_prefetch", async () => {
			__vormaClientGlobal.set("useViewTransitions", true);
			const transitionSpy = vi.fn((callback?: () => void) => {
				callback?.();
				return { finished: Promise.resolve() };
			});
			Object.defineProperty(document, "startViewTransition", {
				value: transitionSpy,
				configurable: true,
			});

			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(createMockResponse(baseRouteData())),
			);

			await vormaNavigate("/ren-transition-navigate");
			await revalidate();
			await beginNavigation({
				href: "/ren-transition-prefetch",
				navigationType: "prefetch",
			}).promise;

			expect(transitionSpy).toHaveBeenCalledTimes(1);
		});

			it("FEC-REN-002_FE-REN-002_FE-REN-006_global_route_state_applies_before_route_change_dispatch_with_scroll_detail", async () => {
			const payload = baseRouteData({
				matchedPatterns: ["/users/:id", "/users/:id/profile"],
				loadersData: [{ user: 7 }, { profile: true }],
				params: { id: "7" },
				splatValues: [],
			});
			const observed: Array<{
				matchedPatterns: string[];
				params: Record<string, string>;
				detail: any;
			}> = [];

			addListener(addRouteChangeListener, (event) => {
				observed.push({
					matchedPatterns: [
						...(__vormaClientGlobal.get("matchedPatterns") || []),
					],
					params: {
						...(__vormaClientGlobal.get("params") || {}),
					},
					detail: event.detail,
				});
			});

			vi.mocked(fetch).mockResolvedValue(createMockResponse(payload));
			await vormaNavigate("/users/7/profile");

			expect(observed.length).toBeGreaterThan(0);
			expect(observed[0]?.matchedPatterns).toEqual(
				payload.matchedPatterns,
			);
				expect(observed[0]?.params).toEqual(payload.params);
				expect(observed[0]?.detail.__scrollState).toEqual({ x: 0, y: 0 });
			});

			it("FEC-REN-009_FE-REN-015_route_change_dispatches_before_head_reconciliation_effects", async () => {
				ensureHeadMarkers("meta");
				const sawMetaAtDispatch: boolean[] = [];
				addListener(addRouteChangeListener, () => {
					sawMetaAtDispatch.push(
						!!document.head.querySelector('meta[name="ren-ordering"]'),
					);
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(
						baseRouteData({
							metaHeadEls: [
								{
									tag: "meta",
									attributesKnownSafe: {
										name: "ren-ordering",
										content: "after-event",
									},
								},
							],
						}),
					),
				);

				await vormaNavigate("/ren-ordering");

				expect(sawMetaAtDispatch).toContain(false);
				expect(
					document.head.querySelector(
						'meta[name="ren-ordering"][content="after-event"]',
					),
				).not.toBeNull();
			});

			it("FEC-REN-003_FE-REN-003_history_push_replace_contract_matches_navigation_inputs", async () => {
			vi.mocked(fetch).mockImplementation(() =>
				Promise.resolve(createMockResponse(baseRouteData())),
			);

			const history = getHistoryInstance();
			const pushSpy = vi.spyOn(history, "push");
			const replaceSpy = vi.spyOn(history, "replace");

			await vormaNavigate("/ren-history-different");
			expect(pushSpy).toHaveBeenCalled();
			expect(replaceSpy).not.toHaveBeenCalled();

			pushSpy.mockClear();
			replaceSpy.mockClear();

			window.history.replaceState({}, "", "/ren-history-same");
			await vormaNavigate("/ren-history-same");
			expect(replaceSpy).toHaveBeenCalled();
			expect(pushSpy).not.toHaveBeenCalled();

			pushSpy.mockClear();
			replaceSpy.mockClear();

			await vormaNavigate("/ren-history-replace-opt", { replace: true });
			expect(replaceSpy).toHaveBeenCalled();
			expect(pushSpy).not.toHaveBeenCalled();
		});

		it("FEC-REN-004_FE-REN-004_browser_history_route_change_event_carries_saved_scroll_state", async () => {
			const routeChangeSpy = vi.fn();
			addListener(addRouteChangeListener, routeChangeSpy);

			vi.mocked(fetch).mockResolvedValue(createMockResponse(baseRouteData()));
			await navigationStateManager.navigate({
				href: "/ren-pop-target",
				navigationType: "browserHistory",
				scrollStateToRestore: { x: 11, y: 22 },
			});

			expect(routeChangeSpy).toHaveBeenCalled();
			const event = routeChangeSpy.mock.calls.at(-1)?.[0];
			expect(event?.detail?.__scrollState).toEqual({ x: 11, y: 22 });
		});

		it("FEC-REN-005_FE-REN-005_title_updates_decode_entities", async () => {
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse(
					baseRouteData({
						title: {
							dangerousInnerHTML:
								"Title &amp; Entity &#x27;Decode&#x27;",
						},
					}),
				),
			);

			await vormaNavigate("/ren-title");
			expect(document.title).toBe("Title & Entity 'Decode'");
		});

		it("FEC-REN-006_FE-REN-007_FE-REN-008_head_updates_are_conditional_and_missing_markers_are_safe_noop", async () => {
			ensureHeadMarkers("meta");
			ensureHeadMarkers("rest");

			const existingMeta = document.createElement("meta");
			existingMeta.setAttribute("name", "description");
			existingMeta.setAttribute("content", "keep-when-undefined");
			insertIntoSection("meta", existingMeta);

			const existingScript = document.createElement("script");
			existingScript.setAttribute("src", "/keep-when-undefined.js");
			insertIntoSection("rest", existingScript);

			vi.mocked(fetch)
				.mockResolvedValueOnce(
					createMockResponse(
						baseRouteData({
							metaHeadEls: undefined,
							restHeadEls: undefined,
						}),
					),
				)
				.mockResolvedValueOnce(
					createMockResponse(
						baseRouteData({
							metaHeadEls: [],
							restHeadEls: [],
						}),
					),
				)
				.mockResolvedValueOnce(
					createMockResponse(
						baseRouteData({
							metaHeadEls: [
								{
									tag: "meta",
									attributesKnownSafe: {
										name: "missing-marker-meta",
										content: "x",
									},
								},
							],
							restHeadEls: [
								{
									tag: "script",
									attributesKnownSafe: {
										src: "/missing-marker-rest.js",
									},
								},
							],
						}),
					),
				);

			await vormaNavigate("/ren-head-undefined");
			expect(
				document.head.querySelector(
					'meta[name="description"][content="keep-when-undefined"]',
				),
			).not.toBeNull();
			expect(
				document.head.querySelector(
					'script[src="/keep-when-undefined.js"]',
				),
			).not.toBeNull();

			await vormaNavigate("/ren-head-clear");
			expect(
				document.head.querySelector('meta[name="description"]'),
			).toBeNull();
			expect(
				document.head.querySelector(
					'script[src="/keep-when-undefined.js"]',
				),
			).toBeNull();

			removeHeadMarkers("meta");
			removeHeadMarkers("rest");

			const result = await navigationStateManager.navigate({
				href: "/ren-head-missing-markers",
				navigationType: "userNavigation",
			});

			expect(result.didNavigate).toBe(true);
			expect(
				document.head.querySelector('meta[name="missing-marker-meta"]'),
			).toBeNull();
			expect(
				document.head.querySelector(
					'script[src="/missing-marker-rest.js"]',
				),
			).toBeNull();
		});

		it("FEC-REN-007_FE-REN-009_FE-REN-010_head_reconcile_dedup_reorder_and_invalid_attributes_fail_loudly", async () => {
			ensureHeadMarkers("meta");

			const stale = document.createElement("meta");
			stale.setAttribute("name", "stale");
			stale.setAttribute("content", "remove-me");
			insertIntoSection("meta", stale);

			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(
					baseRouteData({
						metaHeadEls: [
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "description",
									content: "rendering-spec",
								},
							},
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "viewport",
									content: "width=device-width",
								},
							},
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "description",
									content: "rendering-spec",
								},
							},
						],
					}),
				),
			);

			await vormaNavigate("/ren-head-dedup");

			const metaTags = getSectionElementTags("meta");
			expect(metaTags).toEqual(["meta", "meta"]);
			expect(
				document.head.querySelectorAll(
					'meta[name="description"][content="rendering-spec"]',
				).length,
			).toBe(1);
			expect(
				document.head.querySelector('meta[name="stale"]'),
			).toBeNull();
			const orderedNames = Array.from(
				document.head.querySelectorAll("meta"),
			).map((el) => el.getAttribute("name"));
			expect(orderedNames).toEqual(["viewport", "description"]);

			const beforeFailureSnapshot = document.head.innerHTML;

			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(
					baseRouteData({
						metaHeadEls: [
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "invalid-attr",
									content: null as unknown as string,
								},
							},
						],
					}),
				),
			);

			const invalidResult = await navigationStateManager.navigate({
				href: "/ren-head-invalid-attr",
				navigationType: "userNavigation",
			});
			expect(invalidResult.didNavigate).toBe(false);
			expect(
				vi
					.mocked(console.error)
					.mock.calls.some((call) =>
						call.some((part) => String(part).includes("Panic")),
					),
			).toBe(true);
			expect(document.head.innerHTML).toBe(beforeFailureSnapshot);
		});

		it("FEC-REN-008_FE-REN-011_FE-REN-012_FE-REN-013_FE-REN-014_head_reconcile_ignores_missing_tags_cleans_text_nodes_applies_boolean_and_inner_html_and_reuses_fingerprint_equivalent_nodes", async () => {
			ensureHeadMarkers("meta");
			ensureHeadMarkers("rest");

			const stableMeta = document.createElement("meta");
			stableMeta.setAttribute("name", "description");
			stableMeta.setAttribute("content", "stable");
			insertIntoSection("meta", stableMeta);

			const metaComments = getStartAndEndComments("meta");
			if (!metaComments.endComment) {
				throw new Error("Missing meta end marker");
			}
			document.head.insertBefore(
				document.createTextNode("\n  "),
				metaComments.endComment,
			);

			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse(
					baseRouteData({
						metaHeadEls: [
							{
								attributesKnownSafe: {
									name: "ignored-no-tag",
									content: "ignored",
								},
							} as any,
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "description",
									content: "stable",
								},
							},
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "viewport",
									content: "width=device-width",
								},
							},
						],
						restHeadEls: [
							{
								tag: "script",
								attributesKnownSafe: { src: "/ren-async.js" },
								booleanAttributes: ["async"],
							},
							{
								tag: "style",
								dangerousInnerHTML: "body{--ren-head:1;}",
							},
						],
					}),
				),
			);

			await vormaNavigate("/ren-head-normalize");

			expect(
				document.head.querySelector('meta[name="description"]'),
			).toBe(stableMeta);
			expect(
				document.head.querySelector('meta[name="ignored-no-tag"]'),
			).toBeNull();
			expect(hasTextNodesBetweenMarkers("meta")).toBe(false);
			expect(getSectionElementTags("meta")).toEqual(["meta", "meta"]);

			const asyncScript = document.head.querySelector(
				'script[src="/ren-async.js"]',
			);
			expect(asyncScript).not.toBeNull();
			expect(asyncScript?.hasAttribute("async")).toBe(true);
			expect(asyncScript?.getAttribute("async")).toBe("");

			const styleEl = document.head.querySelector("style");
			expect(styleEl).not.toBeNull();
			expect(styleEl?.innerHTML).toBe("body{--ren-head:1;}");
		});
	});
});

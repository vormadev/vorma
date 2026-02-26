# Vorma Test Audit Scan Report (2026-02-25)

Generated via per-line scan across all test files.

## typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts

- lines: 635
- it(): 14
- expect(): 66
- flagged lines:

```
237:	it("does not leak unhandled rejections when stale prefetch success is dropped before wait phase", async () => {
252:			href: "/stale-prefetch-drop",
265:						matchedPatterns: ["/stale-prefetch-drop"],
266:						importURLs: ["/stale-prefetch-drop.module.js"],
346:	it("ignores late stale navigation responses with loader-data shape failures after newer navigation wins", async () => {
348:		const stalePattern = "/stale-loader-shape";
352:		patternToWaitFnMap[stalePattern] = async ({ serverDataPromise }) => {
358:		await api.__registerClientLoaderPattern(stalePattern);
360:		const staleDeferred = createDeferred<Response>();
375:					? staleDeferred.promise
382:				const staleNavigation = api.vormaNavigate(stalePattern);
397:				staleDeferred.resolve(
399:						matchedPatterns: [stalePattern],
400:						importURLs: ["/stale-loader-shape.js"],
407:				await staleNavigation;
423:	it("does not apply side effects from stale aborted navigation successes", async () => {
425:		const staleDeferred = createDeferred<Response>();
439:				return staleDeferred.promise as any;
458:			const staleNavigation = api.vormaNavigate("/stale-side-effects");
468:			staleDeferred.resolve(
474:						cssBundles: ["/stale-side-effects.css"],
478:							"X-Vorma-Build-Id": "stale-build-2",
483:			await staleNavigation;
492:				buildIDEvents.some((event) => event.newID === "stale-build-2"),
499:					'link[data-vorma-css-bundle="/stale-side-effects.css"]',
512:	it("does not let stale revalidation override later prefetched navigation", async () => {
566:	it("does not apply CSS bundles from stale revalidation responses", async () => {
615:				cssBundles: ["/stale-only.css"],
629:				'link[data-vorma-css-bundle="/stale-only.css"]',
```

## typescript/vorma/client/src/tests/contracts/client.events.contract.test.ts

- lines: 84
- it(): 3
- expect(): 4
- flagged lines: none

## typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts

- lines: 1733
- it(): 51
- expect(): 138
- flagged lines:

```
1592:	it("ignores stale older progressive manifest responses after a newer init", async () => {
1595:			"/stale-manifest-old/:id": 1,
1598:			"/stale-manifest-new/:id": 1,
1674:				"/stale-manifest-new/123",
1676:		).toBe("/stale-manifest-new/:id");
1680:				"/stale-manifest-old/123",
```

## typescript/vorma/client/src/tests/contracts/client.link_click.contract.test.ts

- lines: 393
- it(): 17
- expect(): 41
- flagged lines:

```
347:		const staleFetch = createDeferredFetchCall();
354:					return staleFetch.mock(url, init);
371:		staleFetch.deferred.resolve(createRouteDataResponse());
```

## typescript/vorma/client/src/tests/contracts/client.loading_and_focus.contract.test.ts

- lines: 1344
- it(): 37
- expect(): 121
- flagged lines:

```
1096:	it("revalidates on focus after staleTime has elapsed", async () => {
1102:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 50 });
1116:	it("resets focus stale-time window after successful navigation", async () => {
1128:		await api.vormaNavigate("/focus-stale-gate");
1136:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
1150:			// Focus revalidate should now run after stale window elapses.
1157:	it("does not reset focus stale-time window for hash-only programmatic navigations", async () => {
1163:		window.history.replaceState({}, "", "/focus-stale-hash-only");
1170:		await api.vormaNavigate("/focus-stale-hash-only#details");
1179:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 20 });
1191:	it("does not advance focus stale-time timestamp for aborted navigations", async () => {
1205:		const staleNavigation = api.vormaNavigate("/focus-stale-abort");
1211:		await staleNavigation;
1217:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 20 });
1224:			// Aborted navigation should not suppress staleness-triggered focus revalidate.
1231:	it("resets focus stale-time window after successful revalidation", async () => {
1251:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
1268:	it("does not advance focus stale-time timestamp for aborted revalidations", async () => {
1294:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 20 });
1317:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 0 });
1336:		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 0 });
```

## typescript/vorma/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts

- lines: 496
- it(): 15
- expect(): 30
- flagged lines: none

## typescript/vorma/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts

- lines: 662
- it(): 20
- expect(): 60
- flagged lines:

```
19:	it("aborts stale prefetch and revalidation work when a new user navigation starts", async () => {
24:			href: "/stale-prefetch",
49:	it("runs speculative client-loader code for superseded navigations but discards stale commits", async () => {
62:			return { stale: true };
66:				"/stale-client-loader": 1,
69:				"/stale-client-loader": speculativeClientLoaderWaitFn,
74:		await api.__registerClientLoaderPattern("/stale-client-loader");
77:		const staleNavigationPromise = api.vormaNavigate(
78:			"/stale-client-loader",
92:		await staleNavigationPromise;
200:				cssBundles: ["/late-stale.css"],
215:				'link[data-vorma-css-bundle="/late-stale.css"]',
286:		const staleNavigation = api.vormaNavigate("/clear-all-stale");
297:		await staleNavigation;
```

## typescript/vorma/client/src/tests/contracts/client.navigation_modes.contract.test.ts

- lines: 849
- it(): 25
- expect(): 105
- flagged lines:

```
554:	it("does not let a stale browser-history POP completion override a newer user navigation", async () => {
557:		const stalePOPFetch = createDeferred<Response>();
563:			() => stalePOPFetch.promise,
572:			const stalePOPNavigation = customHistoryListener({
575:					pathname: "/stale-pop-target",
579:					key: "stale-pop-key",
587:			stalePOPFetch.resolve(
594:							"X-Vorma-Build-Id": "stale-pop-build",
599:			await stalePOPNavigation;
617:	it("does not let stale redirect-follow-up navigation override a newer user navigation", async () => {
621:		const staleRedirectStart = api.vormaNavigate("/redirect-race-start");
656:		await staleRedirectStart;
668:	it("does not follow stale native redirects from an aborted user navigation", async () => {
670:		const staleNativeDeferred = createDeferred<Response>();
676:		const staleNativeRedirectResponse = createRouteDataResponse(
680:					"X-Vorma-Build-Id": "stale-native-nav-build",
684:		Object.defineProperty(staleNativeRedirectResponse, "redirected", {
688:		Object.defineProperty(staleNativeRedirectResponse, "url", {
689:			value: "http://localhost:3000/stale-native-nav-redirect",
694:			() => staleNativeDeferred.promise,
704:			const staleNavigation = api.vormaNavigate("/stale-native-start");
711:			staleNativeDeferred.resolve(staleNativeRedirectResponse);
712:			await staleNavigation;
730:	it("does not follow stale client-redirect headers from an aborted user navigation", async () => {
732:		const staleClientRedirectDeferred = createDeferred<Response>();
739:			() => staleClientRedirectDeferred.promise,
751:			const staleNavigation = api.vormaNavigate("/stale-soft-start");
758:			staleClientRedirectDeferred.resolve(
763:							"X-Client-Redirect": "/stale-soft-target",
764:							"X-Vorma-Build-Id": "stale-soft-nav-build",
769:			await staleNavigation;
787:	it("does not hard-reload from stale aborted user-navigation responses", async () => {
789:		const staleHardReloadDeferred = createDeferred<Response>();
798:			() => staleHardReloadDeferred.promise,
805:			const staleNavigation = api.vormaNavigate("/stale-hard-start");
819:			staleHardReloadDeferred.resolve(
824:							"X-Vorma-Reload": "/stale-hard-redirect",
825:							"X-Vorma-Build-Id": "stale-hard-nav-build",
830:			await staleNavigation;
836:			expect(locationHref).not.toContain("/stale-hard-redirect");
```

## typescript/vorma/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts

- lines: 421
- it(): 3
- expect(): 29
- flagged lines:

```
261:					// 4) Navigate away and then complete stale revalidation late:
```

## typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts

- lines: 886
- it(): 31
- expect(): 99
- flagged lines: none

## typescript/vorma/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts

- lines: 639
- it(): 17
- expect(): 66
- flagged lines:

```
345:	it("treats search-param location changes as stale revalidation boundaries", async () => {
346:		window.history.replaceState({}, "", "/search-stale?tab=a");
354:		window.history.replaceState({}, "", "/search-stale?tab=b");
364:		expect(window.location.pathname).toBe("/search-stale");
374:	it("ignores stale revalidation side effects after external location change", async () => {
393:				cssBundles: ["/stale-external.css"],
408:				'link[data-vorma-css-bundle="/stale-external.css"]',
418:	it("does not follow stale revalidation redirects after external location change", async () => {
420:		const staleRedirectDeferred = createDeferred<Response>();
426:			() => staleRedirectDeferred.promise,
438:			staleRedirectDeferred.resolve(
443:							"X-Client-Redirect": "/stale-redirect-target",
444:							"X-Vorma-Build-Id": "stale-soft-build",
467:	it("does not follow stale native revalidation redirects after external location change", async () => {
481:			value: "http://localhost:3000/stale-native-revalidate-redirect",
515:	it("does not perform hard reload from stale revalidation responses", async () => {
542:							"X-Vorma-Reload": "/stale-hard-reload",
543:							"X-Vorma-Build-Id": "stale-hard-build",
553:			expect(locationHref).not.toContain("/stale-hard-reload");
568:	it("does not update build ID from stale revalidation responses", async () => {
592:							"X-Vorma-Build-Id": "stale-revalidation-build",
```

## typescript/vorma/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts

- lines: 1430
- it(): 35
- expect(): 140
- flagged lines:

```
159:	it("drops stale redirect side effects when concurrent different-key submits overlap", async () => {
206:			// Resolve stale A redirect target late with unique build ID.
214:							"X-Vorma-Build-Id": "stale-redirect-a-build",
247:					(event) => event.newID === "stale-redirect-a-build",
260:	it("ignores stale submit-redirect side effects when a newer user navigation commits first", async () => {
312:							"X-Vorma-Build-Id": "submit-redirect-stale-build",
326:					(event) => event.newID === "submit-redirect-stale-build",
472:	it("ignores late stale deduped submit redirect responses after replacement submission wins", async () => {
491:						dedupeKey: "stale-submit",
500:						dedupeKey: "stale-submit",
516:						{ headers: { "X-Client-Redirect": "/stale-redirect" } },
539:	it("ignores late stale deduped submit hard-reload responses after replacement submission wins", async () => {
561:								dedupeKey: "stale-hard-reload",
570:								dedupeKey: "stale-hard-reload",
588:										"X-Vorma-Reload": "/stale-reload",
590:											"stale-reload-build",
607:			expect(locationHrefStub.getHref()).not.toContain("/stale-reload");
618:	it("does not trigger revalidation from a late stale deduped submit response", async () => {
640:				dedupeKey: "stale-submit-revalidate",
647:				dedupeKey: "stale-submit-revalidate",
660:		firstDeferred.resolve(createJSONResponse({ stale: true }));
678:	it("aborts stale deduped submits that are superseded during JSON parsing", async () => {
682:		const firstResponse = createJSONResponse({ stale: true });
701:				dedupeKey: "stale-json-parse",
712:				dedupeKey: "stale-json-parse",
720:		firstJSONDeferred.resolve({ stale: true });
733:	it("does not allow late stale deduped submits to change build ID", async () => {
753:					dedupeKey: "stale-build-id",
761:					dedupeKey: "stale-build-id",
777:					{ stale: true },
780:							"X-Vorma-Build-Id": "stale-build-id-999",
1057:		api.__vormaClientGlobal.set("loadersData", [{ stale: true }]);
```

## typescript/vorma/client/src/tests/contracts/client.utilities.contract.test.ts

- lines: 476
- it(): 23
- expect(): 62
- flagged lines: none

## typescript/vorma/client/src/tests/contracts/head_elements.contract.test.ts

- lines: 888
- it(): 28
- expect(): 97
- flagged lines:

```
117:			const staleDescriptionMeta = document.createElement("meta");
118:			staleDescriptionMeta.setAttribute("name", "description");
119:			staleDescriptionMeta.setAttribute("content", "stale");
132:			document.head.appendChild(staleDescriptionMeta);
```

## typescript/vorma/client/src/tests/contracts/vorma_ctx.contract.test.ts

- lines: 90
- it(): 6
- expect(): 7
- flagged lines: none

## typescript/vorma/client/src/tests/dist/npm_dist_adapters.test.ts

- lines: 183
- it(): 6
- expect(): 11
- flagged lines: none

## typescript/vorma/client/src/tests/dist/npm_dist_adapters_helpers_link_mocked.test.ts

- lines: 802
- it(): 15
- expect(): 59
- flagged lines: none

## typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet.test.ts

- lines: 1343
- it(): 15
- expect(): 102
- flagged lines: none

## typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_branches.test.ts

- lines: 777
- it(): 9
- expect(): 39
- flagged lines: none

## typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_runtime_state.test.ts

- lines: 3947
- it(): 43
- expect(): 147
- flagged lines:

```
3935:				"useClientLoaderData(routeProps) contract violated",
```

## typescript/vorma/client/src/tests/dist/npm_dist_client_runtime.test.ts

- lines: 47
- it(): 2
- expect(): 3
- flagged lines: none

## typescript/vorma/client/src/tests/unit/app_helpers.test.ts

- lines: 409
- it(): 21
- expect(): 38
- flagged lines: none

## typescript/vorma/client/src/tests/unit/begin_navigation_state_machine_internal.test.ts

- lines: 305
- it(): 6
- expect(): 6
- flagged lines:

```
49:	it("reuses matching active navigation and aborts stale lanes", () => {
55:		const stalePrefetch = createEntry({
56:			targetUrl: "http://localhost:3000/stale-prefetch",
60:		const staleRevalidation = createEntry({
61:			targetUrl: "http://localhost:3000/stale-revalidation",
77:				revalidation: staleRevalidation,
78:				prefetch: new Map([[stalePrefetch.targetUrl, stalePrefetch]]),
87:					key: stalePrefetch.targetUrl,
88:					entry: stalePrefetch,
92:					entry: staleRevalidation,
112:		const staleActive = createEntry({
113:			targetUrl: "http://localhost:3000/stale-active",
122:		const stalePrefetch = createEntry({
123:			targetUrl: "http://localhost:3000/stale-prefetch",
127:		const staleRevalidation = createEntry({
128:			targetUrl: "http://localhost:3000/stale-revalidation",
143:				active: staleActive,
144:				revalidation: staleRevalidation,
147:					[stalePrefetch.targetUrl, stalePrefetch],
157:					entry: staleActive,
161:					key: stalePrefetch.targetUrl,
162:					entry: stalePrefetch,
166:					entry: staleRevalidation,
271:	it("aborts stale revalidation lane before creating a new revalidation", () => {
272:		const staleRevalidation = createEntry({
286:				revalidation: staleRevalidation,
296:					entry: staleRevalidation,
```

## typescript/vorma/client/src/tests/unit/client_runtime_initialization.test.ts

- lines: 82
- it(): 4
- expect(): 12
- flagged lines: none

## typescript/vorma/client/src/tests/unit/events_platform.test.ts

- lines: 84
- it(): 4
- expect(): 7
- flagged lines: none

## typescript/vorma/client/src/tests/unit/extras_internal.test.ts

- lines: 96
- it(): 3
- expect(): 10
- flagged lines: none

## typescript/vorma/client/src/tests/unit/fetch_route_data_preload_commands_internal.test.ts

- lines: 49
- it(): 3
- expect(): 3
- flagged lines: none

## typescript/vorma/client/src/tests/unit/fetch_route_data_preload_state_machine_internal.test.ts

- lines: 49
- it(): 3
- expect(): 3
- flagged lines: none

## typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts

- lines: 386
- it(): 9
- expect(): 20
- flagged lines: none

## typescript/vorma/client/src/tests/unit/hash_fragment.test.ts

- lines: 283
- it(): 16
- expect(): 43
- flagged lines: none

## typescript/vorma/client/src/tests/unit/history_listener_prelude.test.ts

- lines: 427
- it(): 10
- expect(): 28
- flagged lines: none

## typescript/vorma/client/src/tests/unit/links_internal.test.ts

- lines: 213
- it(): 7
- expect(): 29
- flagged lines: none

## typescript/vorma/client/src/tests/unit/make_typed_api.test.ts

- lines: 127
- it(): 2
- expect(): 14
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts

- lines: 197
- it(): 3
- expect(): 16
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts

- lines: 296
- it(): 8
- expect(): 23
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_outcome_runtime_internal.test.ts

- lines: 285
- it(): 5
- expect(): 23
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts

- lines: 590
- it(): 18
- expect(): 31
- flagged lines:

```
123:	it("stops when operation-id ownership is stale", () => {
139:			reason: "stale_control_ownership",
182:			reason: "redirect_ignored_for_prefetch_or_stale_revalidation",
208:	it("ignores redirect outcomes for stale revalidation entries", () => {
210:		const staleRevalidationEntry = createEntry({
219:			targetUrl: staleRevalidationEntry.targetUrl,
220:			entry: staleRevalidationEntry,
221:			expectedOperationID: staleRevalidationEntry.operationID,
227:			targetUrl: staleRevalidationEntry.targetUrl,
228:			reason: "redirect_ignored_for_prefetch_or_stale_revalidation",
277:	it("classifies lifecycle state from ownership/prefetch/staleness in one seam", () => {
302:		const staleRevalidationEntry = createEntry({
309:				entry: staleRevalidationEntry,
313:		).toBe("stale_revalidation");
340:	it("pre-waiting deletes stale revalidation entries", () => {
355:			reason: "stale_revalidation_pre_waiting",
408:	it("post-asset stops stale revalidation entries after waiting", () => {
409:		const staleRevalidationEntry = createEntry({
417:				entry: staleRevalidationEntry,
423:			reason: "post_asset_stale_revalidation",
```

## typescript/vorma/client/src/tests/unit/navigation_pass_runtime_internal.test.ts

- lines: 179
- it(): 3
- expect(): 13
- flagged lines:

```
151:		const staleEntry = createNavigationEntry({
165:			findNavigationEntry: () => staleEntry,
```

## typescript/vorma/client/src/tests/unit/navigation_revalidation_lane_internal.test.ts

- lines: 146
- it(): 5
- expect(): 21
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts

- lines: 3966
- it(): 79
- expect(): 290
- flagged lines:

```
657:	it("aborts stale prefetch and revalidation work when browser-history navigation starts to a different target", async () => {
662:			const stalePrefetchControl = runtime.beginNavigation({
663:				href: "/browser-history-stale-prefetch",
666:			const staleRevalidationControl = runtime.beginNavigation({
676:			expect(stalePrefetchControl.abortController?.signal.aborted).toBe(
680:				staleRevalidationControl.abortController?.signal.aborted,
752:				href: "/stale-active-entry",
760:				"/stale-active-entry",
766:				href: "/stale-active-entry",
790:	it("does not delete a newer same-target entry when stale navigate rejection resolves", async () => {
795:				href: "/stale-navigate-rejection",
800:				"/stale-navigate-rejection",
806:				href: "/stale-navigate-rejection",
881:				href: "/stale-prefetch-entry",
889:				"/stale-prefetch-entry",
895:				href: "/stale-prefetch-entry",
923:			window.history.replaceState({}, "", "/stale-revalidation-entry");
1184:describe("navigation runtime outcome stale control guards", () => {
1185:	it("does not delete current entry when stale aborted outcome resolves for same target", async () => {
1187:			"/stale-control-aborted",
1221:	it("does not process success when stale operation ID no longer owns target", async () => {
1223:			"/stale-control-success",
1226:		const staleOutcome = createSuccessNavigationOutcome({
1250:			navigationProps: staleOutcome.props,
1251:			outcome: staleOutcome,
1471:	it("does not synchronize build IDs or effectuate redirects for stale redirect outcomes", async () => {
1473:			"/stale-control-redirect",
1481:				latestBuildID: "stale-redirect-build",
1482:				href: "/stale-redirect-target",
1484:					url: new URL("http://localhost:3000/stale-redirect-target"),
1488:					absoluteURL: "http://localhost:3000/stale-redirect-target",
1489:					relativeURL: "/stale-redirect-target",
1868:								data: [{ stale: true }],
2039:	it("does not resolve navigation intent for stale successful completions", async () => {
2047:				href: "/intent-resolution-stale-first",
2051:				"/intent-resolution-stale-first",
2066:								href: "/intent-resolution-stale-second",
2087:	it("aborts stale revalidation after wait completes without rendering", async () => {
2094:			window.history.replaceState({}, "", "/revalidation-stale");
2109:					"/revalidation-stale-after-wait",
2164:			const staleSuccessOutcome = createSuccessNavigationOutcome({
2166:				responseBuildID: "stale-replaced-build-id",
2172:			staleSuccessOutcome.json.matchedPatterns = ["/stale-route"];
2173:			staleSuccessOutcome.json.importURLs = ["/stale-route.js"];
2174:			staleSuccessOutcome.json.exportKeys = ["default"];
2175:			staleSuccessOutcome.json.errorExportKeys = [""];
2178:					staleSuccessOutcome,
2220:	it("does not mark a newer same-target entry complete from a stale render onFinish callback", async () => {
2224:		let staleOnFinishCallback: (() => void) | undefined;
2228:				staleOnFinishCallback = props.onFinish;
2236:				href: "/same-target-stale-render-finish",
2244:				"/same-target-stale-render-finish",
2264:			expect(staleOnFinishCallback).toBeDefined();
2269:				href: "/same-target-stale-render-finish",
2281:			staleOnFinishCallback?.();
2306:			"/stale-render-commit-fence",
2320:		const staleSuccessOutcome = createSuccessNavigationOutcome({
2326:		staleSuccessOutcome.json.title = {
2329:		staleSuccessOutcome.json.matchedPatterns = ["/stale-commit-fence"];
2330:		staleSuccessOutcome.json.importURLs = ["/stale-commit-fence.js"];
2331:		staleSuccessOutcome.json.exportKeys = ["default"];
2332:		staleSuccessOutcome.json.errorExportKeys = [""];
2333:		staleSuccessOutcome.json.metaHeadEls = [
2337:					name: "stale-commit-fence",
2365:					staleSuccessOutcome,
2373:				document.head.querySelector('meta[name="stale-commit-fence"]'),
2385:describe("navigation runtime submit stale checkpoints", () => {
2386:	it("aborts stale deduped submit before finalize response processing after build-id listener replacement", async () => {
2388:		const firstJson = vi.fn(async () => ({ stale: true }));
2403:				{ dedupeKey: "finalize-stale", revalidate: false },
2431:				{ dedupeKey: "finalize-stale", revalidate: false },
2452:	it("aborts stale deduped submit after JSON parsing when a replacement submission starts during parsing", async () => {
2484:				{ dedupeKey: "json-stale", revalidate: false },
2494:				{ dedupeKey: "json-stale", revalidate: false },
2496:			firstJSONDeferred.resolve({ stale: true });
2512:	it("aborts stale deduped submit when response classification starts a replacement submission", async () => {
2540:									dedupeKey: "classification-stale",
2547:					json: async () => ({ stale: true }),
2563:				{ dedupeKey: "classification-stale", revalidate: false },
2582:	it("aborts stale deduped submit before auto-revalidation when method access starts a replacement submission", async () => {
2602:						{ dedupeKey: "revalidate-stale", revalidate: false },
2624:				{ dedupeKey: "revalidate-stale" },
2885:	it("aborts stale deduped submit when replacement starts during redirect effectuation", async () => {
2922:							dedupeKey: "redirect-effectuation-stale",
2945:					dedupeKey: "redirect-effectuation-stale",
2966:	it("aborts stale deduped submit when replacement starts during auto-revalidation navigation", async () => {
2986:							dedupeKey: "auto-revalidate-stale",
3005:							json: async () => ({ stale: true }),
3024:					dedupeKey: "auto-revalidate-stale",
3829:	it("does not preload stale deps or update build ID from superseded browser-history responses", async () => {
3835:		const staleNavigationFetch = createDeferred<Response>();
3845:				return staleNavigationFetch.promise;
3856:			const staleNavigation = runtime.navigate({
3857:				href: "/stale-browser-history-race",
3898:			staleNavigationFetch.resolve(
3909:						deps: ["/stale-dep.js"],
3921:							"X-Vorma-Build-Id": "stale-browser-history-build",
3926:			await staleNavigation;
```

## typescript/vorma/client/src/tests/unit/navigation_runtime_slots_internal.test.ts

- lines: 180
- it(): 5
- expect(): 9
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_state_machine_bundle_internal.test.ts

- lines: 94
- it(): 2
- expect(): 2
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_state_machine_internal.test.ts

- lines: 68
- it(): 2
- expect(): 6
- flagged lines: none

## typescript/vorma/client/src/tests/unit/navigation_successful_runtime_commit_internal.test.ts

- lines: 79
- it(): 1
- expect(): 6
- flagged lines: none

## typescript/vorma/client/src/tests/unit/redirect_request_init.test.ts

- lines: 144
- it(): 8
- expect(): 13
- flagged lines: none

## typescript/vorma/client/src/tests/unit/redirects_internal.test.ts

- lines: 482
- it(): 14
- expect(): 35
- flagged lines: none

## typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts

- lines: 867
- it(): 33
- expect(): 77
- flagged lines:

```
147:	it("ignores stale error text when no error index is set", () => {
151:			outermostServerError: "stale-server-error",
152:			outermostClientError: "stale-client-error",
753:			legacySnapshot: {
764:				legacySnapshot: {
813:		expect(committedSnapshot.legacySnapshot.matchedPatterns).toEqual(
816:		expect(committedSnapshot.legacySnapshot.loadersData).toEqual(
819:		expect(committedSnapshot.legacySnapshot.clientLoadersData).toEqual(
822:		expect(committedSnapshot.legacySnapshot.outermostError).toBe(
825:		expect(committedSnapshot.legacySnapshot.outermostErrorIdx).toBe(
```

## typescript/vorma/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts

- lines: 79
- it(): 5
- expect(): 5
- flagged lines:

```
14:			staleTimeMS: 0,
29:			staleTimeMS: 0,
44:			staleTimeMS: 0,
50:	it("blocks focus revalidate when stale window has not elapsed", () => {
59:			staleTimeMS: 50,
65:	it("allows focus revalidate exactly at stale window boundary", () => {
74:			staleTimeMS: 50,
```

## typescript/vorma/client/src/tests/unit/revalidation_trigger_timestamp_state_machine_internal.test.ts

- lines: 62
- it(): 4
- expect(): 6
- flagged lines: none

## typescript/vorma/client/src/tests/unit/route_outlet_runtime_internal.test.ts

- lines: 568
- it(): 14
- expect(): 36
- flagged lines: none

## typescript/vorma/client/src/tests/unit/scroll_apply_state.test.ts

- lines: 21
- it(): 2
- expect(): 2
- flagged lines: none

## typescript/vorma/client/src/tests/unit/scroll_state_refresh_state.test.ts

- lines: 103
- it(): 5
- expect(): 12
- flagged lines:

```
69:	it("removes snapshots that are stale or for a different URL", () => {
```

## typescript/vorma/client/src/tests/unit/scroll_state_storage.test.ts

- lines: 80
- it(): 5
- expect(): 12
- flagged lines: none

## typescript/vorma/client/src/tests/unit/submission_lifecycle_commands_internal.test.ts

- lines: 170
- it(): 4
- expect(): 13
- flagged lines:

```
152:	it("finishes stale submissions without removing or emitting removal transition", () => {
```

## typescript/vorma/client/src/tests/unit/typed_adapter_helpers_runtime_internal.test.ts

- lines: 382
- it(): 11
- expect(): 16
- flagged lines:

```
40:	it("does not expose legacy route-props indexed resolver through internal exports", async () => {
248:			'useClientLoaderData(routeProps) contract violated for pattern "/other": route instance is bound to pattern "/probe".',
261:			"useLoaderData(routeProps) contract violated: route instance token is missing or invalid.",
293:			"useLoaderData(routeProps) contract violated: route instance has been disposed.",
322:			"useLoaderData(routeProps) contract violated: routeProps.idx changed after route instance binding.",
```

## typescript/vorma/client/src/tests/unit/utils_runtime.test.ts

- lines: 73
- it(): 5
- expect(): 13
- flagged lines: none

## typescript/vorma/client/src/tests/unit/vorma_ctx.test.ts

- lines: 128
- it(): 8
- expect(): 18
- flagged lines: none

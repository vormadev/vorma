# Mining Dispatch

Claim rules:

1. Claim the lowest-numbered slot with status `OPEN`.
2. Set slot status to `CLAIMED` before editing package artifacts.
3. Edit only the claimed `spec_path` under `spec/packages/**`.
4. One active `CLAIMED` slot per owner.
5. Only the slot owner may mark that slot `DONE`.
6. Set status to `DONE` only after passing all guard checks.
7. A slot must remain `CLAIMED` until both required independent review passes
   are recorded as `PASS_NO_NOTES`.

Preferred commands:

- Claim: `spec/tools/claim_lowest_open_slot.sh <owner>`
- Review:
  `spec/tools/record_review_pass.sh SLOT-XXX <reviewer> <pass(1|2)> <PASS_NO_NOTES|FAIL_NOTES> <notes_ref>`
- Done: `spec/tools/mark_slot_done.sh SLOT-XXX <owner>`

Edit only rows in the TSV block below when claiming or completing work.

```tsv
slot_id	status	priority_group	spec_path	source_roots	owner	updated_utc	notes
SLOT-001	OPEN	vorma	spec/packages/vormaroot	vorma.go	-	-	-
SLOT-002	OPEN	vorma	spec/packages/vormabuild	vormabuild/**	-	-	-
SLOT-003	OPEN	vorma	spec/packages/vormaruntime	vormaruntime/**	-	-	-
SLOT-004	OPEN	vorma	spec/packages/vormaclient/client	vormaclient/client/**	-	-	-
SLOT-005	OPEN	vorma	spec/packages/vormaclient/react	vormaclient/react/**	-	-	-
SLOT-006	OPEN	vorma	spec/packages/vormaclient/preact	vormaclient/preact/**	-	-	-
SLOT-007	OPEN	vorma	spec/packages/vormaclient/solid	vormaclient/solid/**	-	-	-
SLOT-008	OPEN	vorma	spec/packages/vormaclient/vite	vormaclient/vite/**	-	-	-
SLOT-009	OPEN	wave	spec/packages/wave	wave/*.go	-	-	-
SLOT-010	OPEN	wave	spec/packages/wave/tooling	wave/tooling/**	-	-	-
SLOT-011	OPEN	kit	spec/packages/kit/_typescript	kit/_typescript/**	-	-	-
SLOT-012	OPEN	kit	spec/packages/kit/_typescript/converters	kit/_typescript/converters/**	-	-	-
SLOT-013	OPEN	kit	spec/packages/kit/_typescript/cookies	kit/_typescript/cookies/**	-	-	-
SLOT-014	OPEN	kit	spec/packages/kit/_typescript/csrf	kit/_typescript/csrf/**	-	-	-
SLOT-015	OPEN	kit	spec/packages/kit/_typescript/debounce	kit/_typescript/debounce/**	-	-	-
SLOT-016	OPEN	kit	spec/packages/kit/_typescript/fmt	kit/_typescript/fmt/**	-	-	-
SLOT-017	OPEN	kit	spec/packages/kit/_typescript/json	kit/_typescript/json/**	-	-	-
SLOT-018	OPEN	kit	spec/packages/kit/_typescript/listeners	kit/_typescript/listeners/**	-	-	-
SLOT-019	OPEN	kit	spec/packages/kit/_typescript/matcher	kit/_typescript/matcher/**	-	-	-
SLOT-020	OPEN	kit	spec/packages/kit/_typescript/theme	kit/_typescript/theme/**	-	-	-
SLOT-021	OPEN	kit	spec/packages/kit/_typescript/url	kit/_typescript/url/**	-	-	-
SLOT-022	OPEN	kit	spec/packages/kit/bytesutil	kit/bytesutil/**	-	-	-
SLOT-023	OPEN	kit	spec/packages/kit/colorlog	kit/colorlog/**	-	-	-
SLOT-024	OPEN	kit	spec/packages/kit/contextutil	kit/contextutil/**	-	-	-
SLOT-025	OPEN	kit	spec/packages/kit/cookies	kit/cookies/**	-	-	-
SLOT-026	OPEN	kit	spec/packages/kit/cryptoutil	kit/cryptoutil/**	-	-	-
SLOT-027	OPEN	kit	spec/packages/kit/csrf	kit/csrf/**	-	-	-
SLOT-028	OPEN	kit	spec/packages/kit/envutil	kit/envutil/**	-	-	-
SLOT-029	OPEN	kit	spec/packages/kit/executil	kit/executil/**	-	-	-
SLOT-030	OPEN	kit	spec/packages/kit/fsutil	kit/fsutil/**	-	-	-
SLOT-031	OPEN	kit	spec/packages/kit/genericsutil	kit/genericsutil/**	-	-	-
SLOT-032	OPEN	kit	spec/packages/kit/grace	kit/grace/**	-	-	-
SLOT-033	OPEN	kit	spec/packages/kit/headels	kit/headels/**	-	-	-
SLOT-034	OPEN	kit	spec/packages/kit/htmlutil	kit/htmlutil/**	-	-	-
SLOT-035	OPEN	kit	spec/packages/kit/id	kit/id/**	-	-	-
SLOT-036	OPEN	kit	spec/packages/kit/ioutil	kit/ioutil/**	-	-	-
SLOT-037	OPEN	kit	spec/packages/kit/jsonutil	kit/jsonutil/**	-	-	-
SLOT-038	OPEN	kit	spec/packages/kit/keyset	kit/keyset/**	-	-	-
SLOT-039	OPEN	kit	spec/packages/kit/lazyget	kit/lazyget/**	-	-	-
SLOT-040	OPEN	kit	spec/packages/kit/lru	kit/lru/**	-	-	-
SLOT-041	OPEN	kit	spec/packages/kit/matcher	kit/matcher/**	-	-	-
SLOT-042	OPEN	kit	spec/packages/kit/middleware	kit/middleware/**	-	-	-
SLOT-043	OPEN	kit	spec/packages/kit/middleware/etag	kit/middleware/etag/**	-	-	-
SLOT-044	OPEN	kit	spec/packages/kit/middleware/healthcheck	kit/middleware/healthcheck/**	-	-	-
SLOT-045	OPEN	kit	spec/packages/kit/middleware/robotstxt	kit/middleware/robotstxt/**	-	-	-
SLOT-046	OPEN	kit	spec/packages/kit/middleware/secureheaders	kit/middleware/secureheaders/**	-	-	-
SLOT-047	OPEN	kit	spec/packages/kit/mux	kit/mux/**	-	-	-
SLOT-048	OPEN	kit	spec/packages/kit/netutil	kit/netutil/**	-	-	-
SLOT-049	OPEN	kit	spec/packages/kit/reflectutil	kit/reflectutil/**	-	-	-
SLOT-050	OPEN	kit	spec/packages/kit/response	kit/response/**	-	-	-
SLOT-051	OPEN	kit	spec/packages/kit/securebytes	kit/securebytes/**	-	-	-
SLOT-052	OPEN	kit	spec/packages/kit/securestring	kit/securestring/**	-	-	-
SLOT-053	OPEN	kit	spec/packages/kit/set	kit/set/**	-	-	-
SLOT-054	OPEN	kit	spec/packages/kit/signedcookie	kit/signedcookie/**	-	-	-
SLOT-055	OPEN	kit	spec/packages/kit/tasks	kit/tasks/**	-	-	-
SLOT-056	OPEN	kit	spec/packages/kit/theme	kit/theme/**	-	-	-
SLOT-057	OPEN	kit	spec/packages/kit/validate	kit/validate/**	-	-	-
SLOT-058	OPEN	lab	spec/packages/lab/bumper	lab/bumper/**	-	-	-
SLOT-059	OPEN	lab	spec/packages/lab/cliutil	lab/cliutil/**	-	-	-
SLOT-060	OPEN	lab	spec/packages/lab/errutil	lab/errutil/**	-	-	-
SLOT-061	OPEN	lab	spec/packages/lab/esbuildutil	lab/esbuildutil/**	-	-	-
SLOT-062	OPEN	lab	spec/packages/lab/fsmarkdown	lab/fsmarkdown/**	-	-	-
SLOT-063	OPEN	lab	spec/packages/lab/jsonschema	lab/jsonschema/**	-	-	-
SLOT-064	OPEN	lab	spec/packages/lab/mailutil	lab/mailutil/**	-	-	-
SLOT-065	OPEN	lab	spec/packages/lab/parseutil	lab/parseutil/**	-	-	-
SLOT-066	OPEN	lab	spec/packages/lab/repoconcat	lab/repoconcat/**	-	-	-
SLOT-067	OPEN	lab	spec/packages/lab/rpc	lab/rpc/**	-	-	-
SLOT-068	OPEN	lab	spec/packages/lab/sqlutil	lab/sqlutil/**	-	-	-
SLOT-069	OPEN	lab	spec/packages/lab/stringsutil	lab/stringsutil/**	-	-	-
SLOT-070	OPEN	lab	spec/packages/lab/timer	lab/timer/**	-	-	-
SLOT-071	OPEN	lab	spec/packages/lab/tsgen	lab/tsgen/**	-	-	-
SLOT-072	OPEN	lab	spec/packages/lab/tsgen/tsgencore	lab/tsgen/tsgencore/**	-	-	-
SLOT-073	OPEN	lab	spec/packages/lab/viteutil	lab/viteutil/**	-	-	-
SLOT-074	OPEN	lab	spec/packages/lab/xyz	lab/xyz/**	-	-	-
SLOT-075	OPEN	bootstrap-create	spec/packages/bootstrap	bootstrap/**	-	-	-
SLOT-076	OPEN	bootstrap-create	spec/packages/vormaclient/create	vormaclient/create/**	-	-	-
```

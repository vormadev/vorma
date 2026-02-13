# Breaking Changes Ledger (Against `main`)

Baseline for this ledger:

- `main` at `0a94922d` (`2026-01-21`)

Ledger rules:

- Record every planned next-release breaking public change here.
- Include concrete evidence paths.
- Keep status current: `implemented`, `not-yet-accepted`.
- Measure breakage only against `main`.

## Implemented (Against `main`)

```json
[
	{
		"id": "BRK-001",
		"area": "npm ./client API",
		"breakingChange": "route(...) removed from vorma/client; moved to dedicated vorma/buildtime export",
		"status": "implemented",
		"evidence": [
			"vormaclient/client/index.ts",
			"vormaclient/client/buildtime.ts",
			"package.json"
		]
	},
	{
		"id": "BRK-002",
		"area": "Wave build/dev API",
		"breakingChange": "Build/dev responsibilities moved out of wave.Wave default usage into wave/tooling API surface",
		"status": "implemented",
		"evidence": ["wave/wave.go", "wave/tooling/cli.go", "package.json"]
	},
	{
		"id": "BRK-003",
		"area": "Wave config JSON shape",
		"breakingChange": "Bootstrap-generated wave.config.json removed Core.ConfigLocation, Core.DevBuildHook, Core.ProdBuildHook; renamed Vorma keys (ClientRouteDefsFile -> ClientRouteDefinitionPatterns, TSGenOutPath -> TSGenOutDir) and added MainBuildEntry + ServerRouteDefinitionPatterns",
		"status": "implemented",
		"evidence": ["bootstrap/tmpls/wave_config_json_tmpl.txt"]
	},
	{
		"id": "BRK-004",
		"area": "Go top-level config field",
		"breakingChange": "GetHeadElUniqueRules renamed to GetHeadDedupeKeys in app config usage",
		"status": "implemented",
		"evidence": [
			"vormaruntime/glue.go",
			"bootstrap/tmpls/backend_src_router_app_go_tmpl.txt"
		]
	},
	{
		"id": "BRK-005",
		"area": "Go top-level aliases",
		"breakingChange": "BuildOptions alias removed from top-level vorma public aliases",
		"status": "implemented",
		"evidence": ["vorma.go"]
	},
	{
		"id": "BRK-006",
		"area": "Project scaffold layout",
		"breakingChange": "Bootstrap output changed from single route-def file (frontend/src/vorma.routes.ts) to pattern-based multi-file route defs (frontend/src/routes/*.vorma.routes.ts)",
		"status": "implemented",
		"evidence": [
			"bootstrap/bootstrap.go",
			"bootstrap/tmpls/frontend_routes_core_ts_str.txt",
			"bootstrap/tmpls/frontend_routes_links_ts_str.txt"
		]
	},
	{
		"id": "BRK-007",
		"area": "Generated TS helper locations",
		"breakingChange": "Bootstrap output switched from vorma.gen.ts + split helpers (vorma.utils.tsx, vorma.api.ts) to directory output (vorma.gen/index.ts) and consolidated helper file (vorma.app.tsx)",
		"status": "implemented",
		"evidence": [
			"bootstrap/tmpls/frontend_app_tsx_tmpl.txt",
			"bootstrap/tmpls/frontend_home_tsx_tmpl.txt",
			"bootstrap/tmpls/frontend_links_tsx_tmpl.txt"
		]
	},
	{
		"id": "BRK-008",
		"area": "Package path structure",
		"breakingChange": "npm export implementation paths moved from internal/framework/_typescript/* to vormaclient/*",
		"status": "implemented",
		"evidence": ["package.json"]
	}
]
```

## Not Yet Accepted (Future Plans)

```json
[
	{
		"id": "BRK-009",
		"area": "Client export surface",
		"futureBreakingChange": "Split stable app exports from internal/adapter exports so app code cannot depend on __* internals by default",
		"status": "not-yet-accepted",
		"notes": "See API_SIMPLIFICATION_AUDIT.md"
	},
	{
		"id": "BRK-010",
		"area": "Route authoring internals",
		"futureBreakingChange": "Remove side-effect registration dependency while preserving one-backend-file and two-file full-stack authoring ergonomics",
		"status": "not-yet-accepted",
		"notes": "See LOCKED_DECISIONS_AND_ARCHITECTURE.md"
	},
	{
		"id": "BRK-011",
		"area": "Loader index authoring",
		"futureBreakingChange": "Remove explicit index-segment authoring (/_index) from default app-facing route patterns",
		"status": "not-yet-accepted",
		"notes": "Pending explicit acceptance"
	},
	{
		"id": "BRK-013",
		"area": "Bootstrap API client helper",
		"futureBreakingChange": "Replace global mutable setAPIRequestInitResolver(...) path with explicit immutable API-client creation",
		"status": "not-yet-accepted",
		"notes": "Must keep concise default usage"
	}
]
```

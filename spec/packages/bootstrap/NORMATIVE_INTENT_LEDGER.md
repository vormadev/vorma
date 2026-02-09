# bootstrap Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `bootstrap/assets/favicon.svg` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/bootstrap.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/api_proxy_ts_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/backend_src_router_router_go_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/backend_static_entry_go_html_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/backend_wave_dev_go_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/backend_wave_prod_go_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/cmd_app_main_go_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/cmd_build_main_go_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/dist_static_keep_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/dockerfile_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_api_client_ts_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_app_utils_tsx_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_css_tailwind_css_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_entry_tsx_preact_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_entry_tsx_react_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_entry_tsx_solid_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_home_tsx_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_links_tsx_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_root_tsx_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_routes_ts_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/frontend_vite_d_ts_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/gitignore_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/main_critical_css_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/main_css_str.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/package_json_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/ts_config_json_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/vercel_json_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/vite_config_ts_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/tmpls/wave_config_json_tmpl.txt` | in-scope | source+tests | 0 | 0 | pending |  |
| `bootstrap/utils.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

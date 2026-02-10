#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

catalog=spec/PACKAGE_CATALOG.tsv
printf "slot_id\tpriority_group\tspec_path\tsource_roots\n" > "$catalog"

slot=1
add_slot() {
	local group="$1"
	local spec_path="$2"
	local source_roots="$3"
	printf "SLOT-%03d\t%s\t%s\t%s\n" "$slot" "$group" "$spec_path" "$source_roots" >> "$catalog"
	slot=$((slot + 1))
}

# 1) vorma: mirror repo paths, with vorma.go mapped to vormaroot
add_slot "vorma" "spec/packages/vormaroot" "vorma.go"
add_slot "vorma" "spec/packages/vormabuild" "vormabuild/**"
add_slot "vorma" "spec/packages/vormaruntime" "vormaruntime/**"
add_slot "vorma" "spec/packages/vormaclient/client" "vormaclient/client/**"
add_slot "vorma" "spec/packages/vormaclient/react" "vormaclient/react/**"
add_slot "vorma" "spec/packages/vormaclient/preact" "vormaclient/preact/**"
add_slot "vorma" "spec/packages/vormaclient/solid" "vormaclient/solid/**"
add_slot "vorma" "spec/packages/vormaclient/vite" "vormaclient/vite/**"

# 2) wave
add_slot "wave" "spec/packages/wave" "wave/*.go"
add_slot "wave" "spec/packages/wave/tooling" "wave/tooling/**"

# 3) kit packages (all dirs with direct files, excluding root)
while IFS= read -r d; do
	[ "$d" = "kit" ] && continue
	if find "$d" -maxdepth 1 -type f | grep -q .; then
		add_slot "kit" "spec/packages/$d" "$d/**"
	fi
done < <(find kit -type d | sort)

# 4) lab packages (all dirs with direct files, excluding root)
while IFS= read -r d; do
	[ "$d" = "lab" ] && continue
	if find "$d" -maxdepth 1 -type f | grep -q .; then
		add_slot "lab" "spec/packages/$d" "$d/**"
	fi
done < <(find lab -type d | sort)

# 5) bootstrap/create (mirror repo paths)
add_slot "bootstrap-create" "spec/packages/bootstrap" "bootstrap/**"
add_slot "bootstrap-create" "spec/packages/vormaclient/create" "vormaclient/create/**"

# Render MINING_DISPATCH.md from catalog
cat > spec/MINING_DISPATCH.md <<'MD'
# Mining Dispatch

Claim rules:

1. Claim the lowest-numbered slot with status `OPEN`.
2. Set slot status to `CLAIMED` before editing package artifacts.
3. Edit only the claimed `spec_path` under `spec/packages/**`.
4. Set status to `DONE` only after passing all guard checks.
5. Keep the slot `CLAIMED` until both independent review passes are recorded as `PASS_NO_NOTES`.

Preferred commands:

- Claim: `spec/tools/claim_lowest_open_slot.sh <owner>`
- Review: `spec/tools/record_review_pass.sh SLOT-XXX <reviewer> <pass(1|2)> <PASS_NO_NOTES|FAIL_NOTES> <notes_ref>`
- Done: `spec/tools/mark_slot_done.sh SLOT-XXX <owner>`

Edit only rows in the TSV block below when claiming or completing work.

```tsv
slot_id	status	priority_group	spec_path	source_roots	owner	updated_utc	notes
MD
awk -F '\t' 'NR > 1 {printf "%s\tOPEN\t%s\t%s\t%s\t-\t-\t-\n", $1, $2, $3, $4}' "$catalog" >> spec/MINING_DISPATCH.md
printf '%s\n' '```' >> spec/MINING_DISPATCH.md

# Render package index
cat > spec/PACKAGE_CATALOG.md <<'MD'
# Package Catalog

Canonical package order and source-root mapping used by dispatch and guard scripts.

| Slot | Group | Spec Path | Source Roots |
| --- | --- | --- | --- |
MD
awk -F '\t' 'NR > 1 {printf "| %s | %s | `%s` | `%s` |\n", $1, $2, $3, $4}' "$catalog" >> spec/PACKAGE_CATALOG.md

echo "generated spec/PACKAGE_CATALOG.tsv, spec/PACKAGE_CATALOG.md, and spec/MINING_DISPATCH.md"

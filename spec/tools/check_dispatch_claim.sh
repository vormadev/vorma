#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

dispatch_rows=$(mktemp)
trap 'rm -f "$dispatch_rows"' EXIT

awk '
	BEGIN { in_tsv = 0 }
	/^```tsv$/ { in_tsv = 1; next }
	/^```$/ { if (in_tsv) exit }
	in_tsv { print }
' spec/MINING_DISPATCH.md > "$dispatch_rows"

if [ ! -s "$dispatch_rows" ]; then
	echo "dispatch TSV block is missing" >&2
	exit 1
fi

header=$(head -n 1 "$dispatch_rows")
if [ "$header" != $'slot_id\tstatus\tpriority_group\tspec_path\tsource_roots\towner\tupdated_utc\tnotes' ]; then
	echo "dispatch TSV header is invalid" >&2
	exit 1
fi

tail -n +2 "$dispatch_rows" > "$dispatch_rows.data"

first_open_row=$(awk -F '\t' '$2 == "OPEN" {print NR; exit}' "$dispatch_rows.data")
if [ -n "$first_open_row" ]; then
	if ! awk -F '\t' -v row="$first_open_row" 'NR > row && $2 != "OPEN" {exit 1} END {exit 0}' "$dispatch_rows.data"; then
		echo "dispatch order invalid: once OPEN begins, all later slots must be OPEN" >&2
		exit 1
	fi
fi

touched_paths=$(spec/tools/changed_package_paths.sh || true)
touched_count=$(printf '%s\n' "$touched_paths" | sed '/^$/d' | wc -l | tr -d ' ')

if [ "$touched_count" -eq 0 ]; then
	exit 0
fi
if [ "$touched_count" -gt 1 ]; then
	echo "multiple changed package paths detected" >&2
	exit 1
fi

touched_path=$(printf '%s\n' "$touched_paths" | sed -n '/./{p;q;}')
row_info=$(awk -F '\t' -v target="$touched_path" '$4 == target {print NR"\t"$2; exit}' "$dispatch_rows.data")
if [ -z "$row_info" ]; then
	echo "changed package path not found in dispatch: $touched_path" >&2
	exit 1
fi

touched_row=$(printf '%s\n' "$row_info" | cut -f1)
touched_status=$(printf '%s\n' "$row_info" | cut -f2)

if [ "$touched_status" != "CLAIMED" ] && [ "$touched_status" != "DONE" ]; then
	echo "changed package path is not CLAIMED or DONE in dispatch: $touched_path ($touched_status)" >&2
	exit 1
fi

if [ "$touched_status" = "CLAIMED" ]; then
	if awk -F '\t' -v row="$touched_row" 'NR < row && $2 == "OPEN" {exit 0} END {exit 1}' "$dispatch_rows.data"; then
		echo "dispatch order invalid: CLAIMED package has OPEN slot before it" >&2
		echo "changed: $touched_path" >&2
		exit 1
	fi
fi

if [ "$touched_status" = "DONE" ]; then
	if awk -F '\t' -v row="$touched_row" 'NR < row && $2 == "OPEN" {exit 0} END {exit 1}' "$dispatch_rows.data"; then
		echo "dispatch order invalid: DONE package has OPEN slot before it" >&2
		echo "changed: $touched_path" >&2
		exit 1
	fi
fi

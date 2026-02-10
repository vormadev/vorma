#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

slot_id="${1:-}"
if [ -z "$slot_id" ]; then
	echo "usage: spec/tools/mark_slot_done.sh SLOT-XXX" >&2
	exit 1
fi

now_utc=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
lock_dir="spec/.dispatch.lock"
dispatch_file="spec/MINING_DISPATCH.md"
original_tsv=$(mktemp)
updated_tsv=$(mktemp)
updated_dispatch=$(mktemp)
trap 'rm -f "$original_tsv" "$updated_tsv" "$updated_dispatch"; rmdir "$lock_dir" 2>/dev/null || true' EXIT

while ! mkdir "$lock_dir" 2>/dev/null; do
	sleep 0.1
done

awk '
	BEGIN { in_tsv = 0 }
	/^```tsv$/ { in_tsv = 1; next }
	/^```$/ { if (in_tsv) exit }
	in_tsv { print }
' "$dispatch_file" > "$original_tsv"

if [ ! -s "$original_tsv" ]; then
	echo "dispatch TSV block is missing" >&2
	exit 1
fi

row_info=$(awk -F '\t' -v target="$slot_id" 'NR > 1 && $1 == target {print NR"\t"$2"\t"$4; exit}' "$original_tsv")
if [ -z "$row_info" ]; then
	echo "slot not found: $slot_id" >&2
	exit 1
fi

slot_row=$(printf '%s\n' "$row_info" | cut -f1)
slot_status=$(printf '%s\n' "$row_info" | cut -f2)
slot_path=$(printf '%s\n' "$row_info" | cut -f3)

if [ "$slot_status" != "CLAIMED" ] && [ "$slot_status" != "DONE" ]; then
	echo "slot must be CLAIMED or DONE before marking done: $slot_id ($slot_status)" >&2
	exit 1
fi

awk -F '\t' -v OFS='\t' -v row="$slot_row" -v now="$now_utc" '
	NR == 1 {
		print
		next
	}
	NR == row {
		$2 = "DONE"
		$7 = now
		$8 = "done"
		print
		next
	}
	{
		print
	}
' "$original_tsv" > "$updated_tsv"

awk -v tsv_file="$updated_tsv" '
	BEGIN {
		line_count = 0
		while ((getline line < tsv_file) > 0) {
			lines[++line_count] = line
		}
		close(tsv_file)
		in_tsv = 0
	}
	/^```tsv$/ {
		print
		for (i = 1; i <= line_count; i++) {
			print lines[i]
		}
		in_tsv = 1
		next
	}
	in_tsv && /^```$/ {
		print
		in_tsv = 0
		next
	}
	in_tsv {
		next
	}
	{
		print
	}
' "$dispatch_file" > "$updated_dispatch"

mv "$updated_dispatch" "$dispatch_file"

printf 'marked %s DONE (%s)\n' "$slot_id" "$slot_path"

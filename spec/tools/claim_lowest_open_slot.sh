#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

owner="${1:-${USER:-unknown}}"
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

header=$(head -n 1 "$original_tsv")
if [ "$header" != $'slot_id\tstatus\tpriority_group\tspec_path\tsource_roots\towner\tupdated_utc\tnotes' ]; then
	echo "dispatch TSV header is invalid" >&2
	exit 1
fi

if awk -F '\t' 'NR > 1 && $2 != "OPEN" {exit 1} END {exit 0}' "$original_tsv"; then
	changed_count=$(spec/tools/changed_files.sh | sed '/^$/d' | wc -l | tr -d ' ')
	if [ "$changed_count" -gt 0 ]; then
		echo "cannot claim first slot: commit current spec scaffold baseline first" >&2
		exit 1
	fi
fi

if [ "$owner" = "-" ]; then
	echo "owner name '-' is reserved and cannot be used" >&2
	exit 1
fi

existing_claim=$(awk -F '\t' -v target_owner="$owner" 'NR > 1 && $2 == "CLAIMED" && $6 == target_owner {print $1; exit}' "$original_tsv")
if [ -n "$existing_claim" ]; then
	echo "owner already has an active claim: $owner ($existing_claim)" >&2
	exit 1
fi

open_row=$(awk -F '\t' 'NR > 1 && $2 == "OPEN" {print NR; exit}' "$original_tsv")
if [ -z "$open_row" ]; then
	echo "no OPEN slot is available" >&2
	exit 1
fi

awk -F '\t' -v OFS='\t' -v row="$open_row" -v owner="$owner" -v now="$now_utc" '
	NR == 1 {
		print
		next
	}
	NR == row {
		$2 = "CLAIMED"
		$6 = owner
		$7 = now
		$8 = "claimed"
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

claimed_slot=$(awk -F '\t' -v row="$open_row" 'NR == row {print $1}' "$updated_tsv")
claimed_path=$(awk -F '\t' -v row="$open_row" 'NR == row {print $4}' "$updated_tsv")
printf 'claimed %s (%s) for owner %s\n' "$claimed_slot" "$claimed_path" "$owner"

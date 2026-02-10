#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

slot_id="${1:-}"
if [ -z "$slot_id" ]; then
	echo "usage: spec/tools/mark_slot_done.sh SLOT-XXX [owner]" >&2
	exit 1
fi

actor_owner="${2:-${USER:-unknown}}"
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

row_info=$(awk -F '\t' -v target="$slot_id" 'NR > 1 && $1 == target {print NR"\t"$2"\t"$4"\t"$6; exit}' "$original_tsv")
if [ -z "$row_info" ]; then
	echo "slot not found: $slot_id" >&2
	exit 1
fi

slot_row=$(printf '%s\n' "$row_info" | cut -f1)
slot_status=$(printf '%s\n' "$row_info" | cut -f2)
slot_path=$(printf '%s\n' "$row_info" | cut -f3)
slot_owner=$(printf '%s\n' "$row_info" | cut -f4)

if [ "$slot_status" != "CLAIMED" ] && [ "$slot_status" != "DONE" ]; then
	echo "slot must be CLAIMED or DONE before marking done: $slot_id ($slot_status)" >&2
	exit 1
fi

if [ "$slot_owner" = "-" ]; then
	echo "slot owner is invalid for $slot_id" >&2
	exit 1
fi

if [ "$slot_owner" != "$actor_owner" ]; then
	echo "owner mismatch: slot $slot_id is owned by $slot_owner, but actor is $actor_owner" >&2
	exit 1
fi

if rg -n 'TODO' "$slot_path" >/dev/null; then
	echo "cannot mark DONE while TODO markers remain in $slot_path" >&2
	exit 1
fi

if rg -n '\|[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*\|' "$slot_path/70-open-questions.md" >/dev/null; then
	echo "cannot mark DONE while unresolved open questions remain in $slot_path/70-open-questions.md" >&2
	exit 1
fi

if rg -n '^Current Status:[[:space:]]*$|^[[:space:]]*-[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*$|^status:[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*$' "$slot_path/70-open-questions.md" >/dev/null; then
	echo "cannot mark DONE while unresolved status markers remain in $slot_path/70-open-questions.md" >&2
	exit 1
fi

review_file="$slot_path/90-review-gate.md"
if [ ! -f "$review_file" ]; then
	echo "missing review gate file: $review_file" >&2
	exit 1
fi

is_utc_timestamp() {
	local value="$1"
	[[ "$value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
}

get_key() {
	local key="$1"
	awk -F ': ' -v target="$key" '$1 == target {print $2; exit}' "$review_file"
}

current_hash=$(spec/tools/compute_package_artifact_hash.sh "$slot_path")
miner_owner=$(get_key "miner_owner")
target_hash=$(get_key "target_artifacts_hash")
pass1_reviewer=$(get_key "pass1_reviewer")
pass1_result=$(get_key "pass1_result")
pass1_hash=$(get_key "pass1_artifacts_hash")
pass1_time=$(get_key "pass1_completed_utc")
pass1_notes=$(get_key "pass1_notes")
pass2_reviewer=$(get_key "pass2_reviewer")
pass2_result=$(get_key "pass2_result")
pass2_hash=$(get_key "pass2_artifacts_hash")
pass2_time=$(get_key "pass2_completed_utc")
pass2_notes=$(get_key "pass2_notes")

if [ "$miner_owner" != "$slot_owner" ]; then
	echo "review gate miner_owner mismatch: expected $slot_owner, found $miner_owner" >&2
	exit 1
fi

if [ "$pass1_result" != "PASS_NO_NOTES" ] || [ "$pass2_result" != "PASS_NO_NOTES" ]; then
	echo "cannot mark DONE: review gate requires PASS_NO_NOTES for pass1 and pass2" >&2
	exit 1
fi

if [ "$pass1_reviewer" = "-" ] || [ "$pass2_reviewer" = "-" ]; then
	echo "cannot mark DONE: both review pass reviewers must be set" >&2
	exit 1
fi

if [ "$pass1_reviewer" = "$slot_owner" ] || [ "$pass2_reviewer" = "$slot_owner" ]; then
	echo "cannot mark DONE: reviewers must be independent from slot owner $slot_owner" >&2
	exit 1
fi

if [ "$pass1_reviewer" = "$pass2_reviewer" ]; then
	echo "cannot mark DONE: pass1 and pass2 reviewers must be different" >&2
	exit 1
fi

if [ "$target_hash" != "$current_hash" ] || [ "$pass1_hash" != "$current_hash" ] || [ "$pass2_hash" != "$current_hash" ]; then
	echo "cannot mark DONE: review hashes must match current artifact hash $current_hash" >&2
	exit 1
fi

if [ "$pass1_notes" != "-" ] || [ "$pass2_notes" != "-" ]; then
	echo "cannot mark DONE: zero-note passes require pass1_notes and pass2_notes to be '-'" >&2
	exit 1
fi

if ! is_utc_timestamp "$pass1_time" || ! is_utc_timestamp "$pass2_time"; then
	echo "cannot mark DONE: review pass timestamps must be UTC RFC3339" >&2
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

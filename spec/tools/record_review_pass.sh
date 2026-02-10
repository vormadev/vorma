#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

slot_id="${1:-}"
reviewer="${2:-}"
pass_index="${3:-}"
result="${4:-}"
notes_ref="${5:-}"

if [ -z "$slot_id" ] || [ -z "$reviewer" ] || [ -z "$pass_index" ] || [ -z "$result" ] || [ -z "$notes_ref" ]; then
	echo "usage: spec/tools/record_review_pass.sh SLOT-XXX <reviewer> <pass(1|2)> <PASS_NO_NOTES|FAIL_NOTES> <notes_ref>" >&2
	exit 1
fi

if [ "$reviewer" = "-" ]; then
	echo "reviewer '-' is reserved" >&2
	exit 1
fi

if [ "$pass_index" != "1" ] && [ "$pass_index" != "2" ]; then
	echo "pass index must be 1 or 2" >&2
	exit 1
fi

if [ "$result" != "PASS_NO_NOTES" ] && [ "$result" != "FAIL_NOTES" ]; then
	echo "result must be PASS_NO_NOTES or FAIL_NOTES" >&2
	exit 1
fi

if [ "$result" = "PASS_NO_NOTES" ] && [ "$notes_ref" != "-" ]; then
	echo "notes_ref must be '-' when result is PASS_NO_NOTES" >&2
	exit 1
fi

if [ "$result" = "FAIL_NOTES" ] && [ "$notes_ref" = "-" ]; then
	echo "notes_ref is required when result is FAIL_NOTES" >&2
	exit 1
fi

now_utc=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
lock_dir="spec/.dispatch.lock"
dispatch_file="spec/MINING_DISPATCH.md"
original_tsv=$(mktemp)
trap 'rm -f "$original_tsv"; rmdir "$lock_dir" 2>/dev/null || true' EXIT

while ! mkdir "$lock_dir" 2>/dev/null; do
	sleep 0.1
done

awk '
	BEGIN { in_tsv = 0 }
	/^```tsv$/ { in_tsv = 1; next }
	/^```$/ { if (in_tsv) exit }
	in_tsv { print }
' "$dispatch_file" > "$original_tsv"

row_info=$(awk -F '\t' -v target="$slot_id" 'NR > 1 && $1 == target {print $2"\t"$4"\t"$6; exit}' "$original_tsv")
if [ -z "$row_info" ]; then
	echo "slot not found: $slot_id" >&2
	exit 1
fi

slot_status=$(printf '%s\n' "$row_info" | cut -f1)
spec_path=$(printf '%s\n' "$row_info" | cut -f2)
slot_owner=$(printf '%s\n' "$row_info" | cut -f3)

if [ "$slot_status" != "CLAIMED" ] && [ "$slot_status" != "DONE" ]; then
	echo "slot must be CLAIMED or DONE to record review pass: $slot_id ($slot_status)" >&2
	exit 1
fi

if [ "$slot_owner" = "-" ]; then
	echo "slot owner is invalid for $slot_id" >&2
	exit 1
fi

review_file="$spec_path/90-review-gate.md"
if [ ! -f "$review_file" ]; then
	echo "missing review gate file: $review_file" >&2
	exit 1
fi

current_hash=$(spec/tools/compute_package_artifact_hash.sh "$spec_path")

get_key() {
	local key="$1"
	awk -F ': ' -v target="$key" '$1 == target {print $2; exit}' "$review_file"
}

set_key() {
	local key="$1"
	local value="$2"
	local tmp
	tmp=$(mktemp)
	if ! awk -v key="$key" -v value="$value" '
		index($0, key ": ") == 1 {
			print key ": " value
			found = 1
			next
		}
		{ print }
		END { if (!found) exit 2 }
	' "$review_file" > "$tmp"; then
		rm -f "$tmp"
		echo "missing key in review gate: $key ($review_file)" >&2
		exit 1
	fi
	mv "$tmp" "$review_file"
}

miner_owner=$(get_key "miner_owner")
if [ -z "$miner_owner" ] || [ "$miner_owner" = "-" ]; then
	miner_owner="$slot_owner"
	set_key "miner_owner" "$miner_owner"
fi

if [ "$miner_owner" != "$slot_owner" ]; then
	echo "review gate miner_owner mismatch: expected $slot_owner, found $miner_owner" >&2
	exit 1
fi

if [ "$reviewer" = "$miner_owner" ]; then
	echo "reviewer must be independent from miner: $reviewer" >&2
	exit 1
fi

if [ "$pass_index" = "1" ]; then
	set_key "target_artifacts_hash" "$current_hash"
	set_key "pass1_reviewer" "$reviewer"
	set_key "pass1_result" "$result"
	set_key "pass1_artifacts_hash" "$current_hash"
	set_key "pass1_completed_utc" "$now_utc"
	set_key "pass1_notes" "$notes_ref"

	if [ "$result" = "FAIL_NOTES" ]; then
		set_key "pass2_reviewer" "-"
		set_key "pass2_result" "PENDING"
		set_key "pass2_artifacts_hash" "-"
		set_key "pass2_completed_utc" "-"
		set_key "pass2_notes" "-"
	fi
else
	pass1_result=$(get_key "pass1_result")
	pass1_hash=$(get_key "pass1_artifacts_hash")
	pass1_reviewer=$(get_key "pass1_reviewer")

	if [ "$pass1_result" != "PASS_NO_NOTES" ]; then
		echo "cannot record pass 2 before pass 1 is PASS_NO_NOTES" >&2
		exit 1
	fi

	if [ "$pass1_hash" != "$current_hash" ]; then
		echo "cannot record pass 2: pass 1 hash ($pass1_hash) does not match current artifacts ($current_hash)" >&2
		exit 1
	fi

	if [ "$pass1_reviewer" = "$reviewer" ]; then
		echo "pass 2 reviewer must be different from pass 1 reviewer" >&2
		exit 1
	fi

	set_key "target_artifacts_hash" "$current_hash"
	set_key "pass2_reviewer" "$reviewer"
	set_key "pass2_result" "$result"
	set_key "pass2_artifacts_hash" "$current_hash"
	set_key "pass2_completed_utc" "$now_utc"
	set_key "pass2_notes" "$notes_ref"
fi

printf 'recorded review pass %s for %s (%s): %s\n' "$pass_index" "$slot_id" "$spec_path" "$result"

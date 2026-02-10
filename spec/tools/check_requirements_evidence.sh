#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

dispatch_rows=$(mktemp)
trap 'rm -f "$dispatch_rows" "$dispatch_rows.data"' EXIT

awk '
	BEGIN { in_tsv = 0 }
	/^```tsv$/ { in_tsv = 1; next }
	/^```$/ { if (in_tsv) exit }
	in_tsv { print }
' spec/MINING_DISPATCH.md > "$dispatch_rows"

tail -n +2 "$dispatch_rows" > "$dispatch_rows.data"

required_files=(
	"00-scope.md"
	"10-capabilities.md"
	"20-requirements.md"
	"30-state-model.md"
	"40-errors.md"
	"50-external-contracts.md"
	"60-nonfunctional.md"
	"70-open-questions.md"
	"80-assertion-accounting.md"
	"90-review-gate.md"
	"evidence.yaml"
)

failures=0

is_utc_timestamp() {
	local value="$1"
	[[ "$value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
}

get_counter() {
	local key="$1"
	local file="$2"
	awk -F ': ' -v target="$key" '$1 == target {print $2; exit}' "$file"
}

get_key() {
	local key="$1"
	local file="$2"
	awk -F ': ' -v target="$key" '$1 == target {print $2; exit}' "$file"
}

while IFS=$'\t' read -r slot_id status priority_group spec_path source_roots owner updated_utc notes; do
	[ "$status" = "DONE" ] || continue

	for f in "${required_files[@]}"; do
		if [ ! -f "$spec_path/$f" ]; then
			echo "[$slot_id] missing required file: $spec_path/$f" >&2
			failures=$((failures + 1))
		fi
	done

	if [ ! -d "$spec_path" ]; then
		echo "[$slot_id] missing spec path directory: $spec_path" >&2
		failures=$((failures + 1))
		continue
	fi

	if rg -n '\|[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*\|' "$spec_path/70-open-questions.md" >/dev/null; then
		echo "[$slot_id] unresolved open question statuses in $spec_path/70-open-questions.md" >&2
		failures=$((failures + 1))
	fi

	if rg -n '^Current Status:[[:space:]]*$|^[[:space:]]*-[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*$|^status:[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*$' "$spec_path/70-open-questions.md" >/dev/null; then
		echo "[$slot_id] unresolved open question markers found in $spec_path/70-open-questions.md" >&2
		failures=$((failures + 1))
	fi

	if rg -n 'TODO' "$spec_path" >/dev/null; then
		echo "[$slot_id] TODO markers remain in $spec_path" >&2
		failures=$((failures + 1))
	fi

	if ! rg -q '^## Ownership Boundaries$' "$spec_path/00-scope.md"; then
		echo "[$slot_id] ownership boundary section missing in $spec_path/00-scope.md" >&2
		failures=$((failures + 1))
	fi

	if ! rg -q 'Upstream Requirement Refs' "$spec_path/20-requirements.md"; then
		echo "[$slot_id] upstream requirement reference column missing in $spec_path/20-requirements.md" >&2
		failures=$((failures + 1))
	fi

	if ! rg -q 'Ownership' "$spec_path/20-requirements.md"; then
		echo "[$slot_id] ownership column missing in $spec_path/20-requirements.md" >&2
		failures=$((failures + 1))
	fi

	if ! awk -F '|' -v slot_id="$slot_id" '
		function trim(value) {
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
			return value
		}
		/^[[:space:]]*\|[[:space:]]*REQ-[A-Z0-9-]+-[0-9]{4}[[:space:]]*\|/ {
			req_id = trim($2)
			ownership = trim($6)
			upstream = trim($7)

			if (ownership != "OWNED" && ownership != "INHERITED" && ownership != "DELTA") {
				print "[" slot_id "] invalid ownership value for " req_id ": " ownership
				fail = 1
			}

			if ((ownership == "INHERITED" || ownership == "DELTA") && (upstream == "" || upstream == "-")) {
				print "[" slot_id "] missing upstream refs for " req_id " with ownership " ownership
				fail = 1
			}
		}
		END { exit fail ? 1 : 0 }
	' "$spec_path/20-requirements.md"; then
		failures=$((failures + 1))
	fi

	req_ids=$(awk -F '|' '
		function trim(value) {
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
			return value
		}
		/^[[:space:]]*\|[[:space:]]*REQ-[A-Z0-9-]+-[0-9]{4}[[:space:]]*\|/ {
			print trim($2)
		}
	' "$spec_path/20-requirements.md" | sort -u)

	if [ -z "$req_ids" ]; then
		echo "[$slot_id] no requirement IDs found in $spec_path/20-requirements.md" >&2
		failures=$((failures + 1))
	fi

	while IFS= read -r req; do
		[ -n "$req" ] || continue
		if ! rg -q "^[[:space:]]*$req:" "$spec_path/evidence.yaml"; then
			echo "[$slot_id] missing evidence mapping for $req in $spec_path/evidence.yaml" >&2
			failures=$((failures + 1))
		fi
		if ! rg -q "^### $req([[:space:]]|$)" "$spec_path/20-requirements.md"; then
			echo "[$slot_id] missing prose detail section for $req in $spec_path/20-requirements.md" >&2
			failures=$((failures + 1))
		fi
	done <<< "$req_ids"

	if ! rg -q 'kind: test' "$spec_path/evidence.yaml"; then
		echo "[$slot_id] missing test evidence entries in $spec_path/evidence.yaml" >&2
		failures=$((failures + 1))
	fi
	if ! rg -q 'kind: implementation' "$spec_path/evidence.yaml"; then
		echo "[$slot_id] missing implementation evidence entries in $spec_path/evidence.yaml" >&2
		failures=$((failures + 1))
	fi

	counter_file="$spec_path/80-assertion-accounting.md"
	total=$(get_counter "total_assertions" "$counter_file")
	meaningful=$(get_counter "meaningful_assertions" "$counter_file")
	non_meaningful=$(get_counter "non_meaningful_assertions" "$counter_file")
	mapped=$(get_counter "mapped_meaningful_assertions" "$counter_file")
	unclassified=$(get_counter "unclassified_assertions" "$counter_file")

	for value_name in total meaningful non_meaningful mapped unclassified; do
		value=$(eval "printf '%s' \"\${$value_name}\"")
		if ! [[ "$value" =~ ^[0-9]+$ ]]; then
			echo "[$slot_id] non-numeric assertion counter: $value_name=$value" >&2
			failures=$((failures + 1))
		fi
	done

	if [[ "$total" =~ ^[0-9]+$ && "$meaningful" =~ ^[0-9]+$ && "$non_meaningful" =~ ^[0-9]+$ ]]; then
		if [ "$total" -ne $((meaningful + non_meaningful)) ]; then
			echo "[$slot_id] assertion counter invariant failed: total != meaningful + non_meaningful" >&2
			failures=$((failures + 1))
		fi
	fi

	if [[ "$meaningful" =~ ^[0-9]+$ && "$mapped" =~ ^[0-9]+$ ]]; then
		if [ "$meaningful" -ne "$mapped" ]; then
			echo "[$slot_id] assertion counter invariant failed: meaningful != mapped_meaningful" >&2
			failures=$((failures + 1))
		fi
	fi

	if [[ "$unclassified" =~ ^[0-9]+$ ]]; then
		if [ "$unclassified" -ne 0 ]; then
			echo "[$slot_id] assertion counter invariant failed: unclassified_assertions must be 0" >&2
			failures=$((failures + 1))
		fi
	fi

	review_file="$spec_path/90-review-gate.md"
	current_hash=$(spec/tools/compute_package_artifact_hash.sh "$spec_path")
	miner_owner=$(get_key "miner_owner" "$review_file")
	target_hash=$(get_key "target_artifacts_hash" "$review_file")
	pass1_reviewer=$(get_key "pass1_reviewer" "$review_file")
	pass1_result=$(get_key "pass1_result" "$review_file")
	pass1_hash=$(get_key "pass1_artifacts_hash" "$review_file")
	pass1_time=$(get_key "pass1_completed_utc" "$review_file")
	pass1_notes=$(get_key "pass1_notes" "$review_file")
	pass2_reviewer=$(get_key "pass2_reviewer" "$review_file")
	pass2_result=$(get_key "pass2_result" "$review_file")
	pass2_hash=$(get_key "pass2_artifacts_hash" "$review_file")
	pass2_time=$(get_key "pass2_completed_utc" "$review_file")
	pass2_notes=$(get_key "pass2_notes" "$review_file")

	if [ "$miner_owner" != "$owner" ]; then
		echo "[$slot_id] review gate miner_owner must match slot owner: expected $owner, found $miner_owner" >&2
		failures=$((failures + 1))
	fi

	if [ "$pass1_result" != "PASS_NO_NOTES" ] || [ "$pass2_result" != "PASS_NO_NOTES" ]; then
		echo "[$slot_id] review gate requires PASS_NO_NOTES for both pass1 and pass2" >&2
		failures=$((failures + 1))
	fi

	if [ "$pass1_reviewer" = "-" ] || [ "$pass2_reviewer" = "-" ]; then
		echo "[$slot_id] review gate reviewers must be set for both passes" >&2
		failures=$((failures + 1))
	fi

	if [ "$pass1_reviewer" = "$owner" ] || [ "$pass2_reviewer" = "$owner" ]; then
		echo "[$slot_id] review gate reviewers must be independent from miner owner $owner" >&2
		failures=$((failures + 1))
	fi

	if [ "$pass1_reviewer" = "$pass2_reviewer" ]; then
		echo "[$slot_id] review gate pass1 and pass2 reviewers must differ" >&2
		failures=$((failures + 1))
	fi

	if [ "$target_hash" != "$current_hash" ] || [ "$pass1_hash" != "$current_hash" ] || [ "$pass2_hash" != "$current_hash" ]; then
		echo "[$slot_id] review gate hashes must match current artifact hash ($current_hash)" >&2
		failures=$((failures + 1))
	fi

	if [ "$pass1_notes" != "-" ] || [ "$pass2_notes" != "-" ]; then
		echo "[$slot_id] PASS_NO_NOTES requires pass1_notes and pass2_notes to be '-'" >&2
		failures=$((failures + 1))
	fi

	if ! is_utc_timestamp "$pass1_time" || ! is_utc_timestamp "$pass2_time"; then
		echo "[$slot_id] review gate pass timestamps must be UTC RFC3339 (YYYY-MM-DDTHH:MM:SSZ)" >&2
		failures=$((failures + 1))
	fi
done < "$dispatch_rows.data"

if [ "$failures" -gt 0 ]; then
	echo "requirement/evidence checks failed with $failures issue(s)" >&2
	exit 1
fi

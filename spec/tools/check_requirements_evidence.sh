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
	"evidence.yaml"
)

failures=0

while IFS=$'\t' read -r slot_id status priority_group spec_path source_roots owner updated_utc notes; do
	[ "$status" = "DONE" ] || continue

	for f in "${required_files[@]}"; do
		if [ ! -f "$spec_path/$f" ]; then
			echo "[$slot_id] missing required file: $spec_path/$f" >&2
			failures=$((failures + 1))
		fi
	done

	if rg -n '\|[[:space:]]*(OPEN|UNRESOLVED)[[:space:]]*\|' "$spec_path/70-open-questions.md" >/dev/null; then
		echo "[$slot_id] unresolved open question statuses in $spec_path/70-open-questions.md" >&2
		failures=$((failures + 1))
	fi

	if rg -n 'TODO' "$spec_path" >/dev/null; then
		echo "[$slot_id] TODO markers remain in $spec_path" >&2
		failures=$((failures + 1))
	fi

	get_counter() {
		local key="$1"
		local file="$2"
		awk -F ': ' -v target="$key" '$1 == target {print $2; exit}' "$file"
	}

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

	req_ids=$(rg -o 'REQ-[A-Z0-9-]+-[0-9]{4}' "$spec_path/20-requirements.md" | sort -u || true)
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
	done <<< "$req_ids"

	if ! rg -q 'kind: test' "$spec_path/evidence.yaml"; then
		echo "[$slot_id] missing test evidence entries in $spec_path/evidence.yaml" >&2
		failures=$((failures + 1))
	fi
	if ! rg -q 'kind: implementation' "$spec_path/evidence.yaml"; then
		echo "[$slot_id] missing implementation evidence entries in $spec_path/evidence.yaml" >&2
		failures=$((failures + 1))
	fi
done < "$dispatch_rows.data"

if [ "$failures" -gt 0 ]; then
	echo "requirement/evidence checks failed with $failures issue(s)" >&2
	exit 1
fi

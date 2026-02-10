#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if rg -n '/Users/' spec --glob '*.md' --glob '*.yaml' --glob '*.yml' --glob '*.tsv' >/tmp/spec_abs_path_hits.txt; then
	echo "machine-specific absolute paths found under spec/:" >&2
	cat /tmp/spec_abs_path_hits.txt >&2
	rm -f /tmp/spec_abs_path_hits.txt
	exit 1
fi
rm -f /tmp/spec_abs_path_hits.txt

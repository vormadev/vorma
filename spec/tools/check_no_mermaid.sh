#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if rg -n '```(mermaid|plantuml|graphviz)' spec/packages spec/_templates >/tmp/spec_chart_hits.txt; then
	echo "diagram/chart code blocks are prohibited in package artifacts and templates:" >&2
	cat /tmp/spec_chart_hits.txt >&2
	rm -f /tmp/spec_chart_hits.txt
	exit 1
fi
rm -f /tmp/spec_chart_hits.txt

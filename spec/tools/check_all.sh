#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

spec/tools/check_phase_scope.sh
spec/tools/check_worker_allowlist.sh
spec/tools/check_single_package_scope.sh
spec/tools/check_append_only_docs.sh
spec/tools/check_dispatch_manual_edits.sh
spec/tools/check_dispatch_claim.sh
spec/tools/check_requirements_evidence.sh
spec/tools/check_absolute_paths.sh

echo "all spec guardrail checks passed"

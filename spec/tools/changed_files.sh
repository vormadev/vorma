#!/usr/bin/env bash
set -euo pipefail

{
	git diff --name-only --diff-filter=ACMRTD HEAD
	git ls-files --others --exclude-standard
} | sed '/^$/d' | sort -u

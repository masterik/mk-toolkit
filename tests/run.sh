#!/usr/bin/env bash
#
# Run the shell layer's test suite: bats over the scripts in plugin/scripts/.
#
# The binary's tests are Go's, beside their packages: `go test ./...`.
#
#   usage: tests/run.sh
#
# Exit: 0 all green, 1 something failed, 2 bats is not installed.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

overall=0

echo "== bats (shell scripts) =="
if ! command -v bats >/dev/null 2>&1; then
	echo "bats-core not found — install it to run the shell-script suite:" >&2
	echo "  brew install bats-core" >&2
	exit 2
fi
bats tests/bats/ || overall=1

exit "$overall"

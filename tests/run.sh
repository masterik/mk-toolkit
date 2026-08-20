#!/usr/bin/env bash
#
# Run the whole test suite: node --test over findings.mjs, bats over the shell scripts.
#
#   usage: tests/run.sh
#
# Exit: 0 all green, 1 something failed, 2 bats is not installed.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

overall=0

echo "== node --test (findings.mjs) =="
node --test tests/findings.test.mjs || overall=1

echo
echo "== bats (shell scripts) =="
if ! command -v bats >/dev/null 2>&1; then
	echo "bats-core not found — install it to run the shell-script suite:" >&2
	echo "  brew install bats-core   # macOS" >&2
	echo "  sudo apt install -y bats # Debian/Ubuntu" >&2
	exit 2
fi
bats tests/bats/ || overall=1

exit "$overall"

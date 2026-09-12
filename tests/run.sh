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

# `gate-detect.bats` writes its ledger records by running the real writer, which is
# now the binary. Built once, here, rather than per test: a `go build` inside a bats
# case would dominate the suite's runtime. Temporary — this whole directory goes when
# the last script does.
echo "== building mkit (gate-detect.bats needs the ledger writer) =="
MKIT_BIN="$(mktemp -d "${TMPDIR:-/tmp}/mkit-bin.XXXXXX")/mkit"
if ! go build -o "$MKIT_BIN" ./cmd/mkit; then
	echo "go build failed — the shell suite needs the binary for the gate ledger" >&2
	exit 2
fi
export MKIT_BIN

echo "== bats (shell scripts) =="
if ! command -v bats >/dev/null 2>&1; then
	echo "bats-core not found — install it to run the shell-script suite:" >&2
	echo "  brew install bats-core" >&2
	exit 2
fi
bats tests/bats/ || overall=1

exit "$overall"

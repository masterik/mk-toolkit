#!/usr/bin/env bash
#
# Report (and, with --apply, delete) local Claude / Codex session storage older
# than a retention window. Read-only categories only: per-session transcripts
# and per-session scratch dirs. Never touches installed software (plugins,
# ccline, computer-use, vendor_imports) or singleton state files (history.jsonl,
# *.sqlite*) — those need a different, non-mechanical pruning strategy.
#
#   usage: storage-prune.sh [--days N] [--provider claude|codex|all] [--apply]
#
#   --days N       retention window in days (default: 7)
#   --provider P   claude | codex | all (default: all)
#   --apply        actually delete; default is a dry-run report only

set -euo pipefail

DAYS=7
PROVIDER=all
APPLY=0

usage() {
	sed -n '3,13p' "$0" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
	case "$1" in
	--days)
		DAYS="$2"
		shift 2
		;;
	--provider)
		PROVIDER="$2"
		shift 2
		;;
	--apply)
		APPLY=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "storage-prune: unknown argument: $1" >&2
		usage
		exit 1
		;;
	esac
done

case "$PROVIDER" in
claude | codex | all) ;;
*)
	echo "storage-prune: --provider must be claude, codex, or all" >&2
	exit 1
	;;
esac

CLAUDE_HOME="${CLAUDE_HOME:-$HOME/.claude}"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"

total_bytes=0

human() {
	awk -v b="$1" 'BEGIN {
		split("B KB MB GB TB", u, " ")
		i = 1
		while (b >= 1024 && i < 5) { b /= 1024; i++ }
		printf "%.1f%s", b, u[i]
	}'
}

sum_size() {
	local sum=0 f sz
	while IFS= read -r f; do
		[ -e "$f" ] || continue
		sz=$(stat -f%z "$f" 2>/dev/null || echo 0)
		sum=$((sum + sz))
	done
	echo "$sum"
}

# Prune loose files directly under $2 older than $DAYS days, optionally
# restricted to a -name pattern ($3, e.g. "*.jsonl").
prune_files() {
	local label="$1" base="$2" pattern="${3:-}" files count bytes
	[ -d "$base" ] || return 0
	if [ -n "$pattern" ]; then
		files=$(find "$base" -type f -name "$pattern" -mtime +"$DAYS" 2>/dev/null || true)
	else
		files=$(find "$base" -type f -mtime +"$DAYS" 2>/dev/null || true)
	fi
	if [ -z "$files" ]; then
		printf '  %-38s nothing older than %sd\n' "$label" "$DAYS"
		return 0
	fi
	count=$(printf '%s\n' "$files" | grep -c .)
	bytes=$(printf '%s\n' "$files" | sum_size)
	total_bytes=$((total_bytes + bytes))
	printf '  %-38s %4d files, %s\n' "$label" "$count" "$(human "$bytes")"
	if [ "$APPLY" -eq 1 ]; then
		printf '%s\n' "$files" | while IFS= read -r f; do rm -f "$f"; done
	fi
}

# Prune immediate subdirectories of $2 that contain zero files newer than $DAYS
# days — protects any session dir still being actively written to.
prune_stale_dirs() {
	local label="$1" base="$2" dir fresh dbytes count=0 bytes=0
	local -a stale=()
	[ -d "$base" ] || return 0
	for dir in "$base"/*/; do
		[ -d "$dir" ] || continue
		dir="${dir%/}"
		fresh=$(find "$dir" -type f -mtime -"$DAYS" -print -quit 2>/dev/null || true)
		[ -n "$fresh" ] && continue
		dbytes=$(find "$dir" -type f -print 2>/dev/null | sum_size)
		bytes=$((bytes + dbytes))
		count=$((count + 1))
		stale+=("$dir")
	done
	total_bytes=$((total_bytes + bytes))
	if [ "$count" -eq 0 ]; then
		printf '  %-38s nothing stale\n' "$label"
		return 0
	fi
	printf '  %-38s %4d dirs,  %s\n' "$label" "$count" "$(human "$bytes")"
	if [ "$APPLY" -eq 1 ]; then
		for dir in "${stale[@]}"; do rm -rf "$dir"; done
	fi
}

echo "mode: $([ "$APPLY" -eq 1 ] && echo APPLY || echo DRY-RUN)   retention: ${DAYS}d"
echo

if [ "$PROVIDER" = "all" ] || [ "$PROVIDER" = "claude" ]; then
	echo "Claude ($CLAUDE_HOME):"
	prune_files "projects/**/*.jsonl (transcripts)" "$CLAUDE_HOME/projects" "*.jsonl"
	prune_stale_dirs "file-history/<session>/" "$CLAUDE_HOME/file-history"
	prune_stale_dirs "session-env/<session>/" "$CLAUDE_HOME/session-env"
	prune_files "paste-cache/*" "$CLAUDE_HOME/paste-cache"
	prune_files "shell-snapshots/*" "$CLAUDE_HOME/shell-snapshots"
	prune_files "telemetry/*" "$CLAUDE_HOME/telemetry"
	if [ "$APPLY" -eq 1 ]; then
		find "$CLAUDE_HOME/projects" -type d -empty -delete 2>/dev/null || true
	fi
	echo
fi

if [ "$PROVIDER" = "all" ] || [ "$PROVIDER" = "codex" ]; then
	echo "Codex ($CODEX_HOME):"
	prune_files "sessions/**/*.jsonl (transcripts)" "$CODEX_HOME/sessions" "*.jsonl"
	prune_files "shell_snapshots/*" "$CODEX_HOME/shell_snapshots"
	if [ "$APPLY" -eq 1 ]; then
		find "$CODEX_HOME/sessions" -type d -empty -delete 2>/dev/null || true
	fi
	echo
fi

if [ "$APPLY" -eq 1 ]; then
	echo "total reclaimed: $(human "$total_bytes")"
else
	echo "total reclaimable: $(human "$total_bytes")"
	[ "$total_bytes" -gt 0 ] && echo "(dry run — re-run with --apply to delete)"
fi

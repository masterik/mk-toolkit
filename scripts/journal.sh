#!/usr/bin/env bash
#
# The commit journal: record *why* a unit of work exists, and report whether those
# records still describe the working tree.
#
#   usage: journal.sh add --paths <a,b|repeatable> --type T --scope S
#                         --subject "..." --why "..." [--source note|stop|subagent-stop]
#          journal.sh status                 classification + coverage, facts.sh grammar
#          journal.sh uncovered              dirty paths no entry claims (the hook's path)
#          journal.sh drop --committed | --orphaned | <seq>
#          journal.sh compact                drop committed/orphaned, renumber from 1
#          journal.sh enable | disable | enabled [--why] | path
#
# Why this is a script and not prose in a SKILL.md: every question it answers is
# mechanical. Hashing a path (`git hash-object`), the set arithmetic between the
# recorded paths and the dirty tree, and the class each entry lands in are a lookup
# table over git plumbing — exactly the work that fails silently when a model does it
# by eye, and the only reason a `fresh` verdict is worth trusting.
#
# It never authors a record's judgement. `type`, `scope`, `subject` and above all `why`
# arrive from the caller and are stored verbatim; this script stamps only what it can
# prove (seq, ts, branch, head, blob hashes) and later reports how far the tree moved.
# `overlap:` names a path two entries both claim — which hunks go where stays the
# `commit` skill's decision, the same way `gate-detect.sh` proposes `fast=` beside
# `docs_candidates:` without running anything.
#
# Enablement is three-way and repo-first: <git-dir>/mkit/journal.enabled (on here),
# <git-dir>/mkit/journal.disabled (off here, outvoting the default below), then the
# user-scoped ~/.claude/mkit/journal.default that `install.sh` writes. Nothing set at
# either level means off, which is the state the plugin still ships in.
#
# Storage: <absolute-git-dir>/mkit/journal.jsonl, append-only JSONL, one `unit` record
# per line, beside the run directories `run-open.sh` owns — never committed, never in
# `git status`, and a linked worktree gets its own (so entries die with the worktree).
# `add` only ever appends; `drop`/`compact` rewrite through a temp file and `mv`, never
# in place, so an interrupted rewrite cannot leave a truncated journal.
#
# Reads are scoped to the current branch — each record carries its own — which is what
# lets one file serve the whole repo without slugging branch names into filenames.
#
# Known gaps, deliberately not half-handled:
#   - renames. `git diff --name-only` reports only a rename's destination, so the
#     recorded old path leaves the dirty set: the entry classifies `orphaned` and the
#     new path turns up under `uncovered:`. That degrades to the behavior before the
#     journal existed, which beats tracking renames badly.
#   - a path containing a comma cannot be passed to `--paths`, which splits on commas.
#   - a path containing a newline. Every path set here is a newline-joined string, so
#     such a name cannot be represented at all — it is read correctly from git (`-z`)
#     and then splits into two bogus members. It never round trips, so it stays
#     uncovered, which is the same degradation as a rename.
#
# Exit: 0 ok, 1 not a git repo / no journal where one is required, 2 bad usage.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

TAB=$'\t'
NL=$'\n'
# Field separator for this script's own internal streams. Not a tab: a tab is IFS
# whitespace, so `IFS=$'\t' read` collapses runs of them and an empty --scope would
# silently shift every field after it. 0x1f never appears in a path or a subject.
SEP=$'\037'

cmd="${1:-}"
[ -n "$cmd" ] ||
	mkit_die 'usage: journal.sh add|status|uncovered|drop|compact|enable|disable|enabled|path' 2
shift

mkit_dir="$(mkit_dir_or_die)"
journal="$mkit_dir/journal.jsonl"
marker="$mkit_dir/journal.enabled"
tombstone="$mkit_dir/journal.disabled"
user_default="$(mkit_user_dir)/journal.default"

# --- the cheap half: the opt-in markers and the file location ----------------------
# `enabled` is the hook's first call on every agent turn, so it stays a `git rev-parse`
# plus at most three stats. No porcelain, no jq.
#
# Three-way, in this order — the repo always outranks the profile:
#
#   journal.enabled    this repo, explicitly on
#   journal.disabled   this repo, explicitly off — the tombstone exists *only* to
#                      outvote a user-scoped default. Without it, `disable` in a repo
#                      covered by ~/.claude/mkit/journal.default would be a no-op that
#                      reported success, which is the worst possible answer.
#   journal.default    the user-scoped default (install.sh), applying to every repo
#                      that has said nothing either way
#   otherwise          off — the shipped state, unchanged: install the plugin and
#                      nothing writes anywhere until someone opts in at one of the
#                      two levels.
#
# The tombstone beats the marker if both somehow exist, so a hand-made file cannot leave
# a repo journaling against an explicit `disable`. Precedence never resolves *toward*
# writing.
journal_state() {
	if [ -f "$tombstone" ]; then
		printf 'disabled repo\n'
	elif [ -f "$marker" ]; then
		printf 'enabled repo\n'
	elif [ -f "$user_default" ]; then
		printf 'enabled user\n'
	else
		printf 'disabled none\n'
	fi
}

case "$cmd" in
enabled)
	# The bare word is the contract — journal-nudge.sh gate 2 and facts.sh both compare
	# against it exactly, so `--why` adds a second field rather than a second line.
	why=no
	while [ $# -gt 0 ]; do
		case "$1" in
		--why) why=yes ;;
		*) mkit_die "enabled: unexpected argument: $1" 2 ;;
		esac
		shift
	done
	state="$(journal_state)"
	if [ "$why" = yes ]; then printf '%s\n' "$state"; else printf '%s\n' "${state%% *}"; fi
	exit 0
	;;
enable)
	[ $# -eq 0 ] || mkit_die 'enable takes no arguments' 2
	mkdir -p "$mkit_dir"
	# Clear the tombstone first: `enable` must be able to undo `disable` even where a
	# user-scoped default is in play, and a repo left holding both files reads as off.
	rm -f "$tombstone"
	: >"$marker"
	printf 'journal enabled: %s\n' "$journal"
	exit 0
	;;
disable)
	[ $# -eq 0 ] || mkit_die 'disable takes no arguments' 2
	rm -f "$marker"
	# Only write a tombstone when one is actually load-bearing. With no user-scoped
	# default, removing the marker already means off, and a pristine repo should stay
	# byte-identical to a never-enabled one — that is what keeps `disable` reversible
	# to *nothing* rather than to a permanent opt-out nobody asked for.
	if [ -f "$user_default" ]; then
		mkdir -p "$mkit_dir"
		: >"$tombstone"
		printf 'journal disabled for this repo (overriding %s)\n' "$user_default"
	else
		printf 'journal disabled\n'
	fi
	exit 0
	;;
path)
	[ $# -eq 0 ] || mkit_die 'path takes no arguments' 2
	printf '%s\n' "$journal"
	exit 0
	;;
add | status | uncovered | drop | compact) ;;
*) mkit_die "unknown subcommand: $cmd" 2 ;;
esac

# `add` is the one mutating subcommand, so it is where the opt-in is enforced. Without
# this, a record written in a repo with no marker is invisible: `facts.sh --journal`
# reports `journal=off` and never reads the file, so the intent is captured and then
# silently dropped — write-only journaling. `note`'s SKILL.md already promises this
# refusal and tells the agent to relay it, so the message is the contract.
#
# Only `add`. `status`/`uncovered`/`drop`/`compact` stay readable on a disabled repo, so
# a journal written before `disable` can still be inspected and cleaned up.
if [ "$cmd" = add ]; then
	add_state="$(journal_state)"
	[ "${add_state%% *}" = enabled ] ||
		mkit_die "journaling is not enabled for this repo — run: journal.sh enable" 1
fi

command -v jq >/dev/null 2>&1 || mkit_die 'jq is required to read or write the journal' 1
mkit_require_repo

# The caller's own directory, captured before the cd: `--paths` is relative to it.
cwd="$(pwd -P)"
toplevel="$(git rev-parse --show-toplevel)"
# Run repo-wide from here on, for the same reason facts.sh does: called from a
# subdirectory, the pathspec'd file lists below would cover only that subdirectory
# while the recorded paths stay repo-relative, and coverage would silently disagree.
cd "$toplevel" || mkit_die "cannot enter repo root: $toplevel" 1
toplevel="$(pwd -P)"
branch="$(git branch --show-current)"

# --- helpers ----------------------------------------------------------------------

# Repo-relative form of a caller-supplied path, or die.
#
# Purely lexical, no realpath: a recorded path may name a file that was just deleted,
# so it must not have to exist. Globbing is off while splitting, so a component that
# happens to be `*` names a file called `*` instead of expanding.
journal_relpath() {
	local p="$1" abs out comp
	case "$p" in
	/*) abs="$p" ;;
	*) abs="$cwd/$p" ;;
	esac
	out=""
	set -f
	local IFS=/
	set -- $abs
	for comp in "$@"; do
		case "$comp" in
		'' | '.') ;;
		'..') out="${out%/*}" ;;
		*) out="$out/$comp" ;;
		esac
	done
	set +f
	abs="${out:-/}"
	case "$abs" in
	"$toplevel") mkit_die "--paths needs a file, not the repo root: $p" 2 ;;
	"$toplevel"/*) printf '%s\n' "${abs#"$toplevel"/}" ;;
	*) mkit_die "path is outside the repository: $p" 2 ;;
	esac
}

# The blob hash of a path as it stands on disk, or empty when it is gone. Untracked
# files hash fine, which is what keeps them in scope; a deleted path hashes to "",
# which is what makes a recorded deletion comparable at all.
#
# A symlink is hashed as its link *text*, because that is exactly what git stores as a
# symlink's blob. `git hash-object -- <link>` follows the link and hashes the target's
# content instead, which is a different hash from git's own (verified: 3fef9925 vs the
# index's 847d9afa for the same link) and answers a different question — a link whose
# target was edited read as `drifted` though the link never moved, and one retargeted at
# a same-content file read as `fresh`. `-L` must come before `-e`, since `-e` follows the
# link too; and a *broken* link cannot be hashed through the filesystem at all
# (`hash-object` fails with "could not open"), which recorded "" and pinned the entry to
# `fresh` across every later retarget.
blob_of() {
	if [ -L "$1" ]; then
		printf '%s' "$(readlink "$1")" | git hash-object --stdin
	elif [ -e "$1" ]; then
		git hash-object -- "$1" 2>/dev/null || true
	fi
}

# Every git path list this script reads goes through here — never `git diff --name-only`
# or `git ls-files` directly, and the sweep is the point rather than tidiness: two sets
# read in different encodings disagree with each other, which is worse than both being
# wrong the same way.
#
# By default git C-quotes any path that is not plain printable ASCII: `café.txt` arrives
# as the literal `"caf\303\251.txt"`, which never matches the real name a caller passes
# to `--paths`. Both halves of that were reproduced — recording the real name left the
# entry `orphaned` with the path uncovered forever (so the hook re-nudged a path the
# agent could not satisfy), and recording the quoted literal pinned the entry to `fresh`
# forever, since a nonexistent path hashes to "" and matched the "" that was recorded.
#
# `-z` on top of quotePath=false, and every caller must pass it: once quoting is off, NUL
# is the only thing that still delimits a name containing a space (or a newline — see the
# known gaps; those are read correctly here and lost in the newline-joined sets). `-z`
# goes before any `--`, or git reads it as a pathspec.
git_paths() {
	git -c core.quotePath=false "$@" | tr '\0' '\n'
}

set_has() {
	printf '%s\n' "$1" | grep -qxF -- "$2"
}

count_set() {
	printf '%s\n' "$1" | awk 'NF' | wc -l | tr -d ' '
}

# Two dirty sets, and the difference between them matters.
#
#   dirty_all  every path git sees as changed — what classification tests against, so
#              an entry about a lockfile is not mistaken for a reverted one
#   dirty_cov  the same set under the pathspec exclusions facts.sh uses — what coverage
#              arithmetic runs on, so a path matching *.lock or *.snap never triggers a
#              nudge. Note the glob, not the intent: Cargo.lock, yarn.lock, Gemfile.lock,
#              poetry.lock and friends match, but package-lock.json, pnpm-lock.yaml and
#              go.sum do not, and are nudged like any other file.
load_dirty() {
	dirty_all="$( {
		git_paths diff --name-only -z
		git_paths diff --cached --name-only -z
		git_paths ls-files --others --exclude-standard -z
	} | sort -u | awk 'NF')"
	dirty_cov="$( {
		git_paths diff --name-only -z -- . ':(exclude)*.lock' ':(exclude)*.snap'
		git_paths diff --cached --name-only -z -- . ':(exclude)*.lock' ':(exclude)*.snap'
		git_paths ls-files --others --exclude-standard -z -- ':(exclude)*.lock' ':(exclude)*.snap'
	} | sort -u | awk 'NF')"
}

# Every claim an entry on this branch makes: the head it was recorded against, the path,
# and the blob that path had at the time. One jq call — this is all `uncovered` reads.
entry_claims() {
	[ -f "$journal" ] || return 0
	jq -r --arg b "$branch" --arg s "$SEP" '
		select(.kind == "unit") | select(.branch == $b) | . as $e |
		($e.paths[]? | . as $p |
		  [($e.head // ""), $p, (($e.blobs[$p]) // "")] | join($s))
	' "$journal"
}

# Coverage arithmetic: which dirty paths have no record that still describes them.
#
# Being claimed once is not coverage. Subtracting every entry's paths regardless of
# freshness meant a file edited again after its entry was recorded — the entry now
# `drifted` — still counted as covered: journal_uncovered=0, the hook silent, and the
# second reason for touching that file never captured. (A `committed` entry flipping back
# to `drifted` on the next edit is the same code path, not a second bug.) So a claim
# covers its path only while the path still hashes to what was recorded, and a claim
# whose head no longer resolves covers nothing — the same precedence `unknown-head` has
# in classify_entry, for the same reason.
#
# `uncovered` is the hook's per-turn path and staying cheap is its whole design property,
# so the hash check is kept off the general case twice over:
#
#   - only claims on a path that is *currently* dirty are checked at all (awk does that
#     intersection in one pass), so a journal full of spent entries costs nothing;
#   - the regular files among them are hashed by ONE `hash-object --stdin-paths`, not one
#     fork each. Measured on 50 entries / 150 claimed dirty paths / 200 dirty paths:
#     0.171s per `uncovered` call before this check existed, 0.234s with it batched, and
#     2.45s with a fork per path. A per-turn hook cannot cost two seconds.
#
# Only symlinks and vanished paths still go through blob_of one at a time: `--stdin-paths`
# would follow a symlink (the f05 bug) and aborts the whole batch on a missing file, and
# both are rare enough that the fork does not matter.
#
# awk over two streams rather than `grep -vf`, because an empty pattern list matches
# every line and would report a fully dirty tree as fully covered.
load_coverage() {
	local head path blob good="" bad="" claimed="" reg="" reg_paths="" hashes=""
	local claims live
	# Assigned to a variable first, and never read straight into the loop below through a
	# here-doc: a command substitution inside a here-doc discards its exit status, so a
	# journal jq cannot parse would read as an empty journal instead of failing. facts.sh
	# distinguishes those (`journal=unreadable` vs `journal=empty`) by this call's status.
	claims="$(entry_claims)"
	live="$(printf '%s\n' "$claims" |
		awk -F"$SEP" 'NR == FNR { if (NF) d[$0] = 1; next } NF && ($2 in d)' \
			<(printf '%s\n' "$dirty_cov") -)"
	while IFS=$SEP read -r head path blob; do
		[ -n "$path" ] || continue
		# One rev-parse per distinct head, memoized in two strings: every entry of a
		# session usually shares one head, and forking per claim would not be cheap.
		if [ -n "$head" ]; then
			case "$good$NL" in
			*"$NL$head$NL"*) ;;
			*)
				case "$bad$NL" in
				*"$NL$head$NL"*) continue ;;
				esac
				if git rev-parse --verify --quiet "$head^{commit}" >/dev/null 2>&1; then
					good="$good$NL$head"
				else
					bad="$bad$NL$head"
					continue
				fi
				;;
			esac
		fi
		# Kind first, with builtins only — no fork in the common case.
		if [ -L "$path" ] || [ ! -e "$path" ]; then
			[ "$(blob_of "$path")" = "$blob" ] || continue
			claimed="$claimed$path$NL"
		else
			reg="$reg$path$SEP$blob$NL"
			reg_paths="$reg_paths$path$NL"
		fi
	done <<-EOF
		$live
	EOF
	if [ -n "$reg_paths" ]; then
		# One line of output per line of input, in order, which is what lets awk zip the
		# two streams by line number below. A batch that fails for any reason (a file
		# removed between the listing and here) covers nothing, which is the safe way to
		# be wrong: the path gets nudged.
		hashes="$(printf '%s' "$reg_paths" | git hash-object --stdin-paths 2>/dev/null || true)"
		claimed="$claimed$(awk -v FS="$SEP" '
			NR == FNR { h[FNR] = $0; next }
			NF && h[FNR] == $2 { print $1 }' \
			<(printf '%s\n' "$hashes") <(printf '%s' "$reg"))$NL"
	fi
	covered_paths="$(printf '%s' "$claimed" | sort -u | awk 'NF')"
	uncovered="$(awk 'NR == FNR { if (NF) seen[$0] = 1; next } NF && !($0 in seen)' \
		<(printf '%s\n' "$covered_paths") <(printf '%s\n' "$dirty_cov"))"
	n_uncovered="$(count_set "$uncovered")"
	n_covered=$(($(count_set "$dirty_cov") - n_uncovered))
}

# One flat, separator-delimited stream for the whole journal: an `E` line per entry
# followed by a `P` line per path. Tabs and newlines are squashed out of the display
# fields here, because `status` output is line-oriented and a multi-line `why` would
# forge a second key. Quotes and everything else survive untouched.
read_entries() {
	[ -f "$journal" ] || return 0
	jq -r --arg b "$branch" --arg s "$SEP" '
		def flat: gsub("[\t\n\r]"; " ") | gsub($s; " ");
		select(.kind == "unit") | select(.branch == $b) | . as $e |
		(["E", ($e.seq | tostring), ($e.type // ""), ($e.scope // ""),
		  ($e.source // ""), ($e.head // ""),
		  (($e.subject // "") | flat),
		  (($e.why // "") | flat)] | join($s)),
		($e.paths[]? | . as $p |
		  ["P", ($e.seq | tostring), $p, (($e.blobs[$p]) // "")] | join($s))
	' "$journal"
}

# Entries land in parallel arrays in append order, because append order *is* the
# dependency order a later commit plan has to preserve.
load_entries() {
	local tag f2 f3 f4 f5 f6 f7 f8 i=0
	n_entries=0
	while IFS=$SEP read -r tag f2 f3 f4 f5 f6 f7 f8; do
		case "$tag" in
		E)
			i=$((i + 1))
			e_seq[i]="$f2"
			e_type[i]="$f3"
			e_scope[i]="$f4"
			e_source[i]="$f5"
			e_head[i]="$f6"
			e_subject[i]="$f7"
			e_why[i]="$f8"
			e_paths[i]=""
			e_blobs[i]=""
			e_class[i]=""
			;;
		P)
			[ "$i" -gt 0 ] || continue
			e_paths[i]="${e_paths[i]}${e_paths[i]:+$NL}$f3"
			e_blobs[i]="${e_blobs[i]}${e_blobs[i]:+$NL}$f3$SEP$f4"
			;;
		esac
	done <<-EOF
		$(read_entries)
	EOF
	n_entries=$i
}

# The classification table from journal.md, run here. Precedence is the whole point:
#
#   unknown-head   the recorded head no longer resolves (gc after a rebase, a clone) —
#                  nothing below can be computed against it, and it must never read as
#                  `fresh`, so it wins outright
#   fresh          every path still dirty and every hash still matching
#   drifted        every path still dirty, some hash moved — or only *some* paths still
#                  dirty, which means the unit was partly consumed and the remaining
#                  intent still applies. `committed` there would drop a live `why`
#   committed      no path dirty any more, and a commit in <head>..HEAD touched them
#   orphaned       no path dirty any more, nothing committed them — reverted, or the
#                  old half of a rename
classify_entry() {
	local i="$1" p b head n_paths=0 n_dirty=0
	local -a pa=()
	head="${e_head[i]}"
	if [ -n "$head" ] && ! git rev-parse --verify --quiet "$head^{commit}" >/dev/null 2>&1; then
		printf 'unknown-head\n'
		return 0
	fi
	while IFS= read -r p; do
		[ -n "$p" ] || continue
		n_paths=$((n_paths + 1))
		# `:(literal)`, because pa is only ever a pathspec for the rev-list below and a
		# recorded name is a name, not a pattern: a file called `a*.txt` would otherwise
		# match every sibling and report a commit that never touched it — the same class
		# of round-trip break as git's quoting.
		pa[${#pa[@]}]=":(literal)$p"
		if set_has "$dirty_all" "$p"; then
			n_dirty=$((n_dirty + 1))
		fi
	done <<<"${e_paths[i]}"
	if [ "$n_paths" -eq 0 ]; then
		printf 'orphaned\n'
		return 0
	fi
	if [ "$n_dirty" -eq "$n_paths" ]; then
		while IFS=$SEP read -r p b; do
			[ -n "$p" ] || continue
			if [ "$(blob_of "$p")" != "$b" ]; then
				printf 'drifted\n'
				return 0
			fi
		done <<<"${e_blobs[i]}"
		printf 'fresh\n'
		return 0
	fi
	if [ "$n_dirty" -gt 0 ]; then
		printf 'drifted\n'
		return 0
	fi
	if [ -n "$head" ] && [ -n "$(git rev-list "$head..HEAD" -- "${pa[@]}" 2>/dev/null)" ]; then
		printf 'committed\n'
	else
		printf 'orphaned\n'
	fi
}

classify_all() {
	local i=1
	n_fresh=0
	n_drifted=0
	n_committed=0
	n_orphaned=0
	n_unknown_head=0
	while [ "$i" -le "$n_entries" ]; do
		e_class[i]="$(classify_entry "$i")"
		case "${e_class[i]}" in
		fresh) n_fresh=$((n_fresh + 1)) ;;
		drifted) n_drifted=$((n_drifted + 1)) ;;
		committed) n_committed=$((n_committed + 1)) ;;
		orphaned) n_orphaned=$((n_orphaned + 1)) ;;
		unknown-head) n_unknown_head=$((n_unknown_head + 1)) ;;
		esac
		i=$((i + 1))
	done
}

# Seqs of every entry in one class, comma-joined for jq.
seqs_of_class() {
	local want="$1" i=1 out=""
	while [ "$i" -le "$n_entries" ]; do
		if [ "${e_class[i]}" = "$want" ]; then
			out="$out${out:+,}${e_seq[i]}"
		fi
		i=$((i + 1))
	done
	printf '%s\n' "$out"
}

# --- the mutation lock -------------------------------------------------------------
# Held across seq allocation *and* the append in `add`, and across the whole
# read-classify-rewrite in `drop`/`compact`. Without it, two `add` calls that read
# max(seq) in the same instant both write seq=1 — reproduced deterministically — and a
# later `drop --committed` (which the `commit` skill runs at its final report) then
# deletes every record sharing that seq, live `why` included. The trigger is the designed
# automatic path, not an exotic one: parallel subagents each fire SubagentStop and each is
# nudged to call `add`.
#
# `mkdir` is the atomic primitive available wherever bash 3.2 is — macOS has no `flock`.
# Uncontended it is one syscall, which is what keeps `add` fast enough for a hook-driven
# turn. The lock lives beside the journal, so it dies with the worktree like everything
# else under the mkit dir.
#
# Stale locks are *stolen*, not fatal: a process killed between `mkdir` and its trap would
# otherwise wedge every later `add` forever. Wait up to ~5s, then break the lock and take
# it. Stealing risks the race we just fixed; failing loudly instead loses the `why` and
# hands the agent an error it cannot act on — and 5s is over an order of magnitude longer
# than a whole `add`, so a live holder is essentially never stolen from.
lock_dir="$mkit_dir/journal.lock"
lock_held=no

journal_unlock() {
	[ "$lock_held" = yes ] || return 0
	lock_held=no
	rm -rf -- "$lock_dir" 2>/dev/null || true
}

journal_lock() {
	local tries=0
	mkdir -p "$mkit_dir"
	while ! mkdir "$lock_dir" 2>/dev/null; do
		tries=$((tries + 1))
		if [ "$tries" -ge 100 ]; then
			rm -rf -- "$lock_dir" 2>/dev/null || true
			mkdir "$lock_dir" 2>/dev/null || true
			break
		fi
		sleep 0.05
	done
	lock_held=yes
	# Every exit path releases: `mkit_die` exits, errexit exits, and ^C or a kill during
	# the rewrite would otherwise leave the lock for the next caller to wait out.
	trap 'journal_unlock' EXIT
	trap 'journal_unlock; exit 130' INT
	trap 'journal_unlock; exit 143' TERM
}

# Rewrite the journal without the named seqs, through a temp file in the same
# directory so the `mv` is atomic. An interrupted rewrite must not lose the journal.
rewrite_without() {
	local drops="$1" renumber="$2" tmp
	tmp="$(mktemp "$journal.XXXXXX")"
	if [ "$renumber" = yes ]; then
		jq -c --argjson drops "[$drops]" 'select(.seq as $s | ($drops | index($s)) == null)' "$journal" |
			jq -c -s 'to_entries | map(.value + {seq: (.key + 1)}) | .[]' >"$tmp"
	else
		jq -c --argjson drops "[$drops]" 'select(.seq as $s | ($drops | index($s)) == null)' "$journal" >"$tmp"
	fi
	mv -f "$tmp" "$journal"
}

records_in_journal() {
	if [ -f "$journal" ]; then wc -l <"$journal" | tr -d ' '; else printf '0\n'; fi
}

# --- add ---------------------------------------------------------------------------
if [ "$cmd" = add ]; then
	declare -a paths_in=()
	type=""
	scope=""
	subject=""
	why=""
	source=note
	while [ $# -gt 0 ]; do
		case "$1" in
		--paths)
			[ $# -ge 2 ] || mkit_die '--paths needs a value' 2
			v="$2"
			[ -n "$v" ] || mkit_die '--paths needs at least one path' 2
			set -f
			old_ifs="$IFS"
			IFS=,
			for p in $v; do
				[ -n "$p" ] || continue
				rp="$(journal_relpath "$p")" || exit $?
				paths_in[${#paths_in[@]}]="$rp"
			done
			IFS="$old_ifs"
			set +f
			shift 2
			;;
		--type | --scope | --subject | --why | --source)
			# `shift 2` with only one argument left fails, and under `set -e` that
			# exits 1 printing nothing — while every other bad invocation exits 2 with
			# an `mkit:` line. Check the count first so the contract holds for all of
			# them; --scope is the one that may legitimately be empty.
			[ $# -ge 2 ] || mkit_die "$1 needs a value" 2
			case "$1" in
			--type) type="$2" ;;
			--scope) scope="$2" ;;
			--subject) subject="$2" ;;
			--why) why="$2" ;;
			--source) source="$2" ;;
			esac
			shift 2
			;;
		*) mkit_die "unknown option: $1" 2 ;;
		esac
	done

	[ ${#paths_in[@]} -gt 0 ] || mkit_die '--paths is required and needs at least one path' 2
	[ -n "$subject" ] || mkit_die '--subject is required and must not be empty' 2
	[ -n "$why" ] || mkit_die '--why is required and must not be empty' 2
	case "$source" in
	note | stop | subagent-stop) ;;
	*) mkit_die "--source must be note|stop|subagent-stop, got: $source" 2 ;;
	esac

	# Same path twice in one unit is a caller slip, not an error: dedupe in place,
	# keeping first-mention order.
	declare -a paths=()
	seen=""
	for p in "${paths_in[@]}"; do
		case "$seen" in *"$NL$p$NL"*) continue ;; esac
		seen="$seen$NL$p$NL"
		paths[${#paths[@]}]="$p"
	done

	pairs=""
	for p in "${paths[@]}"; do
		[ ! -d "$p" ] || mkit_die "--paths names a directory, name the files: $p" 2
		pairs="$pairs$p$TAB$(blob_of "$p")$NL"
	done

	# JSON is built by jq, never by string interpolation: a `why` containing a quote or
	# a newline must land as data, not as a broken line that poisons every later read.
	paths_json="$(printf '%s\n' "${paths[@]}" |
		jq -R -s 'split("\n") | map(select(length > 0))')"
	blobs_json="$(printf '%s' "$pairs" |
		jq -R -s 'split("\n") | map(select(length > 0) | split("\t"))
		          | map({(.[0]): (.[1] // "")}) | add // {}')"

	mkdir -p "$mkit_dir"
	# Locked from here: reading max(seq) and appending the line that claims it are one
	# operation, and any gap between them is a duplicate seq.
	journal_lock
	seq=1
	if [ -s "$journal" ]; then
		seq="$(jq -s 'map(.seq // 0) | max // 0' "$journal")"
		seq=$((seq + 1))
	fi
	line="$(jq -cn \
		--argjson seq "$seq" \
		--arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--arg branch "$branch" \
		--arg head "$(git rev-parse HEAD 2>/dev/null || true)" \
		--argjson paths "$paths_json" \
		--argjson blobs "$blobs_json" \
		--arg type "$type" \
		--arg scope "$scope" \
		--arg subject "$subject" \
		--arg why "$why" \
		--arg source "$source" \
		'{kind: "unit", seq: $seq, ts: $ts, branch: $branch, head: $head,
		  paths: $paths, blobs: $blobs, type: $type, scope: $scope,
		  subject: $subject, why: $why, source: $source}')"
	printf '%s\n' "$line" >>"$journal"
	journal_unlock
	printf 'added seq=%d paths=%d\n' "$seq" "${#paths[@]}"
	exit 0
fi

# --- uncovered: the hook's fast path ----------------------------------------------
# Still no classification pass — but not a bare set difference either, and the earlier
# claim here that treating every entry as covering its paths is sound was exactly wrong.
# It is sound only about *existence*: a path that is still dirty cannot be `committed` or
# `orphaned`. It says nothing about intent — a path edited again after its entry was
# recorded has a second reason nobody has stated, and counting the stale claim as coverage
# silenced the hook for precisely the paths the journal exists to explain.
#
# So load_coverage hash-checks each claim on a currently dirty path, and skips what
# classification needs history for (`git rev-list`, the committed/orphaned split), which
# cannot apply to a dirty path anyway. That keeps this the cheap call the hook needs.
if [ "$cmd" = uncovered ]; then
	paths_max=200
	while [ $# -gt 0 ]; do
		case "$1" in
		--paths-max)
			[ $# -ge 2 ] || mkit_die '--paths-max needs a value' 2
			case "$2" in '' | *[!0-9]*) mkit_die "--paths-max takes a count, got: $2" 2 ;; esac
			[ "$2" -ge 1 ] || mkit_die '--paths-max must be at least 1' 2
			paths_max="$2"
			shift 2
			;;
		*) mkit_die 'usage: uncovered [--paths-max N]' 2 ;;
		esac
	done
	load_dirty
	load_coverage
	printf 'journal_uncovered=%d\n' "$n_uncovered"
	if [ "$n_uncovered" -gt 0 ]; then
		# Bounded, and it says when it truncated — same contract as every facts.sh
		# list. The hook pastes this straight into additionalContext on every prompt,
		# so an unbounded list is unbounded prompt: a first-run repo with a thousand
		# untracked files would inject the whole tree, every turn.
		printf 'uncovered:\n'
		printf '%s\n' "$uncovered" | head -"$paths_max"
		[ "$n_uncovered" -gt "$paths_max" ] &&
			printf '... %d more not shown (paths_max=%d)\n' \
				"$((n_uncovered - paths_max))" "$paths_max"
	fi
	exit 0
fi

# --- status -----------------------------------------------------------------------
if [ "$cmd" = status ]; then
	[ $# -eq 0 ] || mkit_die 'status takes no arguments' 2
	load_dirty
	load_coverage
	load_entries
	classify_all

	printf 'journal_entries=%d\n' "$n_entries"
	if [ "$n_entries" -gt 0 ]; then
		printf 'journal_fresh=%d journal_drifted=%d journal_committed=%d' \
			"$n_fresh" "$n_drifted" "$n_committed"
		printf ' journal_orphaned=%d journal_unknown_head=%d\n' \
			"$n_orphaned" "$n_unknown_head"
	fi
	printf 'journal_covered=%d journal_uncovered=%d\n' "$n_covered" "$n_uncovered"

	if [ "$n_entries" -gt 0 ]; then
		printf 'entries:\n'
		i=1
		while [ "$i" -le "$n_entries" ]; do
			printf 'seq=%s class=%s type=%s scope=%s source=%s paths=%s\n' \
				"${e_seq[i]}" "${e_class[i]}" "${e_type[i]}" "${e_scope[i]}" \
				"${e_source[i]}" \
				"$(printf '%s\n' "${e_paths[i]}" | awk 'NF' | tr '\n' ',' | sed 's/,$//')"
			printf 'subject=%s\n' "${e_subject[i]}"
			printf 'why=%s\n' "${e_why[i]}"
			i=$((i + 1))
		done
	fi

	if [ "$n_uncovered" -gt 0 ]; then
		printf 'uncovered:\n'
		printf '%s\n' "$uncovered"
	fi

	# A path two *live* entries both claim needs patch staging. Name it and stop: which
	# hunk belongs to which unit is the skill's call, on the diff, not this script's.
	#
	# Live means fresh, drifted or unknown-head — every class whose paths are still
	# dirty. Restricting this to `fresh` missed the commonest real overlap: two units
	# touch one file, so editing it for the second drifts the first, and the pair that
	# most needs patch staging is exactly the pair that stops being reported.
	fresh_pairs=""
	i=1
	while [ "$i" -le "$n_entries" ]; do
		case "${e_class[i]}" in
		fresh | drifted | unknown-head) live=yes ;;
		*) live=no ;;
		esac
		if [ "$live" = yes ]; then
			while IFS= read -r p; do
				[ -n "$p" ] || continue
				fresh_pairs="$fresh_pairs$p$TAB${e_seq[i]}$NL"
			done <<<"${e_paths[i]}"
		fi
		i=$((i + 1))
	done
	overlap="$(printf '%s' "$fresh_pairs" | awk -F"$TAB" '
		NF >= 2 {
			if (!($1 in cnt)) { order[++k] = $1; acc[$1] = $2 }
			else acc[$1] = acc[$1] "," $2
			cnt[$1]++
		}
		END { for (j = 1; j <= k; j++) if (cnt[order[j]] > 1)
			printf "%s seq=%s\n", order[j], acc[order[j]] }')"
	if [ -n "$overlap" ]; then
		printf 'overlap:\n'
		printf '%s\n' "$overlap"
	fi
	exit 0
fi

# --- drop / compact ---------------------------------------------------------------
# Both rewrite, so both need a journal — but the arguments are checked first: a bad
# invocation is a bad invocation whether or not this repo has ever journaled.

if [ "$cmd" = drop ]; then
	target="${1:-}"
	[ -n "$target" ] || mkit_die 'usage: journal.sh drop --committed | --orphaned | <seq>' 2
	[ $# -eq 1 ] || mkit_die 'drop takes exactly one argument' 2
	case "$target" in
	--committed | --orphaned) ;;
	*[!0-9]*) mkit_die "drop takes --committed, --orphaned or a seq number, got: $target" 2 ;;
	esac
	[ -f "$journal" ] || mkit_die "no journal to rewrite: $journal" 1
	# Locked across the classification *and* the rewrite: the seqs to drop are decided
	# from one read of the journal and applied to another, so an `add` landing between
	# the two would be dropped by a seq it never saw allocated.
	journal_lock
	case "$target" in
	--committed | --orphaned)
		load_dirty
		load_entries
		classify_all
		drops="$(seqs_of_class "${target#--}")"
		;;
	*) drops="$target" ;;
	esac
	if [ -z "$drops" ]; then
		journal_unlock
		printf 'dropped 0\n'
		exit 0
	fi
	before="$(records_in_journal)"
	rewrite_without "$drops" no
	dropped="$((before - $(records_in_journal)))"
	journal_unlock
	printf 'dropped %d\n' "$dropped"
	exit 0
fi

if [ "$cmd" = compact ]; then
	[ $# -eq 0 ] || mkit_die 'compact takes no arguments' 2
	[ -f "$journal" ] || mkit_die "no journal to rewrite: $journal" 1
	# Same read-and-rewrite as `drop`, plus a renumber — which makes a concurrent `add`
	# worse still, since every surviving seq moves under it.
	journal_lock
	load_dirty
	load_entries
	classify_all
	drops="$(seqs_of_class committed)"
	orphans="$(seqs_of_class orphaned)"
	drops="$drops${drops:+${orphans:+,}}$orphans"
	before="$(records_in_journal)"
	rewrite_without "$drops" yes
	after="$(records_in_journal)"
	journal_unlock
	printf 'compacted %d -> %d\n' "$before" "$after"
	exit 0
fi

#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

# `status` packs several key=value pairs per line (journal_covered=1 journal_uncovered=1),
# so split on whitespace as well as newlines before matching the key.
field() { printf '%s\n' "$output" | tr ' ' '\n' | sed -n "s/^$1=//p" | head -1; }

journal_file() { printf '%s\n' "$MKIT_TMP/.git/mkit/journal.jsonl"; }

# Journaling must be enabled before `add` will write — that is the opt-in contract, and
# it has its own test below. Every other add-based test wants it already satisfied, so
# the helpers make it a precondition rather than restating it fifteen times.
journal_enabled_repo() {
	[ -f "$MKIT_TMP/.git/mkit/journal.enabled" ] ||
		"$SCRIPTS/journal.sh" enable >/dev/null
}

# One entry, with everything but the paths defaulted.
add_entry() {
	journal_enabled_repo
	"$SCRIPTS/journal.sh" add --paths "$1" --type feat --scope core \
		--subject "${2:-a subject}" --why "${3:-a reason}"
}

# --- the marker and the path -------------------------------------------------------

@test "enabled reports disabled until the marker exists" {
	run "$SCRIPTS/journal.sh" enabled
	[ "$status" -eq 0 ]
	[ "$output" = disabled ]
}

@test "enable creates the marker, disable removes it" {
	run "$SCRIPTS/journal.sh" enable
	[ "$status" -eq 0 ]
	[ -f "$MKIT_TMP/.git/mkit/journal.enabled" ]
	run "$SCRIPTS/journal.sh" enabled
	[ "$output" = enabled ]
	run "$SCRIPTS/journal.sh" disable
	[ "$status" -eq 0 ]
	run "$SCRIPTS/journal.sh" enabled
	[ "$output" = disabled ]
}

@test "path prints the journal location under <git-dir>/mkit" {
	run "$SCRIPTS/journal.sh" path
	[ "$status" -eq 0 ]
	[ "$output" = "$(journal_file)" ]
}

# --- the empty states --------------------------------------------------------------

@test "status on a clean tree with no journal reports nothing recorded, nothing dirty" {
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[ "$(field journal_entries)" = 0 ]
	[ "$(field journal_covered)" = 0 ]
	[ "$(field journal_uncovered)" = 0 ]
	[[ "$output" != *"entries:"* ]]
	[[ "$output" != *"uncovered:"* ]]
}

@test "uncovered with no journal at all still names every dirty path" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >untracked.txt
	run "$SCRIPTS/journal.sh" uncovered
	[ "$status" -eq 0 ]
	[ "$(field journal_uncovered)" = 2 ]
	[[ "$output" == *"seed.txt"* ]]
	[[ "$output" == *"untracked.txt"* ]]
}

@test "uncovered prints only the count and the list" {
	printf 'edit\n' >>seed.txt
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" -eq 3 ]
	[ "$(printf '%s\n' "$output" | sed -n 1p)" = "journal_uncovered=1" ]
	[ "$(printf '%s\n' "$output" | sed -n 2p)" = "uncovered:" ]
	[ "$(printf '%s\n' "$output" | sed -n 3p)" = "seed.txt" ]
}

@test "uncovered on a clean tree prints the count and no list" {
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]
}

# --- add ---------------------------------------------------------------------------

@test "add appends one record and reports its seq" {
	printf 'edit\n' >>seed.txt
	run add_entry seed.txt 'change the seed' 'the caller asked for it'
	[ "$status" -eq 0 ]
	[ "$output" = "added seq=1 paths=1" ]
	[ "$(wc -l <"$(journal_file)" | tr -d ' ')" -eq 1 ]
	run jq -r '.kind, .seq, .branch, .type, .scope, .subject, .why, .source, (.paths | length)' "$(journal_file)"
	[[ "$output" == *"unit"* ]]
	[[ "$output" == *"main"* ]]
	[[ "$output" == *"change the seed"* ]]
	[[ "$output" == *"note"* ]]
}

@test "add records the blob hash git would compute for the path" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	recorded="$(jq -r '.blobs["seed.txt"]' "$(journal_file)")"
	[ "$recorded" = "$(git hash-object seed.txt)" ]
}

@test "add takes a comma list and a repeated flag alike" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	printf 'a\n' >a.txt
	printf 'b\n' >b.txt
	run "$SCRIPTS/journal.sh" add --paths seed.txt,a.txt --paths b.txt \
		--type feat --scope '' --subject s --why w
	[ "$status" -eq 0 ]
	[ "$output" = "added seq=1 paths=3" ]
}

@test "add resolves a path relative to the caller's own directory" {
	mkdir -p sub
	printf 'x\n' >sub/f.txt
	cd sub
	run add_entry f.txt
	[ "$status" -eq 0 ]
	[ "$(jq -r '.paths[0]' "$(journal_file)")" = "sub/f.txt" ]
}

@test "add increments seq and never rewrites an earlier line" {
	printf 'edit\n' >>seed.txt
	printf 'a\n' >a.txt
	add_entry seed.txt
	first="$(sed -n 1p "$(journal_file)")"
	run add_entry a.txt
	[ "$output" = "added seq=2 paths=1" ]
	[ "$(sed -n 1p "$(journal_file)")" = "$first" ]
}

@test "a why containing a double quote survives the round trip" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt 'fix it' 'the "old" path was $wrong'
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[[ "$output" == *'why=the "old" path was $wrong'* ]]
}

@test "a why containing a newline cannot forge a second key line" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt 'fix it' "$(printf 'first\nsubject=forged')"
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[ "$(printf '%s\n' "$output" | grep -c '^subject=')" -eq 1 ]
	[[ "$output" == *"why=first subject=forged"* ]]
}

# --- classification ---------------------------------------------------------------

@test "an untouched entry over a dirty path is fresh" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 1 ]
	[[ "$output" == *"class=fresh"* ]]
}

@test "an untracked file is in scope and classifies fresh" {
	printf 'new\n' >new.txt
	add_entry new.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 1 ]
	[ "$(field journal_uncovered)" = 0 ]
}

@test "editing a recorded path after the fact makes the entry drifted" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	printf 'more\n' >>seed.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_fresh)" = 0 ]
	[[ "$output" == *"class=drifted"* ]]
}

@test "committing the recorded paths makes the entry committed" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	git add seed.txt
	git commit -q -m 'the change'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_committed)" = 1 ]
	[[ "$output" == *"class=committed"* ]]
}

@test "a reverted path leaves the entry orphaned" {
	printf 'new\n' >gone.txt
	add_entry gone.txt
	rm gone.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_orphaned)" = 1 ]
	[[ "$output" == *"class=orphaned"* ]]
}

@test "a recorded deletion is still fresh, with an empty blob" {
	git rm -q seed.txt
	add_entry seed.txt 'drop the seed' 'obsolete'
	[ "$(jq -r '.blobs["seed.txt"]' "$(journal_file)")" = "" ]
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 1 ]
	[[ "$output" == *"class=fresh"* ]]
}

@test "an unresolvable head classifies unknown-head, never fresh" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	jq -c '.head = "0123456789012345678901234567890123456789"' "$(journal_file)" >"$MKIT_TMP/j"
	mv "$MKIT_TMP/j" "$(journal_file)"
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_unknown_head)" = 1 ]
	[ "$(field journal_fresh)" = 0 ]
	[[ "$output" == *"class=unknown-head"* ]]
}

@test "a partly consumed entry is drifted, so its remaining why is not dropped" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	"$SCRIPTS/journal.sh" add --paths seed.txt,a.txt --type feat --scope core \
		--subject s --why w
	git add seed.txt
	git commit -q -m 'half of it'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_committed)" = 0 ]
}

@test "entries are reported in append order" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt 'first unit'
	add_entry a.txt 'second unit'
	run "$SCRIPTS/journal.sh" status
	first_line="$(printf '%s\n' "$output" | grep -n 'first unit' | cut -d: -f1)"
	second_line="$(printf '%s\n' "$output" | grep -n 'second unit' | cut -d: -f1)"
	[ "$first_line" -lt "$second_line" ]
}

@test "entries recorded on another branch are not reported here" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	git checkout -q -b other
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_entries)" = 0 ]
	[ "$(field journal_uncovered)" = 1 ]
}

# --- coverage and overlap ---------------------------------------------------------

@test "coverage splits the dirty set into covered and uncovered" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_covered)" = 1 ]
	[ "$(field journal_uncovered)" = 1 ]
	[[ "$output" == *"uncovered:"*"a.txt"* ]]
}

@test "*.lock and *.snap are excluded from the coverage math" {
	# Real names, not invented ones: `deps.lock` matched the glob by construction.
	printf 'dep\n' >Cargo.lock
	printf 'snap\n' >ui.snap
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]
}

@test "a path two fresh entries both claim is reported under overlap" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt 'first'
	add_entry seed.txt 'second'
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[[ "$output" == *"overlap:"* ]]
	[[ "$output" == *"seed.txt seq=1,2"* ]]
}

@test "no overlap block when each path is claimed once" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt
	add_entry a.txt
	run "$SCRIPTS/journal.sh" status
	[[ "$output" != *"overlap:"* ]]
}

# --- drop and compact -------------------------------------------------------------

@test "drop takes a single seq" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt
	add_entry a.txt
	run "$SCRIPTS/journal.sh" drop 1
	[ "$status" -eq 0 ]
	[ "$output" = "dropped 1" ]
	[ "$(jq -r '.seq' "$(journal_file)")" = 2 ]
}

@test "drop --committed removes only the committed entries" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt
	add_entry a.txt
	git add seed.txt
	git commit -q -m 'one of them'
	run "$SCRIPTS/journal.sh" drop --committed
	[ "$output" = "dropped 1" ]
	[ "$(jq -r '.paths[0]' "$(journal_file)")" = "a.txt" ]
}

@test "drop --orphaned removes only the orphaned entries" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >gone.txt
	add_entry seed.txt
	add_entry gone.txt
	rm gone.txt
	run "$SCRIPTS/journal.sh" drop --orphaned
	[ "$output" = "dropped 1" ]
	[ "$(jq -r '.paths[0]' "$(journal_file)")" = "seed.txt" ]
}

@test "drop reports zero when nothing matches" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	run "$SCRIPTS/journal.sh" drop --committed
	[ "$status" -eq 0 ]
	[ "$output" = "dropped 0" ]
	[ "$(wc -l <"$(journal_file)" | tr -d ' ')" -eq 1 ]
}

@test "compact drops the spent entries and renumbers from 1" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	printf 'new\n' >gone.txt
	add_entry seed.txt
	add_entry gone.txt
	add_entry a.txt
	rm gone.txt
	git add seed.txt
	git commit -q -m 'one of them'
	run "$SCRIPTS/journal.sh" compact
	[ "$status" -eq 0 ]
	[ "$output" = "compacted 3 -> 1" ]
	[ "$(jq -r '.seq' "$(journal_file)")" = 1 ]
	[ "$(jq -r '.paths[0]' "$(journal_file)")" = "a.txt" ]
}

@test "compact renumbers the survivors of an earlier drop" {
	printf 'edit\n' >>seed.txt
	printf 'new\n' >a.txt
	add_entry seed.txt
	add_entry a.txt
	"$SCRIPTS/journal.sh" drop 1
	run "$SCRIPTS/journal.sh" compact
	[ "$output" = "compacted 1 -> 1" ]
	[ "$(jq -r '.seq' "$(journal_file)")" = 1 ]
}

@test "drop and compact fail loudly when there is no journal" {
	run "$SCRIPTS/journal.sh" drop --committed
	[ "$status" -eq 1 ]
	[[ "$output" == *"no journal"* ]]
	run "$SCRIPTS/journal.sh" compact
	[ "$status" -eq 1 ]
}

# --- rejections -------------------------------------------------------------------

@test "add rejects a path outside the repository" {
	journal_enabled_repo
	run "$SCRIPTS/journal.sh" add --paths ../outside.txt --type feat --scope '' \
		--subject s --why w
	[ "$status" -eq 2 ]
	[[ "$output" == *"outside the repository"* ]]
	run "$SCRIPTS/journal.sh" add --paths /etc/hosts --type feat --scope '' \
		--subject s --why w
	[ "$status" -eq 2 ]
}

@test "add rejects an empty subject" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	run "$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope '' \
		--subject '' --why w
	[ "$status" -eq 2 ]
	[[ "$output" == *"--subject is required"* ]]
	[ ! -f "$(journal_file)" ]
}

@test "add rejects an empty why and a missing --paths" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	run "$SCRIPTS/journal.sh" add --paths seed.txt --type feat --subject s --why ''
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" add --type feat --subject s --why w
	[ "$status" -eq 2 ]
}

@test "add rejects a directory, which has no blob to hash" {
	mkdir -p sub
	printf 'x\n' >sub/f.txt
	run add_entry sub
	[ "$status" -eq 2 ]
	[[ "$output" == *"directory"* ]]
}

@test "add rejects an unknown --source" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	run "$SCRIPTS/journal.sh" add --paths seed.txt --type feat --subject s --why w \
		--source hook
	[ "$status" -eq 2 ]
	[[ "$output" == *"--source must be"* ]]
}

@test "add accepts each documented --source" {
	journal_enabled_repo
	printf 'edit\n' >>seed.txt
	for s in note stop subagent-stop; do
		run "$SCRIPTS/journal.sh" add --paths seed.txt --type feat --subject s \
			--why w --source "$s"
		[ "$status" -eq 0 ]
	done
	run "$SCRIPTS/journal.sh" status
	[[ "$output" == *"source=subagent-stop"* ]]
}

@test "bad usage exits 2" {
	journal_enabled_repo
	run "$SCRIPTS/journal.sh"
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" nonsense
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" add --nope x
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" status extra
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" drop
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" drop not-a-seq
	[ "$status" -eq 2 ]
}

@test "fails outside a git repository" {
	cd "$MKIT_TMP/.."
	run "$SCRIPTS/journal.sh" enabled
	[ "$status" -eq 1 ]
	[[ "$output" == *"not inside a git repository"* ]]
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 1 ]
}

# --- the opt-in gate on `add` (the one mutating subcommand) -------------------------

@test "add refuses when journaling is not enabled for the repo" {
	# Without this the record is written and then never read: facts.sh reports
	# journal=off and never opens the file, so the intent is captured and silently
	# dropped. note/SKILL.md promises this refusal and relays it.
	printf 'x\n' >a.txt
	run "$SCRIPTS/journal.sh" add --paths a.txt --type feat --scope core \
		--subject "s" --why "w"
	[ "$status" -eq 1 ]
	[[ "$output" == *"journaling is not enabled"* ]]
	[ ! -f "$(journal_file)" ]
}

@test "the read-only subcommands still work on a disabled repo" {
	# A journal written before `disable` must stay inspectable and cleanable.
	printf 'x\n' >a.txt
	add_entry a.txt
	"$SCRIPTS/journal.sh" disable >/dev/null
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[ "$(field journal_entries)" = 1 ]
	run "$SCRIPTS/journal.sh" uncovered
	[ "$status" -eq 0 ]
	run "$SCRIPTS/journal.sh" compact
	[ "$status" -eq 0 ]
}

# --- an option with a missing value is a usage error, not a bare exit 1 ------------

@test "every option with a missing value exits 2 with a message" {
	journal_enabled_repo
	printf 'x\n' >a.txt
	for flag in --paths --type --scope --subject --why --source; do
		run "$SCRIPTS/journal.sh" add --paths a.txt --subject s --why w "$flag"
		[ "$status" -eq 2 ]
		[[ "$output" == *"mkit:"* ]]
		[[ "$output" == *"needs a value"* ]] || [[ "$output" == *"needs at least one path"* ]]
	done
}

# --- the uncovered list is bounded, because the hook injects it verbatim -----------

@test "uncovered caps the path list and says how many it dropped" {
	journal_enabled_repo
	for i in $(seq 1 12); do printf 'x\n' >"f$i.txt"; done
	run "$SCRIPTS/journal.sh" uncovered --paths-max 5
	[ "$status" -eq 0 ]
	[ "$(field journal_uncovered)" = 12 ]
	# five paths, plus the overflow line naming the remainder
	[ "$(printf '%s\n' "$output" | grep -c '^f[0-9]*\.txt$')" -eq 5 ]
	[[ "$output" == *"... 7 more not shown (paths_max=5)"* ]]
}

@test "uncovered prints no overflow line when everything fits" {
	journal_enabled_repo
	printf 'x\n' >a.txt
	run "$SCRIPTS/journal.sh" uncovered --paths-max 5
	[[ "$output" != *"more not shown"* ]]
}

@test "uncovered rejects a bad --paths-max" {
	run "$SCRIPTS/journal.sh" uncovered --paths-max abc
	[ "$status" -eq 2 ]
	run "$SCRIPTS/journal.sh" uncovered --paths-max 0
	[ "$status" -eq 2 ]
}

# --- overlap must see every live entry, not only the fresh ones --------------------

@test "overlap reports a path two live entries claim when one has drifted" {
	# The commonest real overlap: two units touch one file, so editing it for the
	# second drifts the first — and the pair that most needs patch staging was
	# exactly the pair a fresh-only test could never produce.
	journal_enabled_repo
	printf 'one\n' >shared.txt
	add_entry shared.txt "first unit"
	printf 'two\n' >>shared.txt
	add_entry shared.txt "second unit"
	run "$SCRIPTS/journal.sh" status
	[ "$status" -eq 0 ]
	[ "$(field journal_drifted)" = 1 ]
	[[ "$output" == *"overlap:"* ]]
	[[ "$output" == *"shared.txt seq=1,2"* ]]
}

# --- the *.lock exclusion is a glob, not an intent ---------------------------------

@test "a lockfile whose name does not match the glob is nudged like any other file" {
	# The exclusion is `*.lock` / `*.snap`. package-lock.json, pnpm-lock.yaml and
	# go.sum are lockfiles that do not match, and pinning that stops the comment
	# beside the pathspec drifting back into "lockfile churn never nudges".
	journal_enabled_repo
	printf '{}\n' >package-lock.json
	printf 'x\n' >go.sum
	printf 'x\n' >Cargo.lock
	run "$SCRIPTS/journal.sh" uncovered
	[ "$status" -eq 0 ]
	[ "$(field journal_uncovered)" = 2 ]
	[[ "$output" == *"package-lock.json"* ]]
	[[ "$output" == *"go.sum"* ]]
	[[ "$output" != *"Cargo.lock"* ]]
}


# --- assertions that actually fail ------------------------------------------------
#
# `[[ ... ]]` is avoided from here down, deliberately. On bash 3.2 — the bash these
# scripts target, and the one bats runs under on macOS — errexit does not fire for a
# failing `[[ ... ]]` compound, so such an assertion aborts a bats test only when it
# happens to be the test's last command. Every assertion below is a simple command or a
# `[ ... ]`, both of which do fail the test.
has_line() { printf '%s\n' "$1" | grep -qxF -- "$2"; }
has_text() { printf '%s\n' "$1" | grep -qF -- "$2"; }
lacks_text() { [ -z "$(printf '%s\n' "$1" | grep -F -- "$2" || true)" ]; }

# --- git's C-quoted path form must never reach the journal -------------------------
#
# By default git quotes any path that is not plain ASCII: `café.txt` comes out of
# `diff --name-only` / `ls-files` as the literal `"caf\303\251.txt"`. That literal
# matches nothing a caller can pass, and both halves of the break were reproduced —
# recording the real name left the entry `orphaned` with the path uncovered forever, and
# recording the quoted form left it `fresh` forever, since a nonexistent path hashes to
# "" and matched the "" that had been recorded.

@test "a non-ASCII path round trips through add, status and uncovered" {
	printf 'x\n' >'café.txt'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(field journal_uncovered)" = 1 ]
	has_line "$output" 'café.txt'
	lacks_text "$output" 'caf\303'

	run add_entry 'café.txt'
	[ "$status" -eq 0 ]
	[ "$(jq -r '.paths[0]' "$(journal_file)")" = 'café.txt' ]
	[ "$(jq -r '.blobs["café.txt"]' "$(journal_file)")" = "$(git hash-object 'café.txt')" ]

	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 1 ]
	[ "$(field journal_uncovered)" = 0 ]
	lacks_text "$output" 'caf\303'

	printf 'more\n' >>'café.txt'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_fresh)" = 0 ]
}

@test "a path with a space and a path with a quote both round trip" {
	journal_enabled_repo
	printf 'x\n' >'has space.txt'
	printf 'x\n' >'we"ird.txt'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(field journal_uncovered)" = 2 ]
	has_line "$output" 'has space.txt'
	has_line "$output" 'we"ird.txt'
	lacks_text "$output" 'we\"ird'

	add_entry 'has space.txt'
	add_entry 'we"ird.txt'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 2 ]
	[ "$(field journal_uncovered)" = 0 ]

	printf 'more\n' >>'has space.txt'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_fresh)" = 1 ]
}

@test "every git path list in the script is read raw and NUL-delimited" {
	# One missed call site is worse than none fixed: two sets read in different
	# encodings disagree with each other. So the sweep is pinned mechanically rather
	# than trusted to review.
	run grep -n -e '--name-only' -e 'ls-files' "$SCRIPTS/journal.sh"
	[ "$status" -eq 0 ]
	while IFS= read -r line; do
		body="$(printf '%s' "${line#*:}" | sed 's/^[[:space:]]*//')"
		case "$body" in '#'* | '') continue ;; esac
		printf '%s\n' "$body" | grep -qF -- 'git_paths'
		printf '%s\n' "$body" | grep -qF -- ' -z'
	done <<<"$output"
	[ "$(grep -c 'core.quotePath=false' "$SCRIPTS/journal.sh")" -ge 1 ]
}

# --- symlinks are hashed as links, not as their targets ----------------------------
#
# `[ -e ]` follows a symlink, so blob_of used to hash the *target's content*: a link
# whose target was edited read as drifted though the link never moved, one retargeted at
# a same-content file read as fresh, and a broken link hashed to "" and stayed fresh
# across every later retarget — which makes `commit` skip its exploratory read.

@test "a symlink is hashed as its link text, the way git stores one" {
	ln -s seed.txt vlink
	git add vlink
	add_entry vlink
	recorded="$(jq -r '.blobs["vlink"]' "$(journal_file)")"
	[ "$recorded" = "$(git rev-parse :vlink)" ]
	# and not the target's content, which is what `hash-object -- <link>` returns
	[ "$recorded" != "$(git hash-object seed.txt)" ]
}

@test "editing a symlink's target leaves the entry fresh: the link did not move" {
	printf 'one\n' >base.txt
	ln -s base.txt vlink
	git add base.txt vlink
	add_entry vlink
	printf 'two\n' >>base.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_fresh)" = 1 ]
	[ "$(field journal_drifted)" = 0 ]
}

@test "retargeting a symlink drifts the entry even when the new target is identical" {
	printf 'same\n' >a.txt
	printf 'same\n' >b.txt
	ln -s a.txt vlink
	git add a.txt b.txt vlink
	add_entry vlink
	ln -sfn b.txt vlink
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_fresh)" = 0 ]
}

@test "a broken symlink still hashes, so retargeting it drifts the entry" {
	ln -s missing-one blink
	git add blink
	add_entry blink
	[ "$(jq -r '.blobs["blink"]' "$(journal_file)")" = "$(git rev-parse :blink)" ]
	ln -sfn missing-two blink
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_fresh)" = 0 ]
}

# --- coverage consults freshness, so a re-edited path is asked about again ----------

@test "a path edited again after its entry is uncovered, not covered by the stale claim" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt 'first unit'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]

	printf 'more\n' >>seed.txt
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(field journal_uncovered)" = 1 ]
	has_line "$output" 'seed.txt'
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_covered)" = 0 ]
	[ "$(field journal_uncovered)" = 1 ]

	# recording the second reason covers it again
	add_entry seed.txt 'second unit' 'a different reason'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]
}

@test "a committed entry stops covering its paths the moment they are edited again" {
	# The committed -> drifted flip-back: the same code path as the case above, and the
	# multi-commit run where `drop --committed` has not run yet.
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	git add seed.txt
	git commit -q -m 'the change'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]
	printf 'again\n' >>seed.txt
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(field journal_uncovered)" = 1 ]
	has_line "$output" 'seed.txt'
}

@test "an entry whose head no longer resolves covers nothing" {
	printf 'edit\n' >>seed.txt
	add_entry seed.txt
	jq -c '.head = "0123456789012345678901234567890123456789"' "$(journal_file)" >"$MKIT_TMP/j"
	mv "$MKIT_TMP/j" "$(journal_file)"
	run "$SCRIPTS/journal.sh" uncovered
	[ "$(field journal_uncovered)" = 1 ]
}

@test "a recorded deletion still covers its path" {
	# blob "" is a recorded fact, not a missing one, and must keep covering.
	git rm -q seed.txt
	add_entry seed.txt 'drop the seed' 'obsolete'
	run "$SCRIPTS/journal.sh" uncovered
	[ "$output" = "journal_uncovered=0" ]
}

# --- mutations are serialized --------------------------------------------------------

@test "concurrent adds each allocate their own seq" {
	# Reproduced before the lock: two adds reading max(seq) together both wrote seq=1,
	# and `drop 1` then removed every record sharing it — a live `why` with it.
	journal_enabled_repo
	for i in 1 2 3 4 5 6; do printf 'x\n' >"f$i.txt"; done
	for i in 1 2 3 4 5 6; do
		"$SCRIPTS/journal.sh" add --paths "f$i.txt" --type feat --scope core \
			--subject "s$i" --why "w$i" >/dev/null &
	done
	wait
	[ "$(wc -l <"$(journal_file)" | tr -d ' ')" -eq 6 ]
	[ "$(jq -r '.seq' "$(journal_file)" | sort -un | wc -l | tr -d ' ')" -eq 6 ]
	run "$SCRIPTS/journal.sh" drop 1
	[ "$output" = "dropped 1" ]
	[ "$(wc -l <"$(journal_file)" | tr -d ' ')" -eq 5 ]
}

@test "a concurrent add and drop leave the journal consistent" {
	journal_enabled_repo
	printf 'x\n' >a.txt
	printf 'y\n' >b.txt
	add_entry a.txt
	"$SCRIPTS/journal.sh" drop 1 >/dev/null &
	"$SCRIPTS/journal.sh" add --paths b.txt --type feat --scope core \
		--subject s --why w >/dev/null &
	wait
	# whichever order they ran in, every surviving line is a whole record
	jq -e -s 'all(.kind == "unit")' "$(journal_file)" >/dev/null
	[ "$(jq -r '.paths[0]' "$(journal_file)" | grep -c 'b.txt')" -eq 1 ]
}

@test "add releases the lock, and steals one a killed process left behind" {
	# A bounded wait, then the lock is broken: a lock nobody holds must not wedge the
	# journal forever. This test deliberately waits the bound out, so it is the slow one.
	journal_enabled_repo
	printf 'x\n' >a.txt
	add_entry a.txt
	[ ! -d "$MKIT_TMP/.git/mkit/journal.lock" ]
	mkdir "$MKIT_TMP/.git/mkit/journal.lock"
	printf 'y\n' >b.txt
	run add_entry b.txt
	[ "$status" -eq 0 ]
	[ "$output" = "added seq=2 paths=1" ]
	[ ! -d "$MKIT_TMP/.git/mkit/journal.lock" ]
}

@test "one drifted path does not uncover the other paths of the same entry" {
	# Freshness is checked per path, not per entry class: an entry naming two files where
	# only one moved has already stated why the untouched one changed. Per-entry would
	# nudge both and ask the agent to re-explain what it just explained.
	journal_enabled_repo
	printf 'a\n' >a.txt
	printf 'b\n' >b.txt
	"$SCRIPTS/journal.sh" add --paths a.txt,b.txt --type feat --scope core \
		--subject s --why w >/dev/null
	printf 'more\n' >>b.txt
	run "$SCRIPTS/journal.sh" status
	[ "$(field journal_drifted)" = 1 ]
	[ "$(field journal_covered)" = 1 ]
	[ "$(field journal_uncovered)" = 1 ]
	has_line "$output" 'b.txt'
	[ -z "$(printf '%s\n' "$output" | sed -n '/^uncovered:$/,$p' | grep -xF 'a.txt' || true)" ]
}

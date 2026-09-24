#!/usr/bin/env bash
# Cut a release: bump plugin.json, commit `chore(release): X.Y.Z`, tag vX.Y.Z, push both.
# The tag starts GoReleaser, which builds the cask and writes the GitHub Release notes.
#
#   tools/release.sh [auto|patch|minor|major|X.Y.Z]    (default: auto)
#
# auto reads the Conventional Commits since the last tag: a breaking change is major (minor
# while pre-1.0), any feat is minor, anything else is patch. Human-run: it asks before it
# pushes, and refuses without a terminal to ask on.
set -euo pipefail

bump=${1:-auto}
manifest=plugin/.claude-plugin/plugin.json

die() { echo "release: $*" >&2; exit 1; }
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'

cd "$(git rev-parse --show-toplevel)"
[[ -t 0 ]] || die "needs a terminal to confirm on"
[[ $(git branch --show-current) == main ]] || die "release from main only"
dirty=$(git status --porcelain --untracked-files=no)
[[ -z $dirty ]] || die "commit or stash tracked changes first:"$'\n'"$dirty"
git fetch -q origin main
[[ -z $(git rev-list HEAD..origin/main) ]] || die "main is behind origin/main; pull first"

last=$(git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --abbrev=0)
[[ -n $(git rev-list "$last"..HEAD) ]] || die "no commits since $last"
[[ ${last#v} =~ $semver ]] || die "last tag $last is not vX.Y.Z"
IFS=. read -r major minor patch <<<"${last#v}"

subjects=$(git log "$last"..HEAD --no-merges --format='%s')
if [[ $bump == auto ]]; then
  # captured, not piped: under pipefail, grep -q exiting early SIGPIPEs git log into a false miss
  messages=$(git log "$last"..HEAD --no-merges --format='%s%n%b')
  if grep -qE '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:' <<<"$messages"; then
    bump=major
  elif grep -qE '^feat(\([^)]*\))?:' <<<"$subjects"; then
    bump=minor
  else
    bump=patch
  fi
  [[ $bump == major && $major == 0 ]] && bump=minor # pre-1.0: breaking is a minor bump
  why="auto: $bump"
else
  why="requested"
fi

case $bump in
  major) version="$((major + 1)).0.0" ;;
  minor) version="$major.$((minor + 1)).0" ;;
  patch) version="$major.$minor.$((patch + 1))" ;;
  *)
    [[ $bump =~ $semver ]] || die "bump must be auto, patch, minor, major or X.Y.Z, not '$bump'"
    IFS=. read -r a b c <<<"$bump"
    ((a > major || (a == major && (b > minor || (b == minor && c > patch))))) ||
      die "$bump is not above $last"
    version=$bump
    ;;
esac
! git rev-parse -q --verify "refs/tags/v$version" >/dev/null || die "tag v$version exists"

# `feat: 3, fix: 5, docs: 2` — what the version was decided from
counts=$(sed -nE 's/^([a-z]+)(\([^)]*\))?!?:.*/\1/p' <<<"$subjects" |
  sort | uniq -c | sort -rn | awk '{printf "%s%s: %s", sep, $2, $1; sep=", "}')
echo "$last → v$version ($why; ${counts:-no conventional commits})"
# the PRs merged into main — what the github-native notes will list
git log "$last"..HEAD --first-parent --merges --format='%s%x1f%b%x1e' |
  awk -v RS='\036' -F'\037' '$1 ~ /Merge pull request #/ {
    match($1, /#[0-9]+/); split($2, body, "\n"); print "  " substr($1, RSTART, RLENGTH) " " body[1] }'
read -r -p "release v$version and push to origin? [y/N] " answer
[[ $answer == [yY] ]] || die "aborted; nothing changed"

sed -i '' -E "s/^(  \"version\": \")[^\"]+(\",)$/\1$version\2/" "$manifest"
grep -q "\"version\": \"$version\"" "$manifest" || die "could not bump $manifest"
git add "$manifest"
git commit -q -m "chore(release): $version"
git tag -a "v$version" -m "v$version"
git push --atomic origin main "v$version"
echo "released v$version — notes: https://github.com/masterik/mk-toolkit/releases/tag/v$version"

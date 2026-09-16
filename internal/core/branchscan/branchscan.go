// Package branchscan classifies every local branch and worktree for a repo-wide
// cleanup: which branches are merged (locally, or via a PR git's own merge-base
// cannot see because of a squash merge), which still have an open PR, which were
// never pushed, and which worktree each one owns.
//
// It reports candidates. It never deletes a branch, removes a worktree, or
// touches a remote — `git fetch --prune` is the one mutation, and it only ever
// updates this repo's own remote-tracking refs.
//
// Layering: returns data, never prints, never assumes a terminal.
package branchscan

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// Class is what a branch is, not what to do about it. The skill maps a class to
// an action; this package does not decide what counts as "safe to delete".
type Class string

// The classes, in the precedence order Run evaluates them.
const (
	// Protected — the default branch, a develop-like branch, or a name pinned in
	// the repo config's `[cleanup] keep`. Never a candidate.
	Protected Class = "protected"
	// Current — you cannot delete the branch you are standing on, however merged
	// it already is. Ahead of Merged for exactly that reason.
	Current Class = "current"
	// Merged — git itself proves it: an ancestor of a protected branch.
	Merged Class = "merged"
	// MergedPR — not an ancestor locally (a squash or rebase merge changes the
	// commit SHAs), but GitHub says the PR merged.
	MergedPR Class = "merged-pr"
	// OpenPR — unmerged work with an open PR.
	OpenPR Class = "open-pr"
	// ClosedPR — unmerged work whose PR was closed without merging.
	ClosedPR Class = "closed-pr"
	// Gone — the upstream existed and was deleted remotely, with no PR trail found.
	Gone Class = "gone"
	// Unpushed — never had an upstream at all.
	Unpushed Class = "unpushed"
	// Tracking — a live upstream, and not merged. Probably still in use elsewhere.
	Tracking Class = "tracking"
)

// Branch is one local branch.
type Branch struct {
	Name  string
	Class Class
	// Upstream is `none`, `gone`, or `<remote>/<name>`.
	Upstream string
	// MergedInto lists the protected branches this one is an ancestor of.
	MergedInto []string
	// PR is `open#12` / `merged#12` / `closed#12`, or empty.
	PR string
}

// Worktree is one worktree, the primary included.
type Worktree struct {
	// Branch is the checked-out branch, or "HEAD" when detached.
	Branch string
	Path   string
	// Origin is primary, claude-code or linked.
	Origin string
	// Clean is yes, no, missing (the path is gone) or error (`git status` itself
	// failed there). `error` never collapses into `yes`: an unreadable worktree
	// is not a proven-clean one.
	Clean string
}

// Scan is the whole report.
type Scan struct {
	Default string
	// Develop is the first of develop/development/dev that exists locally.
	Develop string
	// Protected is Default, plus Develop, plus every pinned Keep name that exists
	// as a local branch. Kept as a list, never a joined string: a branch name may
	// legally contain a comma.
	Protected []string
	// Keep is the pinned `[cleanup] keep` list exactly as the config gave it,
	// reported so a caller can tell a repo that pinned nothing from one whose pins
	// all resolved.
	Keep []string
	// KeepUnknown is every pinned name with no local branch of that name. **Not an
	// error**: a keep list travels with the repo, and a long-lived branch nobody
	// has checked out in this clone is the ordinary state, not a mistake. It is
	// reported so `cleanup` can say the pin had nothing to protect here instead of
	// silently dropping it.
	KeepUnknown []string
	Remote      string
	// Fetch is ok, skipped, no-remote or failed.
	Fetch string
	// WorktreesState is ok or unreadable. `cleanup` plans teardown from these
	// rows, so an enumeration git could not complete must not read as a repo
	// that simply has fewer worktrees.
	WorktreesState string
	// GH is ok, skipped, no-remote, gh-missing, gh-unauthenticated or gh-error.
	GH        string
	Branches  []Branch
	Worktrees []Worktree
}

// Options are `branch scan`'s knobs.
type Options struct {
	// Default is the branch `facts.sh` already resolved. This package never
	// re-derives it, so there is exactly one place that logic lives.
	Default string
	// Keep is the repo config's `[cleanup] keep`. Read by the caller and passed
	// in, for the same reason Default is: this package classifies, it does not
	// decide where an answer comes from.
	Keep    []string
	NoFetch bool
	NoGH    bool
}

// pullRequest is one PR as the batched lookup returns it.
type pullRequest struct {
	HeadRefName string `json:"headRefName"`
	Number      int    `json:"number"`
	State       string `json:"state"`
	HeadRefOid  string `json:"headRefOid"`
}

// Run performs the scan. The only error it returns is a caller mistake: every
// environmental failure is a reported state instead.
func Run(repo *gitrepo.Repo, opt Options) (*Scan, error) {
	if opt.Default == "" {
		return nil, fmt.Errorf("--default needs a branch name")
	}
	if err := run(repo, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+opt.Default); err != nil {
		return nil, fmt.Errorf("--default branch does not exist locally: %s", opt.Default)
	}

	s := &Scan{Default: opt.Default, Develop: Develop(repo, opt.Default), Keep: opt.Keep}
	local, unknown := splitKeep(repo, opt.Keep)
	s.Protected, s.KeepUnknown = ProtectedSet(opt.Default, s.Develop, local), unknown

	current, _ := git(repo, "branch", "--show-current")
	s.Remote = firstLine(mustGit(repo, "remote"))
	s.Fetch = fetch(repo, s.Remote, opt.NoFetch)

	prs, ghState := lookupPRs(repo, s.Remote, opt.NoGH)
	s.GH = ghState

	s.Branches = classify(repo, s, current, prs)
	s.Worktrees, s.WorktreesState = worktrees(repo)
	return s, nil
}

// fetch updates this repo's remote-tracking refs. The one mutation, and the
// reason it exists: without it `upstream=gone` reflects a stale local view. It
// cannot delete, rename or otherwise touch a branch on the remote itself, which
// is the whole of what "local only" means here.
func fetch(repo *gitrepo.Repo, remote string, skip bool) string {
	switch {
	case skip:
		return "skipped"
	case remote == "":
		return "no-remote"
	}
	if err := run(repo, "git", "fetch", remote, "--prune", "-q"); err != nil {
		return "failed"
	}
	return "ok"
}

// lookupPRs makes one batched `gh` call and returns the newest PR per branch.
// Never a per-branch round trip.
//
// One value per distinct cause, the `pr=gh-missing` lesson from facts.sh: only
// some of these mean "there was nothing to look up". `jq-missing` and `no-cache`
// are gone — the JSON is decoded in process, so there is no second tool to miss
// and no temp file whose absence could take the scan down.
func lookupPRs(repo *gitrepo.Repo, remote string, skip bool) (map[string]pullRequest, string) {
	switch {
	case skip:
		return nil, "skipped"
	case remote == "":
		return nil, "no-remote"
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, "gh-missing"
	}
	if err := run(repo, "gh", "auth", "status"); err != nil {
		return nil, "gh-unauthenticated"
	}
	out, err := output(repo, "gh", "pr", "list", "--state", "all",
		"--json", "headRefName,number,state,headRefOid", "--limit", "500")
	if err != nil {
		return nil, "gh-error"
	}
	var list []pullRequest
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, "gh-error"
	}
	// The newest PR (by number) wins when a branch was opened, closed, reopened
	// and PR'd again — history a cleanup decision should see, not just whichever
	// the API returned first.
	newest := map[string]pullRequest{}
	for _, p := range list {
		if cur, ok := newest[p.HeadRefName]; !ok || p.Number > cur.Number {
			newest[p.HeadRefName] = p
		}
	}
	return newest, "ok"
}

func classify(repo *gitrepo.Repo, s *Scan, current string, prs map[string]pullRequest) []Branch {
	var out []Branch
	protected := map[string]bool{}
	for _, p := range s.Protected {
		protected[p] = true
	}
	for _, name := range lines(mustGit(repo, "for-each-ref", "refs/heads", "--format=%(refname:short)")) {
		if name == "" {
			continue
		}
		b := Branch{Name: name, Upstream: upstream(repo, name)}

		// **Default and Develop only, not the whole protected set.** A pinned
		// keep branch is a branch never to delete, not a new proof that other
		// work has landed: treating `merged into staging` as merged would turn a
		// key whose whole purpose is to delete less into one that deletes more.
		//
		// Tested directly rather than by splitting a comma-joined list: a branch
		// name may legally contain a comma, and splitting on one would silently
		// test a nonexistent fragment.
		for _, p := range []string{s.Default, s.Develop} {
			if p == "none" || p == "" {
				continue
			}
			if err := run(repo, "git", "merge-base", "--is-ancestor", name, p); err == nil {
				b.MergedInto = append(b.MergedInto, p)
			}
		}

		p, found := prs[name]
		state := strings.ToLower(p.State)
		if found {
			b.PR = fmt.Sprintf("%s#%d", state, p.Number)
		}

		switch {
		case protected[name]:
			b.Class = Protected
		case name == current:
			b.Class = Current
		case len(b.MergedInto) > 0:
			b.Class = Merged
		case found && state == "merged" && oidIsLocal(repo, name, p.HeadRefOid):
			b.Class = MergedPR
		case found && state == "open":
			b.Class = OpenPR
		case found && state == "closed":
			b.Class = ClosedPR
		case b.Upstream == "gone":
			b.Class = Gone
		case b.Upstream == "none":
			b.Class = Unpushed
		default:
			b.Class = Tracking
		}
		out = append(out, b)
	}
	return out
}

// DevelopCandidates are the develop-like names, in the order the first local one
// wins.
var DevelopCandidates = []string{"develop", "development", "dev"}

// Develop returns the first of DevelopCandidates that exists as a local branch,
// or "none".
//
// Only a *local* branch counts. A develop that exists only as origin/develop is
// not one of this repo's branches to keep — nothing here creates one, and this
// only ever reports on what is already checked out somewhere.
//
// Exported because `mkit repo profile` reports the set cleanup protects with
// nothing pinned, and that set has to be the one this package actually computes.
func Develop(repo *gitrepo.Repo, def string) string {
	for _, cand := range DevelopCandidates {
		if cand == def {
			continue
		}
		if err := run(repo, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+cand); err == nil {
			return cand
		}
	}
	return "none"
}

// ProtectedSet unions the default branch, the develop-like branch and a pinned keep
// list into the branches cleanup must never delete. The one producer of that set.
//
// **The default branch comes first and unconditionally.** A keep list that omits
// it is a mistake, not an instruction: config is an input, and this key only ever
// adds to what cleanup already protects. Order is default, develop, then the
// pinned names as written — stable, so two runs on one repo report the same list
// and a diff of it means something. Blank entries are dropped; they pin nothing.
func ProtectedSet(def, develop string, keep []string) []string {
	out := make([]string, 0, len(keep)+2)
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	add(def)
	if develop != "none" {
		add(develop)
	}
	for _, k := range keep {
		add(strings.TrimSpace(k))
	}
	return out
}

// splitKeep divides the pinned names into those that exist as a local branch here
// and those that do not. A name with no branch is reported, never refused and
// never dropped in silence — see Scan.KeepUnknown.
func splitKeep(repo *gitrepo.Repo, keep []string) (local, unknown []string) {
	for _, k := range keep {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if err := run(repo, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+k); err == nil {
			local = append(local, k)
		} else {
			unknown = append(unknown, k)
		}
	}
	return local, unknown
}

func upstream(repo *gitrepo.Repo, name string) string {
	ref, _ := git(repo, "for-each-ref", "refs/heads/"+name, "--format=%(upstream)")
	if ref == "" {
		return "none"
	}
	track, _ := git(repo, "for-each-ref", "refs/heads/"+name, "--format=%(upstream:track)")
	if strings.Contains(track, "gone") {
		return "gone"
	}
	return strings.TrimPrefix(ref, "refs/remotes/")
}

// oidIsLocal guards against a reused or coincidentally-matching branch name: a
// `merged` PR match is trusted only when its recorded head commit is this
// branch's own current tip, or an ancestor of it (the branch has not gained
// commits the PR never saw).
//
// A missing oid degrades to the pre-existing name-only behavior rather than
// refusing outright — rare, and not this guard's job to eliminate.
func oidIsLocal(repo *gitrepo.Repo, branch, oid string) bool {
	if oid == "" {
		return true
	}
	tip, err := git(repo, "rev-parse", "-q", "--verify", "refs/heads/"+branch)
	if err != nil || tip == "" {
		return false
	}
	if oid == tip {
		return true
	}
	if err := run(repo, "git", "cat-file", "-e", oid+"^{commit}"); err != nil {
		return false
	}
	return run(repo, "git", "merge-base", "--is-ancestor", branch, oid) == nil
}

// worktrees reports one row per worktree, the primary included, and whether the
// enumeration completed at all.
func worktrees(repo *gitrepo.Repo) ([]Worktree, string) {
	var out []Worktree
	primary, path := "", ""
	raw, err := git(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, "unreadable"
	}
	for _, line := range lines(raw) {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
			// `git worktree list` always lists the primary first, regardless of
			// which worktree the caller runs from.
			if primary == "" {
				primary = path
			}
		case strings.HasPrefix(line, "branch "):
			out = append(out, worktree(repo, path, strings.TrimPrefix(line, "branch "), primary))
			path = ""
		case strings.HasPrefix(line, "detached"):
			out = append(out, worktree(repo, path, "HEAD", primary))
			path = ""
		}
	}
	return out, "ok"
}

func worktree(repo *gitrepo.Repo, path, ref, primary string) Worktree {
	w := Worktree{Branch: strings.TrimPrefix(ref, "refs/heads/"), Path: path, Origin: "linked"}
	switch {
	case path == primary:
		w.Origin = "primary"
	case strings.Contains(path, "/.claude/worktrees/"):
		w.Origin = "claude-code"
	}
	if !isDir(path) {
		w.Clean = "missing"
		return w
	}
	// `:(exclude).mkit` for the same reason facts.sh carries it: the run
	// directory lives inside the working directory, so a worktree holding
	// nothing but mkit's own scratch read `clean=no`. That is a machine-consumed
	// key — cleanup demotes a `merged` branch from auto-delete to ask on it, then
	// offers `worktree remove --force` with its "discards uncommitted work"
	// sentence for a worktree whose only untracked file is a log mkit wrote.
	//
	// A failed `status` and an empty one must never collapse into the same
	// answer: an unreadable worktree is not a proven-clean one.
	out, err := output(repo, "git", "-C", path, "status", "--porcelain", "--", ".", ":(exclude).mkit")
	switch {
	case err != nil:
		w.Clean = "error"
	case len(strings.TrimSpace(string(out))) == 0:
		w.Clean = "yes"
	default:
		w.Clean = "no"
	}
	return w
}

func git(repo *gitrepo.Repo, args ...string) (string, error) {
	out, err := output(repo, "git", args...)
	return strings.TrimRight(string(out), "\n"), err
}

func mustGit(repo *gitrepo.Repo, args ...string) string {
	out, _ := git(repo, args...)
	return out
}

func output(repo *gitrepo.Repo, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo.Toplevel
	return cmd.Output()
}

func run(repo *gitrepo.Repo, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo.Toplevel
	return cmd.Run()
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

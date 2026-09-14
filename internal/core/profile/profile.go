// Package profile answers "how does this repo work" by merging what mkit can
// discover with what a human pinned in the repo config.
//
// Every value carries its Source, because the two are not interchangeable: a
// discovered value is re-derived every run and cannot go stale, a pinned one
// captured something inspection could not establish and can. ADR 0001 decision 3
// governs — config is an input, never a permission — so a Profile is complete and
// usable with no config file at all, and says so.
//
// Layering: returns data, never prints, never assumes a terminal.
package profile

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/pluginroot"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Source says where a value came from.
type Source string

const (
	// Discovered — re-derived from the repo every run; cannot go stale.
	Discovered Source = "discovered"
	// Pinned — read from the committed config; captures the undiscoverable.
	Pinned Source = "pinned"
	// Unavailable — neither, and the cause is worth reporting rather than
	// presenting an empty value as an answer.
	Unavailable Source = "unavailable"
)

// Value is a single string answer and where it came from.
type Value struct {
	Value  string `json:"value,omitempty"`
	Source Source `json:"source"`
	// Cause explains an Unavailable source. Never set otherwise.
	Cause string `json:"cause,omitempty"`
}

// List is a multi-valued answer and where it came from.
type List struct {
	Values []string `json:"values"`
	Source Source   `json:"source"`
	Cause  string   `json:"cause,omitempty"`
}

// GateStep is one quality-gate command in sequence.
type GateStep struct {
	// Step is a short label. Derived from the command for a discovered step —
	// naming, not deciding: which command to trust stays the skill's call.
	Step    string `json:"step"`
	Command string `json:"command"`
	Source  Source `json:"source"`
}

// Profile is the merged picture. It is data; formatting is cmd/'s job.
type Profile struct {
	Toplevel string            `json:"toplevel"`
	Config   repoconfig.Status `json:"config"`
	Gate     Gate              `json:"gate"`
	Spec     Spec              `json:"spec"`
	Scopes   List              `json:"commit_scopes"`
	Review   List              `json:"reviewers"`
	Merge    Value             `json:"merge_style"`
	Payload  PayloadInfo       `json:"payload"`
}

// Gate is the quality gate as a sequence.
type Gate struct {
	Steps []GateStep `json:"steps"`
	// Ecosystem is what discovery recognised, e.g. "go,just".
	Ecosystem string `json:"ecosystem,omitempty"`
	Cause     string `json:"cause,omitempty"`
}

// Spec is where specs and task graphs live.
type Spec struct {
	Store Value `json:"store"`
	Ref   Value `json:"ref"`
}

// PayloadInfo records whether the plugin payload was reachable. Reported for the
// payload's own sake — since M5 nothing in this profile is derived from it.
type PayloadInfo struct {
	Found   bool   `json:"found"`
	Dir     string `json:"dir,omitempty"`
	Via     string `json:"via,omitempty"`
	Version string `json:"version,omitempty"`
	Remedy  string `json:"remedy,omitempty"`
}

// Build assembles the profile for a work tree.
//
// Nothing here fails the caller: an unreachable payload, an unreadable doc and an
// absent config are each a reported state. That is the same rule the scripts
// follow — detect at the first call, turn it into a starting fact.
func Build(repo *gitrepo.Repo) (*Profile, error) {
	cfg, _, err := repoconfig.Load(repo.Toplevel)
	if err != nil {
		return nil, err
	}

	p := &Profile{
		Toplevel: repo.Toplevel,
		Config:   repoconfig.Stat(repo),
	}

	root, rerr := pluginroot.Find(repo.Toplevel)
	if rerr == nil {
		p.Payload = PayloadInfo{Found: true, Dir: root.Dir, Via: root.Via, Version: root.Version()}
	} else {
		p.Payload = PayloadInfo{Remedy: pluginroot.Remedy()}
	}

	p.Gate = buildGate(repo, cfg)
	p.Spec = discoverSpec(repo, cfg)
	p.Scopes = discoverScopes(repo, cfg)
	p.Review = discoverReviewers(repo, cfg)
	p.Merge = discoverMerge(repo, cfg)
	return p, nil
}

// buildGate reports the quality gate as a tagged sequence.
//
// The merge of pinned over discovered lives in `gate.Detect`, which is what
// `mkit gate detect` prints — so it exists once, and the profile consumes the
// tagged result rather than redoing it. Until M5 this delegated to the payload's
// gate-detect.sh; discovery no longer needs the payload at all, so an unreachable
// payload is no longer a cause for an empty gate.
func buildGate(repo *gitrepo.Repo, cfg *repoconfig.Config) Gate {
	var g Gate

	// The profile is a report, not a gate run: the ledger annotation would cost
	// a fingerprint on every `repo profile` and say nothing about how the repo
	// works.
	d, err := gate.Detect(repo, cfg, gate.DetectOptions{NoCache: true})
	if err != nil {
		g.Cause = "gate discovery failed: " + err.Error()
		return g
	}
	g.Ecosystem = strings.Join(d.Ecosystems, ",")
	// `scripts=none` from an unreadable package.json is not the same answer as a
	// package.json that declares no scripts. Independent of whether steps were
	// found: a polyglot repo can yield a Go sequence while the Node half went
	// uninspected, and an incomplete gate presented as complete is the worse
	// failure.
	if d.ScriptsState == "unreadable" {
		g.Cause = "gate discovery could not read package.json"
	}
	for _, s := range d.Steps {
		src := Discovered
		if s.Origin == gate.Pinned {
			src = Pinned
		}
		g.Steps = append(g.Steps, GateStep{Step: s.Step, Command: s.Cmd, Source: src})
	}
	return g
}

// discoverSpec reads the store from docs/agents/issue-tracker.md where a repo has
// one — ADR 0001 decision 4: a store the user already declared is better evidence
// than a heuristic over `git remote`. The remote only supplies the ref.
func discoverSpec(repo *gitrepo.Repo, cfg *repoconfig.Config) Spec {
	var s Spec

	if cfg.Spec.Store != "" {
		s.Store = Value{Value: cfg.Spec.Store, Source: Pinned}
	} else if store := readTrackerDoc(repo.Toplevel); store != "" {
		s.Store = Value{Value: store, Source: Discovered}
	} else {
		s.Store = Value{Source: Unavailable, Cause: "no docs/agents/issue-tracker.md and nothing pinned — " +
			"a gh binary and a GitHub remote are not evidence the team tracks work there"}
	}

	switch {
	case cfg.Spec.Ref != "":
		s.Ref = Value{Value: cfg.Spec.Ref, Source: Pinned}
	case s.Store.Value == "github-issues":
		if slug := githubSlug(repo); slug != "" {
			s.Ref = Value{Value: slug, Source: Discovered}
		} else {
			s.Ref = Value{Source: Unavailable, Cause: "no GitHub remote to read the slug from"}
		}
	default:
		s.Ref = Value{Source: Unavailable, Cause: "no ref pinned, and none derivable for this store"}
	}
	return s
}

var trackerH1 = regexp.MustCompile(`(?mi)^#\s*Issue tracker:\s*(.+?)\s*$`)

func readTrackerDoc(toplevel string) string {
	b, err := os.ReadFile(filepath.Join(toplevel, "docs", "agents", "issue-tracker.md"))
	if err != nil {
		return ""
	}
	m := trackerH1.FindSubmatch(b)
	if m == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(string(m[1]))) {
	case "github", "github issues":
		return "github-issues"
	case "gitlab", "gitlab issues":
		return "gitlab"
	case "files", "repo files":
		return "files"
	case "none":
		return "none"
	}
	return ""
}

var remoteSlug = regexp.MustCompile(`github\.com[:/]+([^/]+/[^/]+?)(?:\.git)?/?$`)

func githubSlug(repo *gitrepo.Repo) string {
	remote := repo.Remote()
	if remote == "" {
		return ""
	}
	m := remoteSlug.FindStringSubmatch(repo.RemoteURL(remote))
	if m == nil {
		return ""
	}
	return m[1]
}

var scopeRe = regexp.MustCompile(`^[a-z]+\(([a-zA-Z0-9_.\-/]+)\)!?:`)

// discoverScopes reads conventional-commit scopes out of recent history. History
// is authoritative for this — it is what the repo actually does, not what someone
// once wrote down — so a pinned list replaces it only when one is pinned.
func discoverScopes(repo *gitrepo.Repo, cfg *repoconfig.Config) List {
	if len(cfg.Commit.Scopes) > 0 {
		return List{Values: cfg.Commit.Scopes, Source: Pinned}
	}
	seen := map[string]int{}
	for _, subject := range repo.Log("200") {
		if m := scopeRe.FindStringSubmatch(subject); m != nil {
			seen[m[1]]++
		}
	}
	if len(seen) == 0 {
		return List{Values: []string{}, Source: Unavailable,
			Cause: "no conventional-commit scopes in the last 200 commits"}
	}
	scopes := make([]string, 0, len(seen))
	for s := range seen {
		scopes = append(scopes, s)
	}
	// Most-used first, then alphabetical — a stable order, so two runs on one repo
	// produce the same profile and a diff of it means something.
	sort.Slice(scopes, func(i, j int) bool {
		if seen[scopes[i]] != seen[scopes[j]] {
			return seen[scopes[i]] > seen[scopes[j]]
		}
		return scopes[i] < scopes[j]
	})
	return List{Values: scopes, Source: Discovered}
}

// discoverReviewers reads CODEOWNERS where there is one. Only the owner tokens are
// taken, not the patterns: who reviews *what* is a judgement the pr skill makes
// from the diff, and flattening it here would be that judgement made badly.
func discoverReviewers(repo *gitrepo.Repo, cfg *repoconfig.Config) List {
	if len(cfg.Review.Reviewers) > 0 {
		return List{Values: cfg.Review.Reviewers, Source: Pinned}
	}
	for _, rel := range []string{
		filepath.Join(".github", "CODEOWNERS"), "CODEOWNERS", filepath.Join("docs", "CODEOWNERS"),
	} {
		b, err := os.ReadFile(filepath.Join(repo.Toplevel, rel))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		var owners []string
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.IndexByte(line, '#'); i >= 0 {
				line = line[:i]
			}
			for _, tok := range strings.Fields(line) {
				if strings.HasPrefix(tok, "@") && !seen[tok] {
					seen[tok] = true
					owners = append(owners, tok)
				}
			}
		}
		if len(owners) > 0 {
			return List{Values: owners, Source: Discovered}
		}
	}
	return List{Values: []string{}, Source: Unavailable,
		Cause: "no CODEOWNERS file and nothing pinned"}
}

// discoverMerge asks git config, which is the only local evidence. The remote's
// own merge settings are not consulted: that is a network call on a read-only
// report, and `gh` may be missing or unauthenticated — both of which would turn a
// profile into a thing that sometimes hangs.
func discoverMerge(repo *gitrepo.Repo, cfg *repoconfig.Config) Value {
	if cfg.Merge.Style != "" {
		return Value{Value: cfg.Merge.Style, Source: Pinned}
	}
	if v := repo.LocalConfigValue("mkit.mergeStyle"); v != "" {
		return Value{Value: v, Source: Discovered}
	}
	// Repo-local only, and every rebase flavour counts. `pull.rebase` takes
	// true|false|merges|interactive, and reading it through `git config --get`
	// would surface a developer's global habit as this repo's convention.
	switch repo.LocalConfigValue("pull.rebase") {
	case "true", "merges", "interactive":
		return Value{Value: "rebase", Source: Discovered}
	}
	return Value{Source: Unavailable,
		Cause: "not discoverable locally — the remote's merge settings are a network call, " +
			"so pin it with `mkit init` if this repo squash-merges"}
}

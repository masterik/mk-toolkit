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
	"strconv"
	"strings"

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

// PayloadInfo records whether the plugin payload was reachable, since gate
// discovery is delegated to it until M5 ports gate-detect.sh.
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

	p.Gate = buildGate(repo, cfg, root)
	p.Spec = discoverSpec(repo, cfg)
	p.Scopes = discoverScopes(repo, cfg)
	p.Review = discoverReviewers(repo, cfg)
	p.Merge = discoverMerge(repo, cfg)
	return p, nil
}

// buildGate merges pinned commands over the discovered sequence, by step name.
//
// Discovery is delegated to the payload's gate-detect.sh rather than reimplemented
// here. That is deliberate and temporary: the script is the single implementation
// of that invariant until M5 ports it, and a second one in Go is exactly the
// failure the porting rules name. When the payload is unreachable, the pinned half
// still answers and the discovered half reports its cause.
func buildGate(repo *gitrepo.Repo, cfg *repoconfig.Config, root *pluginroot.Root) Gate {
	var g Gate

	if root == nil {
		g.Cause = "gate discovery needs the plugin payload (gate-detect.sh); " + pluginroot.Remedy()
	} else {
		out, err := root.Script(repo.Toplevel, "gate-detect.sh")
		if err != nil && out == "" {
			g.Cause = "gate-detect.sh did not run"
		}
		var scriptsState string
		for _, line := range strings.Split(out, "\n") {
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch key {
			case "ecosystem":
				if val != "none" {
					g.Ecosystem = val
				}
			case "scripts_state":
				scriptsState = val
			case "full":
				if val == "none" || val == "" {
					continue
				}
				for _, cmd := range strings.Split(val, "|") {
					cmd = strings.TrimSpace(cmd)
					if cmd == "" {
						continue
					}
					g.Steps = append(g.Steps, GateStep{
						Step: stepName(cmd, len(g.Steps)), Command: cmd, Source: Discovered,
					})
				}
			}
		}
		// `full=none` from a degraded run is not the same answer as `full=none`
		// from a repo with no gate. gate-detect.sh exits 0 either way, so without
		// this the profile reports "no gate" when the truth is "not looked".
		if len(g.Steps) == 0 && g.Cause == "" {
			switch scriptsState {
			case "no-jq":
				g.Cause = "gate discovery could not read package.json — jq is missing, " +
					"so a Node repo's scripts were not inspected"
			case "unreadable":
				g.Cause = "gate discovery could not read package.json"
			}
		}
	}

	// Pinned wins per step name, and a pinned step discovery did not find is added.
	for _, name := range sortedKeys(cfg.Gate.Commands) {
		cmd := cfg.Gate.Commands[name]
		replaced := false
		for i := range g.Steps {
			if g.Steps[i].Step == name {
				g.Steps[i] = GateStep{Step: name, Command: cmd, Source: Pinned}
				replaced = true
				break
			}
		}
		if !replaced {
			g.Steps = append(g.Steps, GateStep{Step: name, Command: cmd, Source: Pinned})
		}
	}
	return g
}

// stepName labels a discovered command. An explicit table, not a guess: the label
// is for a human reading the profile and for pinning an override by name, and a
// command matching none of them keeps a positional name rather than a wrong one.
func stepName(cmd string, i int) string {
	for _, name := range []string{"typecheck", "build", "vet", "test", "lint", "fmt", "check"} {
		for _, tok := range strings.Fields(cmd) {
			if tok == name {
				return name
			}
		}
	}
	return "step" + strconv.Itoa(i+1)
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

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

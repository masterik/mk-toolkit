// Package repoconfig reads and writes the repo's committed mkit config.
//
// The file is `<toplevel>/.mkit/config.toml` — the one committed file inside a
// directory that is otherwise ignored scratch. That path, and why it is not a
// root-level `mkit.toml`, `.claude/` or `.agents/`, is
// docs/adr/0001-per-repo-config-and-init.md's config-path amendment.
//
// **Config is an input, never a permission** (ADR 0001 decision 3). Absent is a
// normal state: every caller here must work without a file, and nothing in this
// package treats its absence as an error.
package repoconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// Version is the schema version written into new files. It exists so a later
// reader can recognise a file it does not fully understand rather than guessing.
const Version = 1

// RelPath is the config's path relative to the work tree root.
const RelPath = ".mkit/config.toml"

// Config is the *pinned remainder*: what inspection cannot establish, and nothing
// else. Anything discovery can answer authoritatively is deliberately absent here
// — a pinned copy of a discoverable fact is a staleness surface bought for nothing.
type Config struct {
	Version int    `toml:"version"`
	Gate    Gate   `toml:"gate"`
	Spec    Spec   `toml:"spec"`
	Commit  Commit `toml:"commit"`
	Review  Review `toml:"review"`
	Merge   Merge  `toml:"merge"`
}

// Gate pins quality-gate commands discovery would otherwise guess at — which of
// three test commands is the real one, what "lint" means in this repo.
type Gate struct {
	// Commands maps a step name (build, vet, test, lint) to the command line.
	Commands map[string]string `toml:"commands,omitempty"`
}

// Spec pins where specs and task graphs live. Not discoverable with confidence:
// a gh binary and a GitHub remote do not tell you the team tracks work there.
type Spec struct {
	// Store is one of github-issues, gitlab, files, none.
	Store string `toml:"store,omitempty"`
	// Ref qualifies Store — "owner/repo" for a tracker, a path for files.
	Ref string `toml:"ref,omitempty"`
}

// Commit pins the conventional-commit scopes this repo uses.
type Commit struct {
	Scopes []string `toml:"scopes,omitempty"`
}

// Review pins default reviewers, for repos with no CODEOWNERS to read.
type Review struct {
	Reviewers []string `toml:"reviewers,omitempty"`
}

// Merge pins how this repo integrates a branch: merge, squash or rebase.
type Merge struct {
	Style string `toml:"style,omitempty"`
}

// Path returns the absolute config path for a work tree.
func Path(toplevel string) string {
	return filepath.Join(toplevel, RelPath)
}

// State describes the config file's standing in a checkout. The distinction that
// matters is Shadowed: the file can be written and then silently never travel to a
// fresh clone, which is the one property it exists for.
type State string

const (
	// StateTracked — in the index, so a fresh clone inherits it.
	StateTracked State = "tracked"
	// StateUntracked — on disk and committable, but not committed yet.
	StateUntracked State = "untracked"
	// StateShadowed — an ignore rule covers the path, so committing it is refused.
	StateShadowed State = "shadowed"
	// StateAbsent — no file, and nothing in the way of writing one.
	StateAbsent State = "absent"
)

// Status is everything a caller needs to know about the config file's standing
// without reading it.
type Status struct {
	Path  string `json:"path"`
	State State  `json:"state"`
	// IgnoreSource names the file carrying the rule when State is Shadowed. It is
	// reported rather than assumed because `.gitignore` outranks the common dir's
	// `info/exclude`: a negation written into the wrong one changes nothing.
	IgnoreSource string `json:"ignore_source,omitempty"`
	// IgnorePattern is the rule inside that file which decided the path. The
	// remedy differs by rule, so reporting the file alone is not enough.
	IgnorePattern string `json:"ignore_pattern,omitempty"`
}

// Stat reports the config file's standing. Tracked is checked before ignored,
// because a tracked file is unaffected by ignore rules.
func Stat(repo *gitrepo.Repo) Status {
	st := Status{Path: Path(repo.Toplevel)}
	switch {
	case repo.Tracked(RelPath):
		st.State = StateTracked
	default:
		if ignored, source, pattern := repo.IgnoreRule(RelPath); ignored {
			st.State = StateShadowed
			st.IgnoreSource = source
			st.IgnorePattern = pattern
			return st
		}
		if _, err := os.Stat(st.Path); err == nil {
			st.State = StateUntracked
		} else {
			st.State = StateAbsent
		}
	}
	return st
}

// ShadowedRemedy is the single producer of the sentence for a shadowed config
// path — the Go-side counterpart of lib/common.sh's mkit_config_ignored_remedy,
// and worded identically on purpose.
//
// Naming the file git actually reported is the whole point: editing any other one
// has no effect.
func ShadowedRemedy(source, pattern string) string {
	if source == "" {
		source = ".gitignore"
	}
	// The fix depends on which rule caught the file, so the rule is named. A
	// pattern that excludes the *parent directory* cannot be undone by a negation
	// at all — git never descends into an excluded directory, so the rule itself
	// has to become `.mkit/*`. Any other pattern (a stray `*.toml`, say) is lifted
	// by a negation placed after it, and telling that user to go replace a
	// `.mkit/` rule sends them looking for a line that is not there.
	if pattern == "" {
		return fmt.Sprintf("an ignore rule in %s excludes `%s`; if it is a `.mkit/` rule, "+
			"replace it with `.mkit/*` followed by `!%s` — git cannot re-include a file "+
			"whose parent directory is excluded", source, RelPath, RelPath)
	}
	if excludesParent(pattern) {
		return fmt.Sprintf("replace the `%s` rule in %s with `.mkit/*` followed by `!%s` — "+
			"git cannot re-include a file whose parent directory is excluded",
			pattern, source, RelPath)
	}
	return fmt.Sprintf("the `%s` rule in %s excludes it; add `!%s` after that line in the "+
		"same file", pattern, source, RelPath)
}

// excludesParent reports whether a pattern excludes `.mkit` itself rather than the
// file. Only these need the rule rewritten instead of negated.
func excludesParent(pattern string) bool {
	p := strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	return p == ".mkit"
}

// Load reads the config. A missing file is not an error: it returns a zero Config
// and false, because running with no config is the supported default.
func Load(toplevel string) (*Config, bool, error) {
	b, err := os.ReadFile(Path(toplevel))
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var c Config
	if err := toml.Unmarshal(b, &c); err != nil {
		return nil, true, fmt.Errorf("%s: %w", Path(toplevel), err)
	}
	return &c, true, nil
}

// IsZero reports whether the config pins nothing at all.
func (c *Config) IsZero() bool {
	return len(c.Gate.Commands) == 0 &&
		c.Spec.Store == "" && c.Spec.Ref == "" &&
		len(c.Commit.Scopes) == 0 &&
		len(c.Review.Reviewers) == 0 &&
		c.Merge.Style == ""
}

// Write renders the config to disk, creating `.mkit/` if needed.
//
// Rendered from a template rather than marshalled, because the file is committed
// and read by a human in a diff: it carries comments, and no TOML marshaller in Go
// preserves those. Nothing round-trips this file — mkit parses it and rewrites it
// whole — so a template costs nothing and buys a readable artifact.
func Write(toplevel string, c *Config) error {
	c.Version = Version
	path := Path(toplevel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(render(c)), 0o644)
}

func render(c *Config) string {
	var b strings.Builder
	b.WriteString("# mkit — per-repo configuration.\n")
	b.WriteString("#\n")
	b.WriteString("# Written by `mkit init`; committed, so a fresh clone inherits it.\n")
	b.WriteString("# This file pins only what inspection cannot establish. Anything mkit can\n")
	b.WriteString("# discover is discovered every run and deliberately absent here — see\n")
	b.WriteString("# `mkit repo profile` for the merged picture, which tags each value\n")
	b.WriteString("# `discovered` or `pinned`.\n")
	b.WriteString("#\n")
	b.WriteString("# Config is an input, never a permission: every mkit command runs with this\n")
	b.WriteString("# file absent. Deleting it loses pinned answers, never capability.\n")
	fmt.Fprintf(&b, "\nversion = %d\n", c.Version)

	if len(c.Gate.Commands) > 0 {
		b.WriteString("\n# Quality-gate commands, by step. Pin a step when discovery would guess\n")
		b.WriteString("# wrong — which of three test commands is the real one, what lint means here.\n")
		b.WriteString("[gate.commands]\n")
		for _, k := range sortedKeys(c.Gate.Commands) {
			fmt.Fprintf(&b, "%s = %s\n", quoteKey(k), quote(c.Gate.Commands[k]))
		}
	}
	if c.Spec.Store != "" || c.Spec.Ref != "" {
		b.WriteString("\n# Where specs and task graphs live. Not discoverable with confidence: a gh\n")
		b.WriteString("# binary and a GitHub remote do not tell you the team tracks work there.\n")
		b.WriteString("# store: github-issues | gitlab | files | none\n")
		b.WriteString("[spec]\n")
		if c.Spec.Store != "" {
			fmt.Fprintf(&b, "store = %s\n", quote(c.Spec.Store))
		}
		if c.Spec.Ref != "" {
			fmt.Fprintf(&b, "ref = %s\n", quote(c.Spec.Ref))
		}
	}
	if len(c.Commit.Scopes) > 0 {
		b.WriteString("\n# Conventional-commit scopes this repo uses.\n")
		b.WriteString("[commit]\n")
		fmt.Fprintf(&b, "scopes = %s\n", quoteList(c.Commit.Scopes))
	}
	if len(c.Review.Reviewers) > 0 {
		b.WriteString("\n# Default reviewers, for a repo with no CODEOWNERS to read.\n")
		b.WriteString("[review]\n")
		fmt.Fprintf(&b, "reviewers = %s\n", quoteList(c.Review.Reviewers))
	}
	if c.Merge.Style != "" {
		b.WriteString("\n# How this repo integrates a branch: merge | squash | rebase.\n")
		b.WriteString("[merge]\n")
		fmt.Fprintf(&b, "style = %s\n", quote(c.Merge.Style))
	}
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// quote emits a TOML basic string. Delegated to the marshaller rather than
// hand-rolled, so escaping is the library's problem and not a bug waiting in a
// repo whose lint command contains a quote or a backslash.
// bareKey is TOML's unquoted key grammar. A step named `build.fast` written bare
// would parse back as a *nested table*, and one with a space would not parse at
// all — either way `mkit init` writes a file it cannot read.
var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func quoteKey(k string) string {
	if bareKey.MatchString(k) {
		return k
	}
	return quote(k)
}

func quote(s string) string {
	b, err := toml.Marshal(map[string]string{"v": s})
	if err == nil {
		if _, rest, ok := strings.Cut(strings.TrimRight(string(b), "\n"), "= "); ok {
			return rest
		}
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func quoteList(ss []string) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = quote(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

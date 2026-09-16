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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// Version is the schema version written into new files. It exists so a later
// reader can recognise a file it does not fully understand rather than guessing.
//
// **A file whose version is higher than this one is read, not refused.** The
// decision, and why it is not a choice between "refuse" and "ignore": config is
// an input, never a permission (ADR 0001 decision 3), so a reader that refuses a
// newer file turns a colleague's upgrade into a broken checkout for everyone who
// has not upgraded yet. Reading it is also safe by construction — an unknown key
// is already reported as one, and a key this reader *does* know is a key whose
// meaning this schema fixed. So the newer version is reported as a Problem, the
// values that parse are used, and nothing is refused.
const Version = 1

// The enumerated fields' vocabularies. **One producer**: `mkit init` validates
// its flags and its form against these, and Load validates the file against the
// same sets. Two copies would be two things to keep true, and the file is
// committed and hand-edited — `init`'s own success message says so.
var (
	// SpecStores are the accepted `spec.store` values.
	SpecStores = []string{"github-issues", "gitlab", "files", "none"}
	// MergeStyles are the accepted `merge.style` values.
	MergeStyles = []string{"merge", "squash", "rebase"}
	// ReviewModes are the accepted `review.mode` values — the two rosters
	// `review` already runs. Named here rather than in the skill because whether
	// a team wants the full three-reviewer pass by default is a decision, not a
	// `command -v` result, and nothing in the repo is evidence for it.
	ReviewModes = []string{"full", "quick"}
)

// Allowed returns the accepted values for an enumerated key, or nil for a key
// that has no enumeration. The key→set mapping lives here with the sets, so a
// remedy elsewhere cannot name a vocabulary Load does not enforce.
func Allowed(key string) []string {
	switch key {
	case "spec.store":
		return SpecStores
	case "merge.style":
		return MergeStyles
	case "review.mode":
		return ReviewModes
	}
	return nil
}

// OneOf reports whether v is in allowed.
func OneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

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

	Cleanup Cleanup `toml:"cleanup"`

	// Problems is what Load could not honour in the file it read: keys mkit does
	// not know, values outside an enumerated set, a document that would not parse
	// at all, a version from the future. Never marshalled — it describes the read,
	// not the config — and never an error, because config is an input, never a
	// permission: a malformed file degrades to the values that did parse, and the
	// rest is reported.
	Problems []Problem `toml:"-" json:"problems,omitempty"`
}

// ProblemKind classifies what Load could not honour.
type ProblemKind string

const (
	// ProblemUnknownKey — a key or table mkit does not know. Whatever it meant to
	// pin never took effect.
	ProblemUnknownKey ProblemKind = "unknown-key"
	// ProblemInvalidValue — an enumerated field outside its allowed set.
	ProblemInvalidValue ProblemKind = "invalid-value"
	// ProblemUnparsable — the document is not TOML.
	ProblemUnparsable ProblemKind = "unparsable"
	// ProblemNewerVersion — written by a newer mkit. Reported, never refused.
	ProblemNewerVersion ProblemKind = "newer-version"
)

// Problem is one thing Load could not honour, named with the key and the file it
// is in — a reader who is told "ignored" without being told where cannot fix it.
type Problem struct {
	Kind ProblemKind `json:"kind"`
	// Key is the dotted config key, empty for a whole-document problem.
	Key string `json:"key,omitempty"`
	// Value is the offending value, or the parser's message for an unparsable
	// document.
	Value string `json:"value,omitempty"`
	// Path is the config file, always set.
	Path string `json:"path"`
	// Detail is the whole sentence, produced once here so every consumer —
	// `repo profile`, `doctor` — reports the same words.
	Detail string `json:"detail"`
}

// The four sentences, one producer each.

func unknownKeyProblem(key, path string) Problem {
	return Problem{Kind: ProblemUnknownKey, Key: key, Path: path,
		Detail: fmt.Sprintf("`%s` in %s is not a key mkit knows — it is ignored, "+
			"so whatever it meant to pin never took effect", key, path)}
}

func invalidValueProblem(key, value, path string, allowed []string) Problem {
	return Problem{Kind: ProblemInvalidValue, Key: key, Value: value, Path: path,
		Detail: fmt.Sprintf("`%s` in %s is %q, which is not one of %s — the pinned value "+
			"is ignored and discovery answers instead", key, path, value, strings.Join(allowed, ", "))}
}

// subjectMaxProblem is the one sentence for a `commit.subject_max` that is not a
// usable length.
//
// Its own producer rather than invalidValueProblem's, because that sentence names
// an allowed *set* and ends "discovery answers instead", and both halves would be
// false here: the key is enumerated by nothing, and discovered by nothing — so
// what happens next is `commit`'s own convention, not a discovered answer.
//
// Only a value that cannot be a length at all is refused. There is no upper
// bound: 500 is a silly subject limit but an operable one, and a schema that
// refused it would be pinning a house style rather than catching a mistake.
func subjectMaxProblem(n int, path string) Problem {
	return Problem{Kind: ProblemInvalidValue, Key: "commit.subject_max", Value: strconv.Itoa(n), Path: path,
		Detail: fmt.Sprintf("`commit.subject_max` in %s is %d, which is not a usable subject length "+
			"— a length is a positive number of characters, so the pinned limit is ignored and "+
			"`commit` falls back to its own convention", path, n)}
}

func unparsableProblem(path, msg string) Problem {
	return Problem{Kind: ProblemUnparsable, Value: msg, Path: path,
		Detail: fmt.Sprintf("%s is not valid TOML (%s) — nothing in it is pinned, and every "+
			"command runs on discovery alone", path, msg)}
}

func newerVersionProblem(version int, path string) Problem {
	return Problem{Kind: ProblemNewerVersion, Key: "version", Value: strconv.Itoa(version), Path: path,
		Detail: fmt.Sprintf("%s declares version %d and this mkit knows version %d — it is read, "+
			"not refused; keys this version does not know are reported as unknown", path, version, Version)}
}

// Problem returns the problem recorded for a dotted key, or nil.
func (c *Config) Problem(key string) *Problem {
	for i := range c.Problems {
		if c.Problems[i].Key == key {
			return &c.Problems[i]
		}
	}
	return nil
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

// Commit pins the rules `commit` already promises to honour: the scopes this
// repo uses, and the longest subject it accepts.
type Commit struct {
	Scopes []string `toml:"scopes,omitempty"`
	// SubjectMax is the longest commit subject this repo accepts, in characters.
	//
	// **Not discoverable, and deliberately so** — history shows what past
	// subjects happened to be, not what the repo requires, and the longest one
	// ever written is evidence of nothing. So there is no discovered counterpart:
	// absent means `commit` falls back to its own convention.
	//
	// A pointer, so absent and `subject_max = 0` are different states. They
	// decode to the same int, and zero is not "no limit" — it is a pin that would
	// silently take no effect, which is exactly the failure #19's validation
	// exists to make visible. Nil is absent; a rejected value is cleared back to
	// nil and the Problem is what says which.
	SubjectMax *int `toml:"subject_max,omitempty"`
}

// Review pins default reviewers, for repos with no CODEOWNERS to read, and the
// review roster this repo runs by default.
type Review struct {
	Reviewers []string `toml:"reviewers,omitempty"`
	// Mode is one of full, quick — the roster `review` opens with when the user
	// named none. `$ARGUMENTS` still wins: a pinned default is a default, not a
	// ceiling.
	Mode string `toml:"mode,omitempty"`
}

// Merge pins how this repo integrates a branch: merge, squash or rebase.
type Merge struct {
	Style string `toml:"style,omitempty"`
}

// Cleanup pins the branches a repo-wide cleanup must never delete, beyond the
// ones it already works out for itself.
//
// Not discoverable: branch protection is a network call on an otherwise local
// classifier, and `gh` may be missing or unauthenticated. Which local branches
// are long-lived — a `staging`, a release branch — is exactly the kind of answer
// a committed config exists to hold, and being wrong here deletes a branch.
type Cleanup struct {
	// Keep is branch **names**, not patterns. Narrow on purpose: a glob that
	// matches more than its author meant is the failure this key exists to
	// prevent, so globs wait for a repo that turns up needing them.
	//
	// **It is added to what cleanup protects, never substituted for it.** The
	// default branch is protected whether or not it appears here — a keep list
	// that omits it is a mistake, not an instruction — and the union lives in
	// `branchscan.ProtectedSet`, which is its one producer.
	//
	// No enumeration, so `Allowed("cleanup.keep")` is nil and `validate` has
	// nothing to check: any string is a legal branch name to pin, and a name that
	// is not a local branch here is not an error either — a keep list travels
	// with the repo, and `mkit branch scan` reports such a name as `keep_unknown=`
	// rather than refusing it. A blank entry pins nothing and is dropped by the
	// same producer.
	Keep []string `toml:"keep,omitempty"`
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
	// ParentExcluded reports that the rule excludes `.mkit` itself rather than
	// the file. Only then must the rule be rewritten; otherwise a negation after
	// it is enough.
	ParentExcluded bool `json:"parent_excluded,omitempty"`
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
			st.ParentExcluded = parentExcluded(repo, pattern)
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
func ShadowedRemedy(st Status) string {
	source := st.IgnoreSource
	if source == "" {
		source = ".gitignore"
	}
	// The fix depends on which rule caught the file, so the rule is named. A
	// pattern that excludes the *parent directory* cannot be undone by a negation
	// at all — git never descends into an excluded directory, so the rule itself
	// has to become `.mkit/*`. Any other pattern (a stray `*.toml`, say) is lifted
	// by a negation placed after it, and telling that reader to go replace a
	// `.mkit/` rule sends them looking for a line that is not there.
	if st.IgnorePattern == "" {
		return fmt.Sprintf("an ignore rule in %s excludes `%s`; if it is a `.mkit/` rule, "+
			"replace it with `.mkit/*` followed by `!%s` — git cannot re-include a file "+
			"whose parent directory is excluded", source, RelPath, RelPath)
	}
	if st.ParentExcluded {
		return fmt.Sprintf("replace the `%s` rule in %s with `.mkit/*` followed by `!%s` — "+
			"git cannot re-include a file whose parent directory is excluded",
			st.IgnorePattern, source, RelPath)
	}
	return fmt.Sprintf("the `%s` rule in %s excludes it; add `!%s` after that line in the "+
		"same file", st.IgnorePattern, source, RelPath)
}

// parentExcluded reports whether the rule that shadowed the config excludes the
// `.mkit` *directory* rather than the file inside it. Only then is a negation
// useless and the rule itself has to change.
//
// Two signals, because neither is complete on its own and the whole point is a
// remedy that works:
//
//   - **Ask git about the bare `.mkit`.** Exact wherever git can tell it is a
//     directory, and it catches every non-directory pattern that swallows the
//     parent — `.m*`, `.mkit*`, `/.mkit`, a bare `*`. It cannot answer for a
//     directory-only rule before the directory exists, which on a fresh clone it
//     does not.
//   - **A directory-form pattern (trailing `/`).** Such a pattern matches only
//     directories, so if it decided the fate of a *file* path it must have matched
//     a directory component of it — and `.mkit` is the only one. This is the case
//     the probe misses: `.mkit/`, `**/.mkit/`, `.mki?/`.
//
// Measured across all eleven rule shapes in TestParentExcludedMatchesGit, which
// checks the prediction against whether a negation actually re-includes the file.
func parentExcluded(repo *gitrepo.Repo, pattern string) bool {
	if strings.HasSuffix(pattern, "/") {
		return true
	}
	ignored, _, _ := repo.IgnoreRule(".mkit")
	return ignored
}

// Load reads the config. A missing file is not an error: it returns a zero Config
// and false, because running with no config is the supported default.
//
// **Nothing in the file's content is an error either.** The decode is strict, so
// an unknown key is seen rather than dropped, and the enumerated fields are
// checked on read with the same sets `mkit init` validates against — but every
// finding lands in Config.Problems and the values that did parse are kept. The
// only error this returns is a file it could not read at all.
//
// Strict decode collects *every* offending key: go-toml accumulates them into one
// StrictMissingError at the end of the decode, so the known fields are populated
// and the unknown ones are all named. Failing on the first would make fixing a
// hand-edited file a game of whack-a-mole.
//
// An invalid enumerated value is **cleared** as well as reported. Leaving it in
// place would hand a skill a `merge.style` of "sqaush" to branch on; clearing it
// makes the pinned answer absent, which is a state every caller already handles,
// and the Problem is what turns that absence into "ignored" rather than "never
// pinned" — see profile.discoverMerge.
func Load(toplevel string) (*Config, bool, error) {
	path := Path(toplevel)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	c := &Config{}
	dec := toml.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		var missing *toml.StrictMissingError
		if !errors.As(err, &missing) {
			// Not TOML, or a value whose type the schema cannot hold. Nothing
			// decoded is trustworthy, so the config is the zero one and the
			// document is reported whole.
			return &Config{Problems: []Problem{unparsableProblem(path, err.Error())}}, true, nil
		}
		for i := range missing.Errors {
			c.Problems = append(c.Problems, unknownKeyProblem(strings.Join(missing.Errors[i].Key(), "."), path))
		}
	}
	c.validate(path)
	return c, true, nil
}

// validate checks what the type system cannot: the enumerated fields, and a
// version from the future.
func (c *Config) validate(path string) {
	if c.Spec.Store != "" && !OneOf(c.Spec.Store, SpecStores) {
		c.Problems = append(c.Problems, invalidValueProblem("spec.store", c.Spec.Store, path, SpecStores))
		c.Spec.Store = ""
	}
	if c.Merge.Style != "" && !OneOf(c.Merge.Style, MergeStyles) {
		c.Problems = append(c.Problems, invalidValueProblem("merge.style", c.Merge.Style, path, MergeStyles))
		c.Merge.Style = ""
	}
	if c.Review.Mode != "" && !OneOf(c.Review.Mode, ReviewModes) {
		c.Problems = append(c.Problems, invalidValueProblem("review.mode", c.Review.Mode, path, ReviewModes))
		c.Review.Mode = ""
	}
	// Cleared to zero, which is the same state as absent — and that is what the
	// recorded Problem is for: `Problem("commit.subject_max")` is how a consumer
	// tells "ignored" from "never pinned", exactly as it does for merge.style.
	if c.Commit.SubjectMax != nil && *c.Commit.SubjectMax <= 0 {
		c.Problems = append(c.Problems, subjectMaxProblem(*c.Commit.SubjectMax, path))
		c.Commit.SubjectMax = nil
	}
	if c.Version > Version {
		c.Problems = append(c.Problems, newerVersionProblem(c.Version, path))
	}
}

// IsZero reports whether the config pins nothing at all.
func (c *Config) IsZero() bool {
	return len(c.Gate.Commands) == 0 &&
		c.Spec.Store == "" && c.Spec.Ref == "" &&
		len(c.Commit.Scopes) == 0 && c.Commit.SubjectMax == nil &&
		len(c.Review.Reviewers) == 0 && c.Review.Mode == "" &&
		c.Merge.Style == "" &&
		len(c.Cleanup.Keep) == 0
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
	if len(c.Commit.Scopes) > 0 || c.Commit.SubjectMax != nil {
		b.WriteString("\n# The rules `commit` honours: the conventional-commit scopes this repo\n")
		b.WriteString("# uses, and the longest subject it accepts. subject_max is not discoverable —\n")
		b.WriteString("# history shows what past subjects happened to be, not what is required.\n")
		b.WriteString("[commit]\n")
		if len(c.Commit.Scopes) > 0 {
			fmt.Fprintf(&b, "scopes = %s\n", quoteList(c.Commit.Scopes))
		}
		if c.Commit.SubjectMax != nil {
			fmt.Fprintf(&b, "subject_max = %d\n", *c.Commit.SubjectMax)
		}
	}
	if len(c.Review.Reviewers) > 0 || c.Review.Mode != "" {
		b.WriteString("\n# Default reviewers, for a repo with no CODEOWNERS to read, and the roster\n")
		b.WriteString("# `review` opens with when the user named none: mode = full | quick.\n")
		b.WriteString("[review]\n")
		if len(c.Review.Reviewers) > 0 {
			fmt.Fprintf(&b, "reviewers = %s\n", quoteList(c.Review.Reviewers))
		}
		if c.Review.Mode != "" {
			fmt.Fprintf(&b, "mode = %s\n", quote(c.Review.Mode))
		}
	}
	if c.Merge.Style != "" {
		b.WriteString("\n# How this repo integrates a branch: merge | squash | rebase.\n")
		b.WriteString("[merge]\n")
		fmt.Fprintf(&b, "style = %s\n", quote(c.Merge.Style))
	}
	if len(c.Cleanup.Keep) > 0 {
		b.WriteString("\n# Branches `cleanup` must never delete — names, not patterns. Added to what\n")
		b.WriteString("# it already protects: the default branch is kept whether or not it is listed.\n")
		b.WriteString("[cleanup]\n")
		fmt.Fprintf(&b, "keep = %s\n", quoteList(c.Cleanup.Keep))
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

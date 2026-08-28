package storage

import (
	"os"
	"path/filepath"
)

// Kind distinguishes the two ways a category is pruned.
type Kind int

const (
	// KindFiles prunes regular files directly under a category's base,
	// recursively, older than the retention cutoff.
	KindFiles Kind = iota
	// KindStaleDirs prunes immediate subdirectories of a category's base
	// that contain no file newer than the retention cutoff.
	KindStaleDirs
)

// String renders Kind the way --json exposes it.
func (k Kind) String() string {
	switch k {
	case KindFiles:
		return "files"
	case KindStaleDirs:
		return "stale_dirs"
	default:
		return "unknown"
	}
}

// MarshalJSON renders Kind as its string name rather than its int value.
func (k Kind) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.String() + `"`), nil
}

// Category is one row of a provider's prune table: a label for output, a
// base directory relative to the provider home, a kind, and — for
// KindFiles — an optional glob restricting which files count.
type Category struct {
	Label   string
	RelBase string
	Kind    Kind
	Pattern string
}

// Provider is one home directory (claude, codex) and its ordered category
// list. EmptyDirSweep names the directory, relative to home, that Apply
// sweeps for now-empty subdirectories after deleting — never the directory
// itself.
type Provider struct {
	Name           string
	DisplayName    string
	HomeEnv        string
	HomeDefaultRel string
	Categories     []Category
	EmptyDirSweep  string
}

// ResolveHome returns the provider's home directory: $<HomeEnv> if set and
// non-empty, else $HOME/<HomeDefaultRel>.
func (p Provider) ResolveHome() string {
	if v := os.Getenv(p.HomeEnv); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, p.HomeDefaultRel)
}

// Providers is the fixed provider table, in output order. Adding a provider
// (e.g. opencode) is a data edit here, not new code.
var Providers = []Provider{
	{
		Name:           "claude",
		DisplayName:    "Claude",
		HomeEnv:        "CLAUDE_HOME",
		HomeDefaultRel: ".claude",
		Categories: []Category{
			{Label: "projects/**/*.jsonl (transcripts)", RelBase: "projects", Kind: KindFiles, Pattern: "*.jsonl"},
			{Label: "file-history/<session>/", RelBase: "file-history", Kind: KindStaleDirs},
			{Label: "session-env/<session>/", RelBase: "session-env", Kind: KindStaleDirs},
			{Label: "paste-cache/*", RelBase: "paste-cache", Kind: KindFiles},
			{Label: "shell-snapshots/*", RelBase: "shell-snapshots", Kind: KindFiles},
			{Label: "telemetry/*", RelBase: "telemetry", Kind: KindFiles},
		},
		EmptyDirSweep: "projects",
	},
	{
		Name:           "codex",
		DisplayName:    "Codex",
		HomeEnv:        "CODEX_HOME",
		HomeDefaultRel: ".codex",
		Categories: []Category{
			{Label: "sessions/**/*.jsonl (transcripts)", RelBase: "sessions", Kind: KindFiles, Pattern: "*.jsonl"},
			{Label: "shell_snapshots/*", RelBase: "shell_snapshots", Kind: KindFiles},
		},
		EmptyDirSweep: "sessions",
	},
}

// ByName returns the provider with the given name, and whether it was found.
func ByName(name string) (Provider, bool) {
	for _, p := range Providers {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}

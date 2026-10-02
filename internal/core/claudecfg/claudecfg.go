// Package claudecfg reads the Claude Code settings the sandbox audit reasons
// about: what is configured, so a finding can be matched against it.
//
// Reports, never writes. A settings file that is absent is no entry; one that
// cannot be read or parsed is named in Result.Unreadable, never turned into an
// empty config — "nothing allowlisted" and "could not look" lead to opposite
// advice.
//
// Layering: returns data, never prints, never assumes a terminal.
package claudecfg

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// Scope says which of Claude Code's settings layers a file is.
const (
	ScopeUser    = "user"
	ScopeProject = "project"
	ScopeLocal   = "local"
)

// File is one settings file and the keys extracted from it. Every value belongs
// to Path, which is how a reader tells where an entry came from.
type File struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`

	AllowedDomains   []string `json:"allowed_domains,omitempty"`
	AllowWrite       []string `json:"allow_write,omitempty"`
	ExcludedCommands []string `json:"excluded_commands,omitempty"`
	Allow            []string `json:"allow,omitempty"`
	Ask              []string `json:"ask,omitempty"`
	Deny             []string `json:"deny,omitempty"`
	AdditionalDirs   []string `json:"additional_directories,omitempty"`
	AutoModeAllow    []string `json:"automode_allow,omitempty"`
	AutoModeSoftDeny []string `json:"automode_soft_deny,omitempty"`
	// Env is the variable names only: values are routinely secrets, and the
	// audit reasons about which variables are set, never what they hold.
	Env []string `json:"env,omitempty"`

	// UnknownKeys are dotted key paths under the sections read here that are
	// not in mkit's table. A misspelled key is silently ignored by Claude Code,
	// so naming it is the point; the table can lag the docs, which is why the
	// skill checks them.
	UnknownKeys []string `json:"unknown_keys,omitempty"`
}

// Project is one project root and the settings files found for it.
type Project struct {
	Root  string `json:"root"`
	Files []File `json:"files"`
}

// Result is what the settings say.
type Result struct {
	// Home is the Claude Code home the user settings were read from.
	Home string `json:"home"`
	// UserHome expands "~" in entries and targets; the user's home directory.
	UserHome string `json:"-"`
	// ClaudeMD is whether <Home>/CLAUDE.md exists.
	ClaudeMD   bool      `json:"claude_md"`
	User       *File     `json:"user,omitempty"`
	Projects   []Project `json:"projects"`
	Unreadable []string  `json:"unreadable,omitempty"`
}

// Roots resolves each working directory to its project root, deduplicated and
// sorted. A directory inside a work tree resolves to that tree's own top level
// (a linked worktree to itself, not the main checkout, since each carries its
// own .claude/); one outside any keeps the path as given; one that no longer
// exists — a removed worktree — is skipped.
func Roots(cwds []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cwds {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err != nil {
			continue
		}
		root := c
		if r, err := gitrepo.Open(c); err == nil {
			root = r.Toplevel
		}
		if !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	sort.Strings(out)
	return out
}

// Load reads the user settings under home and, for each root, the project and
// local settings files. userHome is the user's home directory ("" = os.UserHomeDir).
func Load(home, userHome string, roots []string) *Result {
	if userHome == "" {
		userHome, _ = os.UserHomeDir()
	}
	res := &Result{Home: home, UserHome: userHome, Projects: []Project{}}
	if _, err := os.Stat(filepath.Join(home, "CLAUDE.md")); err == nil {
		res.ClaudeMD = true
	}
	if f := res.read(filepath.Join(home, "settings.json"), ScopeUser); f != nil {
		res.User = f
	}
	for _, root := range roots {
		p := Project{Root: root, Files: []File{}}
		for _, e := range []struct{ name, scope string }{
			{"settings.json", ScopeProject}, {"settings.local.json", ScopeLocal},
		} {
			if f := res.read(filepath.Join(root, ".claude", e.name), e.scope); f != nil {
				p.Files = append(p.Files, *f)
			}
		}
		res.Projects = append(res.Projects, p)
	}
	return res
}

func (r *Result) read(path, scope string) *File {
	data, err := os.ReadFile(path)
	if err != nil {
		// Absent is no entry. Anything else — a sandbox denial included — is
		// "could not look", which must not read as "nothing configured".
		if !errors.Is(err, fs.ErrNotExist) {
			r.Unreadable = append(r.Unreadable, path)
		}
		return nil
	}
	f, err := parse(data)
	if err != nil {
		r.Unreadable = append(r.Unreadable, path)
		return nil
	}
	f.Path, f.Scope = path, scope
	return f
}

func parse(data []byte) (*File, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}
	// `null` decodes into a nil map without an error; it is not a settings file.
	if top == nil {
		return nil, errors.New("settings file is not a JSON object")
	}
	f := &File{}
	sections := map[string]map[string]json.RawMessage{}
	for _, name := range []string{"sandbox", "permissions", "autoMode"} {
		raw, ok := top[name]
		if !ok {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil || isNull(raw) {
			f.UnknownKeys = append(f.UnknownKeys, name+" (not an object)")
			continue
		}
		sections[name] = m
	}
	if raw, ok := top["env"]; ok {
		var env map[string]json.RawMessage
		if err := json.Unmarshal(raw, &env); err != nil || isNull(raw) {
			f.UnknownKeys = append(f.UnknownKeys, "env (not an object)")
		} else {
			f.Env = sortedKeys(env)
		}
	}

	list := func(dst *[]string, key string, raw json.RawMessage) {
		if err := json.Unmarshal(raw, dst); err != nil || isNull(raw) {
			f.UnknownKeys = append(f.UnknownKeys, key+" (not a list of strings)")
		}
	}
	for _, k := range sortedKeys(sections["sandbox"]) {
		raw := sections["sandbox"][k]
		switch k {
		case "excludedCommands":
			list(&f.ExcludedCommands, "sandbox."+k, raw)
		case "network", "filesystem":
			var sub map[string]json.RawMessage
			if json.Unmarshal(raw, &sub) != nil || isNull(raw) {
				f.UnknownKeys = append(f.UnknownKeys, "sandbox."+k+" (not an object)")
				continue
			}
			for _, sk := range sortedKeys(sub) {
				full := "sandbox." + k + "." + sk
				switch {
				case !knownSandbox[k][sk]:
					f.UnknownKeys = append(f.UnknownKeys, full)
				case k == "network" && sk == "allowedDomains":
					list(&f.AllowedDomains, full, sub[sk])
				case k == "filesystem" && sk == "allowWrite":
					list(&f.AllowWrite, full, sub[sk])
				}
			}
		default:
			if !knownSandbox[""][k] {
				f.UnknownKeys = append(f.UnknownKeys, "sandbox."+k)
			}
		}
	}
	for _, k := range sortedKeys(sections["permissions"]) {
		raw := sections["permissions"][k]
		switch k {
		case "allow":
			list(&f.Allow, "permissions."+k, raw)
		case "ask":
			list(&f.Ask, "permissions."+k, raw)
		case "deny":
			list(&f.Deny, "permissions."+k, raw)
		case "additionalDirectories":
			list(&f.AdditionalDirs, "permissions."+k, raw)
		default:
			if !knownPermissions[k] {
				f.UnknownKeys = append(f.UnknownKeys, "permissions."+k)
			}
		}
	}
	for _, k := range sortedKeys(sections["autoMode"]) {
		raw := sections["autoMode"][k]
		switch k {
		case "allow":
			list(&f.AutoModeAllow, "autoMode."+k, raw)
		case "soft_deny":
			list(&f.AutoModeSoftDeny, "autoMode."+k, raw)
		default:
			if !knownAutoMode[k] {
				f.UnknownKeys = append(f.UnknownKeys, "autoMode."+k)
			}
		}
	}
	return f, nil
}

func isNull(raw json.RawMessage) bool { return strings.TrimSpace(string(raw)) == "null" }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Files is every settings file read, user first.
func (r *Result) Files() []File {
	var out []File
	if r.User != nil {
		out = append(out, *r.User)
	}
	for _, p := range r.Projects {
		out = append(out, p.Files...)
	}
	return out
}

func (r *Result) expand(p string) string {
	switch {
	case p == "~":
		return r.UserHome
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(r.UserHome, p[2:])
	case strings.HasPrefix(p, "//"):
		return filepath.Clean(p[1:])
	}
	return p
}

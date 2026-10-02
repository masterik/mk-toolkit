package claudecfg

import (
	"path/filepath"
	"strings"
)

// Cover names the settings entry a finding's target already matches.
type Cover struct {
	File  string `json:"file"`
	Entry string `json:"entry"`
}

// Verdict is what the config says about one sandbox-block target.
type Verdict struct {
	// Covered is set when an entry already matches — by mkit's reading of the
	// pattern, which the docs only partly define. Never set with Protected.
	Covered *Cover `json:"covered_by,omitempty"`
	// Protected is a path in the region Claude Code denies writes to whatever
	// the allowlist says: an entry there is inert, so it is not a remedy.
	Protected bool `json:"protected,omitempty"`
}

// Judge matches one target — `network-outbound host:port`, or a denied
// operation and path — against the config. roots are the project roots the
// finding occurred in: the user's settings always apply, a project's own only
// to its own roots.
func (r *Result) Judge(target string, roots []string) Verdict {
	op, subject, _ := strings.Cut(target, " ")
	if op == "network-outbound" {
		return r.judgeHost(subject, roots)
	}
	// Only a denied write is judged: allowWrite and additionalDirectories never
	// govern a read, so a blocked read under one is not "covered". A target with
	// no operation (a normalized EPERM line) names no path either.
	if !strings.HasPrefix(op, "file-write") || !isPath(subject) {
		return Verdict{}
	}
	p := filepath.Clean(r.expand(subject))
	if r.protected(p) {
		return Verdict{Protected: true}
	}
	for _, f := range r.applicable(roots) {
		base := ""
		if f.Scope != ScopeUser {
			base = projectOf(f.Path)
		}
		// additionalDirectories grants access inside a directory, not the
		// directory's own creation, so only a descendant counts for it;
		// allowWrite names the path itself too.
		for _, l := range []struct {
			entries []string
			proper  bool
		}{{f.AllowWrite, false}, {f.AdditionalDirs, true}} {
			for _, e := range l.entries {
				if r.contains(e, base, p, l.proper) {
					return Verdict{Covered: &Cover{File: f.Path, Entry: e}}
				}
			}
		}
	}
	return Verdict{}
}

func (r *Result) judgeHost(subject string, roots []string) Verdict {
	host := strings.ToLower(subject)
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	for _, f := range r.applicable(roots) {
		for _, e := range f.AllowedDomains {
			if hostMatches(strings.ToLower(e), host) {
				return Verdict{Covered: &Cover{File: f.Path, Entry: e}}
			}
		}
	}
	return Verdict{}
}

// applicable is the files that can govern a finding: user settings, then the
// project and local files of the roots it occurred in.
func (r *Result) applicable(roots []string) []File {
	var out []File
	if r.User != nil {
		out = append(out, *r.User)
	}
	for _, p := range r.Projects {
		for _, root := range roots {
			if p.Root == root {
				out = append(out, p.Files...)
				break
			}
		}
	}
	return out
}

// hostMatches: an exact entry, or `*.example.com` for any subdomain of it.
func hostMatches(entry, host string) bool {
	if entry == host {
		return true
	}
	if suffix, ok := strings.CutPrefix(entry, "*."); ok {
		return strings.HasSuffix(host, "."+suffix)
	}
	return false
}

// contains is whether the allowed directory entry holds p. A relative entry is
// read against the project root of the file it came from, and has no meaning in
// the user file, where it matches nothing.
func (r *Result) contains(entry, base, p string, proper bool) bool {
	e := r.expand(entry)
	if !filepath.IsAbs(e) {
		if base == "" {
			return false
		}
		e = filepath.Join(base, e)
	}
	e = filepath.Clean(e)
	if p == e {
		return !proper
	}
	return strings.HasPrefix(p, e+string(filepath.Separator))
}

func isPath(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~")
}

// projectOf is the project root a `<root>/.claude/<file>` path belongs to.
func projectOf(settingsPath string) string {
	return filepath.Dir(filepath.Dir(settingsPath))
}

// protected: Claude Code's own settings, hooks, skills and plugin store are
// writable by nothing the sandbox grants, so an allowWrite entry reaching them
// does nothing.
func (r *Result) protected(p string) bool {
	if r.Home != "" {
		h := filepath.Clean(r.Home)
		// plugins/data is the one carve-out: Claude Code lets a plugin keep state
		// there, and an allowWrite entry for it works.
		if inside(p, filepath.Join(h, "plugins", "data")) {
			return false
		}
		for _, sub := range protectedHomeEntries {
			if inside(p, filepath.Join(h, sub)) {
				return true
			}
		}
	}
	for _, name := range []string{"settings.json", "settings.local.json"} {
		if strings.HasSuffix(p, "/.claude/"+name) {
			return true
		}
	}
	for _, dir := range []string{"hooks", "skills", "commands", "agents"} {
		if strings.Contains(p+"/", "/.claude/"+dir+"/") {
			return true
		}
	}
	return false
}

// protectedHomeEntries are the parts of the Claude home the sandbox denies
// writes to whatever the allowlist says. The list is the set Claude Code
// reports today and can lag it, like the key table; a path missing here shows
// as an ordinary allowWrite candidate, and the skill's docs check is the
// backstop.
var protectedHomeEntries = []string{
	"settings.json", "CLAUDE.md", "plugins", "hooks", "skills", "commands", "agents",
	"workflows", "routines", "rules", "output-styles", "projects", "todos", "statsig",
	"logs", "shell-snapshots", "session-env", "backups", "shares", "jobs", "daemon",
	"local", "access-audit", "mcp-skill-archives", "mcp-discovery-cache", "cowork_plugins",
	"scheduled_tasks.json", "launch.json", "loop.md",
}

func inside(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

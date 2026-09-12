// Package pluginroot locates the plugin payload.
//
// mkit ships over two deliberately independent channels (ADR 0003) — the binary
// via Homebrew, the payload via the GitHub marketplace — so the binary has to find
// the payload rather than assume it. Since M5 the payload is skills and references
// only: nothing in it is executed, and `mkit facts` reports where it is so a skill
// can link into `_shared/references/…`.
//
// The payload is not on any stable path (a cask has no opt/ symlink and Caskroom is
// version-pinned), so finding it is a search with named fallbacks and an honest
// failure. Not finding it is a reportable state, never a crash.
package pluginroot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrNotFound means no payload checkout could be located. Callers report it as a
// degraded surface with a remedy, never as a fatal error.
var ErrNotFound = errors.New("plugin payload not found")

// PluginName is the manifest name the payload identifies itself by.
const PluginName = "mkit"

// Root is a located payload checkout.
type Root struct {
	// Dir is the payload root — the directory holding scripts/ and skills/.
	Dir string
	// Via names how it was found, so a report can say why it looked there.
	Via string
}

// Find locates the payload, preferring the most authoritative answer.
//
// Order, and the reason for each:
//
//  1. CLAUDE_PLUGIN_ROOT — set by the harness when a skill invokes us. When it is
//     present it is definitionally the payload this session is running.
//  2. MKIT_PLUGIN_ROOT — the explicit override, and what the tests use.
//  3. A `plugin/` directory beside the work tree root, *and* carrying our manifest
//     name — the development case. Ahead of the installed copy on purpose: in a
//     payload checkout the tree being edited is the one a report is about, and an
//     installed 0.14.0 answering for a 0.16.0 work tree is a wrong answer that
//     looks right. In any other repo the manifest test fails and this falls
//     through.
//  4. The marketplace checkout under the Claude config dir — where a normal
//     install lives, and the answer everywhere that is not a payload checkout.
//
// Every *searched* candidate is identified by its manifest name, never by its path.
// The two environment variables are explicit overrides and are trusted as given.
func Find(toplevel string) (*Root, error) {
	if d := os.Getenv("CLAUDE_PLUGIN_ROOT"); isPayload(d) {
		return &Root{Dir: d, Via: "CLAUDE_PLUGIN_ROOT"}, nil
	}
	if d := os.Getenv("MKIT_PLUGIN_ROOT"); isPayload(d) {
		return &Root{Dir: d, Via: "MKIT_PLUGIN_ROOT"}, nil
	}
	if toplevel != "" {
		if d := filepath.Join(toplevel, "plugin"); isPayload(d) && manifestField(d, "name") == PluginName {
			return &Root{Dir: d, Via: "development checkout"}, nil
		}
	}
	for _, base := range marketplaceBases() {
		// A marketplace checkout is the whole repo, with the payload in a
		// subdirectory named by marketplace.json's `source` — `plugin/` here, but
		// not necessarily anywhere else, so both depths are searched.
		var candidates []string
		for _, pattern := range []string{
			filepath.Join(base, "*"),
			filepath.Join(base, "*", "*"),
		} {
			m, _ := filepath.Glob(pattern)
			candidates = append(candidates, m...)
		}
		for _, d := range candidates {
			// Identified by the manifest's own name, never by the path. The
			// checkout is named after the *marketplace owner* (`masterik/plugin`
			// on this machine), so a path substring test for the repo name
			// silently matches nothing.
			if isPayload(d) && manifestField(d, "name") == PluginName {
				return &Root{Dir: d, Via: "marketplace checkout"}, nil
			}
		}
	}
	return nil, ErrNotFound
}

// Remedy is the single producer of the sentence for a payload that cannot be found.
// It names a step that works, per the project's rule that a degradation sentence
// names a remedy or says the command is human-run.
func Remedy() string {
	return "install the payload: `/plugin marketplace add masterik/mk-toolkit`, " +
		"then enable the mkit plugin — the binary and the payload ship over separate " +
		"channels (ADR 0003), so Homebrew does not install it"
}

// Version reads the payload manifest's version. Empty when unreadable — the two
// channels version independently by design (ADR 0003), so a missing answer is a
// fact to report, not a failure.
func (r *Root) Version() string { return manifestField(r.Dir, "version") }

// HasHooks reports whether the manifest declares a `hooks` key.
//
// The payload ships no hooks: hooks.json and its one SessionStart script were
// removed in 0.15.0, and the manifest must not carry the key either. A manifest
// that grew one is a regression worth naming, which is why this is asked at all.
func (r *Root) HasHooks() bool {
	b, err := os.ReadFile(manifestPath(r.Dir))
	if err != nil {
		return false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	_, ok := m["hooks"]
	return ok
}

func manifestPath(dir string) string {
	return filepath.Join(dir, ".claude-plugin", "plugin.json")
}

// manifestField reads one top-level string field. Decoded into a map rather than
// a struct: the manifest belongs to the harness's schema, not ours, and this reads
// what it needs without claiming to understand the rest.
func manifestField(dir, field string) string {
	b, err := os.ReadFile(manifestPath(dir))
	if err != nil {
		return ""
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	var v string
	if json.Unmarshal(m[field], &v) != nil {
		return ""
	}
	return v
}

func marketplaceBases() []string {
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		cfg = filepath.Join(home, ".claude")
	}
	return []string{
		filepath.Join(cfg, "plugins", "marketplaces"),
		filepath.Join(cfg, "plugins", "repos"),
	}
}

// isPayload is the shape test, not a name test: a directory is the payload when it
// carries the manifest and the scripts the binary calls into.
func isPayload(dir string) bool {
	if dir == "" {
		return false
	}
	for _, marker := range []string{
		filepath.Join(".claude-plugin", "plugin.json"),
		filepath.Join("scripts", "lib", "common.sh"),
	} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return false
		}
	}
	return true
}

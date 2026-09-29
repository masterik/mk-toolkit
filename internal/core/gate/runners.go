package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// GateSteps are the step names a task runner's recipe can stand in for, in the
// order a chain runs them.
var GateSteps = []string{"lint", "typecheck", "vet", "test", "build"}

// runner is a task runner the repo configures: the command that runs one of its
// recipes, and the recipes that run with no arguments.
type runner struct {
	eco     string
	prefix  string
	recipes []string
}

func (r runner) has(name string) bool { return slices.Contains(r.recipes, name) }

// link is one command of the chain, tagged with the gate step it performs where
// it is proposed — the place that knows. The tag is for matching it to a recipe;
// StepName is still the label a pin is keyed by. "" is a step no recipe stands
// in for.
type link struct {
	step string
	cmd  string
}

func cmds(chain []link) []string {
	out := make([]string, len(chain))
	for i, l := range chain {
		out[i] = l.cmd
	}
	return out
}

// runners reads the repo's own task runners, most specific first: a justfile or
// Makefile at the top is the entry point the repo chose for humans, so its
// `test` is the repo's answer to "how do I test" in a way `go test ./...` is
// not — it carries the flags, the env and the tools the maintainers settled on.
func runners(root string) []runner {
	var out []runner
	for _, f := range []string{"justfile", "Justfile", ".justfile"} {
		if exists(root, f) {
			out = append(out, runner{"just", "just", justRecipes(filepath.Join(root, f))})
			break
		}
	}
	for _, f := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if exists(root, f) {
			out = append(out, runner{"make", "make", makeTargets(filepath.Join(root, f))})
			break
		}
	}
	for _, f := range []string{"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml"} {
		if exists(root, f) {
			out = append(out, runner{"task", "task", taskfileTasks(filepath.Join(root, f))})
			break
		}
	}
	for _, f := range []string{"deno.json", "deno.jsonc"} {
		if exists(root, f) {
			if ts := denoTasks(filepath.Join(root, f)); len(ts) > 0 {
				out = append(out, runner{"", "deno task", ts})
			}
			break
		}
	}
	return out
}

var (
	// A just recipe header: a name at column 0, parameters, a colon — and not an
	// assignment (`x := y`, `alias b := build`, `set shell := […]`).
	justHeader = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)([^:]*):(.*)$`)
	// A make rule: one or more targets, then `:` or `::`, and not `:=`.
	makeHeader = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_./ -]*?)\s*::?(.*)$`)
)

func lines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

// justRecipes is every recipe that runs bare: one with a required parameter
// (`release version:`) cannot stand in for a step.
func justRecipes(path string) []string {
	var out []string
	for _, l := range lines(path) {
		m := justHeader.FindStringSubmatch(l)
		if m == nil || strings.HasPrefix(m[3], "=") {
			continue
		}
		required := false
		for _, p := range strings.Fields(m[2]) {
			if !strings.HasPrefix(p, "*") && !strings.Contains(p, "=") {
				required = true
			}
		}
		if !required {
			out = append(out, m[1])
		}
	}
	return uniqSorted(out)
}

func makeTargets(path string) []string {
	var out []string
	for _, l := range lines(path) {
		m := makeHeader.FindStringSubmatch(l)
		if m == nil || strings.HasPrefix(m[2], "=") {
			continue
		}
		out = append(out, strings.Fields(m[1])...)
	}
	return uniqSorted(out)
}

// taskfileTasks reads the keys directly under a Taskfile's top-level `tasks:`.
// Not a YAML parser: the one shape it needs is a mapping's first indent level.
func taskfileTasks(path string) []string {
	var out []string
	in, indent := false, -1
	for _, l := range lines(path) {
		t := strings.TrimRight(l, " \t\r")
		if t == "" || strings.HasPrefix(strings.TrimSpace(t), "#") {
			continue
		}
		lead := len(t) - len(strings.TrimLeft(t, " "))
		if lead == 0 {
			in, indent = t == "tasks:", -1
			continue
		}
		if !in {
			continue
		}
		if indent < 0 {
			indent = lead
		}
		if lead != indent {
			continue
		}
		name, _, ok := strings.Cut(strings.TrimSpace(t), ":")
		if ok && name != "" {
			out = append(out, strings.Trim(name, `"'`))
		}
	}
	return uniqSorted(out)
}

func denoTasks(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Tasks map[string]json.RawMessage `json:"tasks"`
	}
	// deno.jsonc allows comments and trailing commas; deno.json does too, in
	// practice, since Deno reads both the same way.
	if json.Unmarshal(stripJSONC(b), &cfg) != nil {
		return nil
	}
	var out []string
	for name := range cfg.Tasks {
		out = append(out, name)
	}
	return uniqSorted(out)
}

// stripJSONC removes `//` and `/* */` comments outside strings, and a comma
// that closes an object or array, leaving plain JSON.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case inStr:
			out = append(out, c)
			if c == '\\' && i+1 < len(b) {
				i++
				out = append(out, b[i])
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			if i < len(b) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				i++
			}
			i++
		case c == '}' || c == ']':
			j := len(out) - 1
			for j >= 0 && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

func uniqSorted(s []string) []string {
	sort.Strings(s)
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// preferRecipes lays the runners' recipes over the ecosystem chain, one step at
// a time: a step the repo's runner defines runs through the runner, a step it
// does not keeps the ecosystem's command, and a runner step the ecosystem never
// proposes (a Go repo's `lint`) joins the chain in GateSteps order.
func preferRecipes(chain []link, rs []runner) []link {
	recipe := map[string]string{}
	for _, s := range GateSteps {
		for _, r := range rs {
			if r.has(s) {
				recipe[s] = r.prefix + " " + s
				break
			}
		}
	}
	if len(recipe) == 0 {
		return chain
	}
	var out []link
	used := map[string]bool{}
	for _, l := range chain {
		r, ok := recipe[l.step]
		switch {
		case !ok:
			out = append(out, l)
		case !used[l.step]:
			// In a polyglot repo `npm run test` and `go test ./...` both name
			// `test`; the runner's recipe is the one that answers for the repo.
			out = append(out, link{l.step, r})
			used[l.step] = true
		}
	}
	for _, s := range GateSteps {
		if used[s] || recipe[s] == "" {
			continue
		}
		at := len(out)
		for i, l := range out {
			if slices.Index(GateSteps, l.step) > slices.Index(GateSteps, s) {
				at = i
				break
			}
		}
		out = slices.Insert(out, at, link{s, recipe[s]})
		used[s] = true
	}
	return out
}

package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestJustRecipesRunBareOnly(t *testing.T) {
	p := writeFile(t, t.TempDir(), "justfile", strings.Join([]string{
		"set shell := [\"bash\", \"-c\"]",
		"alias b := build",
		"version := \"1\"",
		"# a comment: with a colon",
		"ci: build vet test lint",
		"build:",
		"\tgo build ./...",
		"@lint:",
		"\tgolangci-lint run",
		"run *ARGS:",
		"\tgo run . {{ARGS}}",
		"release BUMP=\"auto\":",
		"\ttools/release.sh",
		"deploy target:",
		"\techo {{target}}",
		"[private]",
		"test:",
		"\tgo test ./...",
	}, "\n"))
	want := []string{"build", "ci", "lint", "release", "run", "test"}
	if got := justRecipes(p); !reflect.DeepEqual(got, want) {
		t.Errorf("recipes = %v, want %v", got, want)
	}
}

func TestMakeTargetsSkipAssignmentsAndSpecials(t *testing.T) {
	p := writeFile(t, t.TempDir(), "Makefile", strings.Join([]string{
		"GO := go",
		"FLAGS = -v:x",
		".PHONY: test lint",
		"test lint: deps",
		"\t$(GO) test ./...",
		"build::",
		"\t$(GO) build",
		"%.o: %.c",
	}, "\n"))
	want := []string{"build", "lint", "test"}
	if got := makeTargets(p); !reflect.DeepEqual(got, want) {
		t.Errorf("targets = %v, want %v", got, want)
	}
}

func TestTaskfileTasksAreTheFirstLevelUnderTasks(t *testing.T) {
	p := writeFile(t, t.TempDir(), "Taskfile.yml", strings.Join([]string{
		"version: '3'",
		"vars:",
		"  X: y",
		"tasks:",
		"  test:",
		"    cmds:",
		"      - go test ./...",
		"  \"lint\":",
		"    cmds: [golangci-lint run]",
		"includes:",
		"  other: ./x",
	}, "\n"))
	want := []string{"lint", "test"}
	if got := taskfileTasks(p); !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
}

func TestPreferRecipesStepByStep(t *testing.T) {
	just := runner{"just", "just", []string{"build", "lint", "test"}}
	make := runner{"make", "make", []string{"test", "vet"}}
	goChain := []link{{"vet", "go vet ./..."}, {"test", "go test ./..."}, {"build", "go build ./..."}}
	for _, tc := range []struct {
		name  string
		chain []link
		rs    []runner
		want  []string
	}{
		{"no runner keeps the chain", goChain[:2], nil,
			[]string{"go vet ./...", "go test ./..."}},
		{"recipes replace by step and add lint in order", goChain, []runner{just},
			[]string{"just lint", "go vet ./...", "just test", "just build"}},
		{"the first runner wins a step", goChain[:2], []runner{just, make},
			[]string{"just lint", "make vet", "just test", "just build"}},
		{"one recipe answers a polyglot step once",
			[]link{{"test", "npm run test"}, {"test", "go test ./..."}}, []runner{make},
			[]string{"make vet", "make test"}},
		{"a runner alone is a chain", nil, []runner{just},
			[]string{"just lint", "just test", "just build"}},
		{"a step is matched by its tag, not its command's words",
			[]link{{"lint", "ruff check ."}, {"test", "pytest -q"}, {"typecheck", "mypy ."}}, []runner{just},
			[]string{"just lint", "just test", "mypy .", "just build"}},
		{"untagged steps keep their place",
			[]link{{"", "golangci-lint run"}, {"test", "go test ./..."}}, []runner{make},
			[]string{"golangci-lint run", "make vet", "make test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cmds(preferRecipes(tc.chain, tc.rs)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %v\nwant %v", got, tc.want)
			}
		})
	}
}

func TestDenoTasksReadJSONC(t *testing.T) {
	p := writeFile(t, t.TempDir(), "deno.jsonc", `{
  // tasks the repo runs
  "tasks": {
    "test": "deno test -A", /* block */
    "lint": "deno lint // not a comment",
  },
}`)
	if got, want := denoTasks(p), []string{"lint", "test"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
}

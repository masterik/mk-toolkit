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
	for _, tc := range []struct {
		name  string
		chain []string
		rs    []runner
		want  []string
	}{
		{"no runner keeps the chain", []string{"go vet ./...", "go test ./..."}, nil,
			[]string{"go vet ./...", "go test ./..."}},
		{"recipes replace by step and add lint in order",
			[]string{"go vet ./...", "go test ./...", "go build ./..."}, []runner{just},
			[]string{"just lint", "go vet ./...", "just test", "just build"}},
		{"the first runner wins a step",
			[]string{"go vet ./...", "go test ./..."}, []runner{just, make},
			[]string{"just lint", "make vet", "just test", "just build"}},
		{"one recipe answers a polyglot step once",
			[]string{"npm run test", "go test ./..."}, []runner{make},
			[]string{"make vet", "make test"}},
		{"a runner alone is a chain", nil, []runner{just},
			[]string{"just lint", "just test", "just build"}},
		{"unnamed steps keep their place",
			[]string{"cargo clippy --all-targets -- -D warnings", "cargo test"}, []runner{make},
			[]string{"cargo clippy --all-targets -- -D warnings", "make vet", "make test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := preferRecipes(tc.chain, tc.rs); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %v\nwant %v", got, tc.want)
			}
		})
	}
}

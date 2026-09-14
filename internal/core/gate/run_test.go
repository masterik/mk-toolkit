package gate

import (
	"reflect"
	"strings"
	"testing"
)

// The normalization seam, which is where a regression makes the cache appear to
// work while caching nothing.
func TestParseSingleNormalizesForTheLedger(t *testing.T) {
	cases := []struct {
		name      string
		argv      []string
		cmd, norm string
		sameAsCmd bool
	}{
		{
			name: "whitespace-free argv joins",
			argv: []string{"bun", "run", "lint"},
			cmd:  `'bun' 'run' 'lint'`,
			norm: "bun run lint",
		},
		{
			// The seam that makes a review -> finish cache hit possible: this
			// exact argv is what both call forms reduce to.
			name: "a plain argv joins",
			argv: []string{"go", "test", "./..."},
			cmd:  `'go' 'test' './...'`,
			norm: "go test ./...",
		},
		{
			// The join is lossy here — `printf '%s' foo bar` would produce the
			// same string while executing differently — so the key falls back
			// to the unambiguous quoted form.
			name:      "an argument with a space falls back to the quoted form",
			argv:      []string{"printf", "%s", "foo bar"},
			cmd:       `'printf' '%s' 'foo bar'`,
			sameAsCmd: true,
		},
		{
			// A glob is not a literal once it is unquoted: `[%s]` would expand
			// against the working directory, so it can never share a key with
			// the argv that meant it literally.
			name:      "a glob falls back to the quoted form",
			argv:      []string{"printf", "[%s]", "x"},
			cmd:       `'printf' '[%s]' 'x'`,
			sameAsCmd: true,
		},
		{
			// `echo it's` is not even valid unquoted — the key has to be the
			// quoted form or it names a command that cannot run.
			name:      "an embedded single quote falls back to the quoted form",
			argv:      []string{"echo", "it's"},
			cmd:       `'echo' 'it'\''s'`,
			sameAsCmd: true,
		},
		{
			// A shell variable is expanded once unquoted, so a literal `$FLAG`
			// and a --chain step whose shell expands it must not share a key.
			name:      "a shell variable falls back to the quoted form",
			argv:      []string{"test", "$FLAG"},
			cmd:       `'test' '$FLAG'`,
			sameAsCmd: true,
		},
		{
			// An empty argument vanishes entirely when joined.
			name:      "an empty argument falls back to the quoted form",
			argv:      []string{"echo", ""},
			cmd:       `'echo' ''`,
			sameAsCmd: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseSingle("step", c.argv)
			if got.Cmd != c.cmd {
				t.Errorf("Cmd = %s, want %s", got.Cmd, c.cmd)
			}
			want := c.norm
			if c.sameAsCmd {
				want = c.cmd
			}
			if got.Norm != want {
				t.Errorf("Norm = %s, want %s", got.Norm, want)
			}
		})
	}
}

func TestParseChain(t *testing.T) {
	if got, want := ParseChain("lint=bun run lint"), (Step{Name: "lint", Cmd: "bun run lint", Norm: "bun run lint"}); !reflect.DeepEqual(got, want) {
		t.Errorf("= %+v, want %+v", got, want)
	}
	// No separator: both halves are the whole string, which is what makes
	// Validate report a malformed spec rather than an empty command.
	if got := ParseChain("noequals"); got.Name != got.Cmd {
		t.Errorf("= %+v, want Name == Cmd", got)
	}
	// Only the first `=` separates: a command may contain more.
	if got := ParseChain("test=FOO=1 bun test"); got.Cmd != "FOO=1 bun test" {
		t.Errorf("Cmd = %q", got.Cmd)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string][]Step{
		"nothing to run":        {},
		"malformed step spec":   {{Name: "noequals", Cmd: "noequals"}},
		"empty command":         {{Name: "lint", Cmd: ""}},
		"name may only contain": {{Name: "bad step", Cmd: "true"}},
	}
	for want, steps := range cases {
		err := Validate(steps)
		if err == nil {
			t.Errorf("%s: expected an error", want)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
	if err := Validate([]Step{{Name: "lint", Cmd: "true", Norm: "true"}}); err != nil {
		t.Errorf("a valid step was rejected: %v", err)
	}
}

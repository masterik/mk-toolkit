package gate

import "testing"

func TestStepName(t *testing.T) {
	for _, tc := range []struct{ cmd, want string }{
		{"go vet ./...", "vet"},
		{"go test ./...", "test"},
		{"go build ./...", "build"},
		{"golangci-lint run", "step1"}, // no bare token matches; positional, not wrong
		{"npm run typecheck", "typecheck"},
		{"make check", "check"},
	} {
		if got := StepName(tc.cmd, 0); got != tc.want {
			t.Errorf("StepName(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
	// Positional names must not run off the end of the digits.
	if got := StepName("mystery", 11); got != "step12" {
		t.Errorf("StepName(_, 11) = %q, want step12", got)
	}
}

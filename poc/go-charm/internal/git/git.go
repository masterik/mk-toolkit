// Package git provides helper functions for querying local git state
// via subprocess calls.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Branch returns the current git branch name.
// Returns "unknown" if the branch cannot be determined.
func Branch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// StagedCount returns the number of files in the git staging area.
func StagedCount() int {
	out, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// Oneline returns the latest commit as a single-line summary.
func Oneline() string {
	out, err := exec.Command("git", "log", "-1", "--oneline").Output()
	if err != nil {
		return "(no commits)"
	}
	return strings.TrimSpace(string(out))
}

// DiffStat returns the diffstat of the latest commit compared to its parent.
func DiffStat() string {
	out, err := exec.Command("git", "diff", "HEAD~1", "--stat", "--no-color").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// StageAll runs "git add -A" to stage all changes.
func StageAll() error {
	if err := exec.Command("git", "add", "-A").Run(); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	return nil
}

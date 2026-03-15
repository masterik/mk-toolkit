// Package claude provides subprocess integration for invoking the Claude CLI
// and streaming its output into a Bubble Tea program.
package claude

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// DefaultModel is the Claude model used for commit generation.
	DefaultModel = "haiku"
	// DefaultPrompt is the base prompt sent to Claude.
	DefaultPrompt = "/commit"
)

// LineMsg carries a single line of Claude's stdout output.
type LineMsg string

// DoneMsg signals that the Claude subprocess has exited.
type DoneMsg struct {
	ExitCode int
}

// Run starts the Claude CLI as a subprocess and streams stdout lines into
// the given tea.Program via Send. It returns a DoneMsg when the process exits.
//
// If customNote is non-empty, it is appended to the commit prompt.
func Run(p *tea.Program, customNote string) tea.Cmd {
	return func() tea.Msg {
		prompt := DefaultPrompt
		if customNote != "" {
			prompt = fmt.Sprintf("%s %s", DefaultPrompt, customNote)
		}

		cmd := exec.Command("claude",
			"-p", prompt,
			"--model="+DefaultModel,
			"--dangerously-skip-permissions",
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return DoneMsg{ExitCode: 1}
		}
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return DoneMsg{ExitCode: 1}
		}

		// Stream stdout lines into the Bubble Tea event loop.
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if p != nil {
				p.Send(LineMsg(scanner.Text()))
			}
		}

		exitCode := 0
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		return DoneMsg{ExitCode: exitCode}
	}
}

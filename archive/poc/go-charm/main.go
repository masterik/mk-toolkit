// Command fz-commit is an interactive git commit helper powered by Claude.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forgez/poc-go-charm/internal/git"
	"forgez/poc-go-charm/internal/tui"
)

func main() {
	start := time.Now()

	branch := git.Branch()
	staged := git.StagedCount()

	fmt.Fprintf(os.Stderr, "[fz] ready in %dms\n", time.Since(start).Milliseconds())

	m := tui.NewModel(branch, staged)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	m.SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(tui.Model); ok && fm.ExitCode != 0 {
		os.Exit(fm.ExitCode)
	}
}

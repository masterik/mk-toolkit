package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

func newGateRunCmd() *cobra.Command {
	var tail, grepN int
	var keepGoing, noLedger, chain bool

	cmd := &cobra.Command{
		Use:   "run <run-dir> [flags] <step> -- <command...>",
		Short: "Run quality-gate steps, log each in full, print a bounded verdict",
		Long: "Run quality-gate steps, log each in full, and print a bounded verdict.\n\n" +
			"  mkit gate run \"$RUN\" lint -- bun run lint\n" +
			"  mkit gate run \"$RUN\" --chain 'lint=bun run lint' 'test=bun run test'\n\n" +
			"Flags must come before `--`: everything after it is the command.\n\n" +
			"Exits with the failing step's own code, 0 if everything passed, 2 on bad usage.\n" +
			"Each finished step also appends one record to <toplevel>/.mkit/gate.jsonl — what\n" +
			"was proven, over which content. `mkit gate detect` reads it back. Nothing here\n" +
			"ever skips a step; the skill owns that trade-off and must label a skipped step\n" +
			"`cached`.",
		Args:                  cobra.ArbitraryArgs,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr("usage: mkit gate run <run-dir> <step> -- <command...> | mkit gate run <run-dir> --chain 'step=cmd' ...")
			}
			runDir, rest := args[0], args[1:]

			steps, err := parseSteps(rest, chain, cmd.Flags().ArgsLenAtDash())
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			// A repo is what the ledger is keyed against; without one there is
			// simply no ledger, which is a degradation the gate itself does not
			// notice.
			repo, _ := gitrepo.Open("")

			res, err := gate.Run(repo, steps, gate.RunOptions{
				RunDir: runDir, Tail: tail, Grep: grepN,
				KeepGoing: keepGoing, NoLedger: noLedger,
			})
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}

			if FromContext(cmd).JSON {
				if err := writeGateRunJSON(cmd.OutOrStdout(), res); err != nil {
					return err
				}
			} else {
				renderGateRun(cmd.OutOrStdout(), res, tail, grepN)
			}
			if res.Exit != 0 {
				// The verdict is already on stdout; Msg stays empty so nothing
				// is said twice. See ExitError.
				return &ExitError{Code: res.Exit}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&tail, "tail", 30, "lines of the log to print after a failure")
	cmd.Flags().IntVar(&grepN, "grep", 40, "maximum failure lines to surface from the log")
	cmd.Flags().BoolVar(&keepGoing, "keep-going", false, "run every step instead of stopping at the first failure")
	cmd.Flags().BoolVar(&noLedger, "no-ledger", false, "do not record what was proven")
	cmd.Flags().BoolVar(&chain, "chain", false, "read every remaining argument as a 'step=command' spec")
	return cmd
}

// parseSteps turns the two call forms into steps. dash is ArgsLenAtDash — the
// number of arguments cobra saw before `--`, or -1 when there was none.
func parseSteps(args []string, chain bool, dash int) ([]gate.Step, error) {
	if chain {
		steps := make([]gate.Step, 0, len(args))
		for _, spec := range args {
			steps = append(steps, gate.ParseChain(spec))
		}
		return steps, gate.Validate(steps)
	}
	// dash counts from the front of the *whole* argument list, and the run
	// directory was taken off it already — so exactly one step name before `--`
	// is dash == 2.
	if dash < 2 {
		return nil, fmt.Errorf("no step name before --")
	}
	if dash > 2 {
		return nil, fmt.Errorf("exactly one step name goes before --, got %d", dash-1)
	}
	name, argv := args[0], args[1:]
	if len(argv) == 0 {
		return nil, fmt.Errorf("step '%s' has an empty command: a gate cannot pass by running nothing", name)
	}
	steps := []gate.Step{gate.ParseSingle(name, argv)}
	return steps, gate.Validate(steps)
}

func renderGateRun(out io.Writer, res *gate.Result, tail, grepN int) {
	var ran []string
	for _, s := range res.Steps {
		ran = append(ran, s.Step)
		if s.Exit == 0 {
			_, _ = fmt.Fprintf(out, "%s ok %ds\n", s.Step, s.Secs)
			continue
		}
		_, _ = fmt.Fprintf(out, "%s FAIL exit=%d %ds log=%s\n", s.Step, s.Exit, s.Secs, s.Log)
		if len(s.Failures) > 0 {
			_, _ = fmt.Fprintf(out, "failures (max %d):\n%s\n", grepN, strings.Join(s.Failures, "\n"))
		}
		_, _ = fmt.Fprintf(out, "tail -%d:\n", tail)
		for _, l := range s.Tail {
			_, _ = fmt.Fprintln(out, l)
		}
		_, _ = fmt.Fprintf(out, "log_lines=%d\n", s.LogLines)
	}
	if res.Exit == 0 {
		_, _ = fmt.Fprintf(out, "gate=ok steps=%s\n", strings.Join(ran, ","))
		return
	}
	_, _ = fmt.Fprintf(out, "gate=FAILED step=%s exit=%d\n", res.FailedStep, res.Exit)
}

type gateRunJSON struct {
	Steps []gateStepJSON `json:"steps"`
	Gate  string         `json:"gate"`
	// FailedStep is empty on a pass.
	FailedStep string `json:"failed_step"`
	Exit       int    `json:"exit"`
}

type gateStepJSON struct {
	Step string `json:"step"`
	Cmd  string `json:"cmd"`
	Exit int    `json:"exit"`
	Secs int    `json:"secs"`
	Log  string `json:"log"`
	// Failures and Tail are the same bounded excerpt the human form prints, and
	// for the same reason: the log itself must never reach an agent, but a
	// verdict with no diagnosis in it is a verdict nobody can act on.
	Failures []string `json:"failures,omitempty"`
	Tail     []string `json:"tail,omitempty"`
	LogLines int      `json:"log_lines"`
}

func writeGateRunJSON(out io.Writer, res *gate.Result) error {
	j := gateRunJSON{Gate: "ok", FailedStep: res.FailedStep, Exit: res.Exit}
	if res.Exit != 0 {
		j.Gate = "FAILED"
	}
	j.Steps = make([]gateStepJSON, 0, len(res.Steps))
	for _, s := range res.Steps {
		j.Steps = append(j.Steps, gateStepJSON{
			Step: s.Step, Cmd: s.Cmd, Exit: s.Exit, Secs: s.Secs,
			Log: s.Log, Failures: s.Failures, Tail: s.Tail, LogLines: s.LogLines,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(j)
}

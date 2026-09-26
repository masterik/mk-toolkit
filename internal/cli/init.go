package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/initplan"
	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
	tui "github.com/masterik/mk-toolkit/internal/tui/repoinit"
)

// initResult is the machine-facing shape. `written` is the fact a caller acts on;
// `state` says why when nothing was written.
type initResult struct {
	Path    string `json:"path"`
	Written bool   `json:"written"`
	State   string `json:"state"`
	Detail  string `json:"detail,omitempty"`
	Remedy  string `json:"remedy,omitempty"`
}

func newInitCmd() *cobra.Command {
	var (
		gate       []string
		specStore  string
		specRef    string
		scopes     []string
		subjectMax int
		reviewers  []string
		reviewMode string
		merge      string
		keep       []string
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write this repo's committed mkit config",
		Long: "Pin what discovery cannot establish, into `.mkit/config.toml`, committed.\n\n" +
			"Config is an input, never a permission: every mkit command runs with this file\n" +
			"absent, and `mkit init` only removes repeated discovery. It writes nothing\n" +
			"outside the repo, and is a no-op on a repo that already has a config — pass\n" +
			"--force to rewrite one.\n\n" +
			"On a terminal it opens a paged form (Gate, Spec, Commit, Review, Merge, Cleanup):\n" +
			"every answer is a choice with a description, pre-selected from the pinned value,\n" +
			"then the discovered one, then a form default (merge style `merge`, review mode\n" +
			"`full`; everything else \"don't pin\"). It ends on the exact file it would write.\n" +
			"--force opens the form on the existing config.\n\n" +
			"Every field is also a flag, so a skill can drive it. Any flag, --yes, or no\n" +
			"terminal means no form and no form default: only what was given is pinned.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			st := repoconfig.Stat(repo)

			// Shadowed first: writing here produces a file that never travels to
			// a fresh clone, which is the single property the config exists for.
			// Refusing is the honest answer — a written-and-invisible config is
			// worse than no config.
			if st.State == repoconfig.StateShadowed {
				return emitInit(out, opts, initResult{
					Path: st.Path, State: "shadowed",
					Detail: "an ignore rule covers this path, so the config would never reach a fresh clone",
					Remedy: repoconfig.ShadowedRemedy(st),
				})
			}

			existing, present, err := repoconfig.Load(repo.Toplevel)
			if err != nil {
				return err
			}
			// A file mkit could not fully honour still counts as configured. It
			// decodes to something close to zero, and without this a repo whose
			// config is one typo away from correct would be silently overwritten
			// by an `init` that thought it was writing into empty space.
			if present && (!existing.IsZero() || len(existing.Problems) > 0) && !force {
				return emitInit(out, opts, initResult{
					Path: st.Path, State: "already-configured", Written: false,
					Detail: "nothing changed — pass --force to rewrite, or edit the file directly",
				})
			}

			cfg := existing
			if err := applyFlags(cfg, flagValues{
				gate: gate, specStore: specStore, specRef: specRef,
				scopes: scopes, subjectMax: subjectMax,
				// Whether the flag was *given* is the fact, not whether it is
				// non-zero: `--subject-max 0` is a value the user typed, and a
				// zero-value test cannot tell it from an omitted flag.
				subjectMaxSet: cmd.Flags().Changed("subject-max"),
				reviewers:     reviewers, reviewMode: reviewMode,
				merge: merge, keep: keep,
			}); err != nil {
				return err
			}

			// The TUI runs only when there is a terminal and the caller gave no
			// field on the command line. Flags win: a skill driving this must
			// never find an alt-screen in its pipe. Whether a flag was *given* is
			// the test, not whether cfg is zero, so `--force` opens the form on the
			// existing config rather than skipping it.
			if opts.Interactive && !opts.Yes && !fieldFlagGiven(cmd) {
				plan := initplan.Build(initplan.Input{
					Existing:   existing,
					Discovered: profile.Discover(repo),
					Candidates: initplan.Gather(repo),
				})
				answers, save, err := tui.Run(plan)
				if err != nil {
					return err
				}
				if !save {
					return emitInit(out, opts, initResult{
						Path: st.Path, State: "aborted", Written: false,
						Detail: "aborted — nothing written",
					})
				}
				if cfg, err = initplan.Apply(plan, answers); err != nil {
					return err
				}
			}

			if cfg.IsZero() {
				return emitInit(out, opts, initResult{
					Path: st.Path, State: "nothing-to-pin", Written: false,
					Detail: "no values given, so there is nothing to pin — discovery already " +
						"answers everything this repo needs",
				})
			}

			if err := repoconfig.Write(repo.Toplevel, cfg); err != nil {
				return err
			}
			return emitInit(out, opts, initResult{
				Path: st.Path, State: "written", Written: true,
				Detail: "commit it so a fresh clone inherits it: git add " + repoconfig.RelPath,
			})
		},
	}

	cmd.Flags().StringArrayVar(&gate, "gate", nil, "pin a gate step as step=command (repeatable)")
	cmd.Flags().StringVar(&specStore, "spec-store", "", "github-issues | gitlab | files | none")
	cmd.Flags().StringVar(&specRef, "spec-ref", "", "qualifies --spec-store (owner/repo, or a path)")
	cmd.Flags().StringArrayVar(&scopes, "scope", nil, "pin a conventional-commit scope (repeatable)")
	cmd.Flags().IntVar(&subjectMax, "subject-max", 0,
		"pin the longest commit subject this repo accepts, in characters")
	cmd.Flags().StringArrayVar(&reviewers, "reviewer", nil, "pin a default reviewer (repeatable)")
	cmd.Flags().StringVar(&reviewMode, "review-mode", "", "full | quick — the roster the review skill opens with")
	cmd.Flags().StringVar(&merge, "merge", "", "merge | squash | rebase")
	cmd.Flags().StringArrayVar(&keep, "keep", nil,
		"pin a branch cleanup must never delete (repeatable); the default branch is kept regardless")
	cmd.Flags().BoolVar(&force, "force", false, "rewrite an existing config")
	return cmd
}

// Enumerated fields are validated before anything is written. A pinned value is
// read back by every later run and by a skill that branches on it, so persisting
// `--merge sqaush` buys a typo a long life; failing at the flag is where it costs
// least.
//
// The allowed sets live in `repoconfig` because `repoconfig.Load` validates the
// same fields on read — the file is committed and hand-edited, and a second copy
// of the vocabulary here is a second thing to keep true.
var (
	specStores  = repoconfig.SpecStores
	mergeStyles = repoconfig.MergeStyles
	reviewModes = repoconfig.ReviewModes
)

// flagValues is every pinnable field as `init` received it. A struct rather than
// a tenth positional parameter: the list grows with the schema, and a caller that
// transposes two `[]string` arguments compiles.
type flagValues struct {
	gate       []string
	specStore  string
	specRef    string
	scopes     []string
	subjectMax int
	// subjectMaxSet records that --subject-max was given at all. SubjectMax is a
	// *int precisely so 0 is distinguishable from absent; testing subjectMax != 0
	// here would collapse that distinction at the one door the type exists for.
	subjectMaxSet bool
	reviewers     []string
	reviewMode    string
	merge         string
	keep          []string
}

func applyFlags(cfg *repoconfig.Config, f flagValues) error {
	gate, store, ref := f.gate, f.specStore, f.specRef
	scopes, reviewers, merge, keep := f.scopes, f.reviewers, f.merge, f.keep
	for _, g := range gate {
		step, command, ok := strings.Cut(g, "=")
		if !ok || step == "" || command == "" {
			return fmt.Errorf("--gate %q: expected step=command", g)
		}
		if cfg.Gate.Commands == nil {
			cfg.Gate.Commands = map[string]string{}
		}
		cfg.Gate.Commands[step] = command
	}
	if store != "" {
		if !repoconfig.OneOf(store, specStores) {
			return fmt.Errorf("--spec-store %q: expected one of %s", store, strings.Join(specStores, ", "))
		}
		cfg.Spec.Store = store
	}
	if ref != "" {
		cfg.Spec.Ref = ref
	}
	if len(scopes) > 0 {
		cfg.Commit.Scopes = scopes
	}
	// Validated here for the same reason the enumerated fields are: the value is
	// written into a committed file and read back by every later run, so a
	// nonsensical limit costs least at the flag. The rule is `repoconfig`'s —
	// a positive number of characters, no upper bound — and Load enforces the
	// same one on read, because the file is hand-edited too.
	if f.subjectMaxSet {
		if f.subjectMax <= 0 {
			return fmt.Errorf("--subject-max %d: a subject length is %s", f.subjectMax,
				repoconfig.Rule("commit.subject_max"))
		}
		n := f.subjectMax
		cfg.Commit.SubjectMax = &n
	}
	if len(reviewers) > 0 {
		cfg.Review.Reviewers = reviewers
	}
	if f.reviewMode != "" {
		if !repoconfig.OneOf(f.reviewMode, reviewModes) {
			return fmt.Errorf("--review-mode %q: expected one of %s", f.reviewMode, strings.Join(reviewModes, ", "))
		}
		cfg.Review.Mode = f.reviewMode
	}
	if merge != "" {
		if !repoconfig.OneOf(merge, mergeStyles) {
			return fmt.Errorf("--merge %q: expected one of %s", merge, strings.Join(mergeStyles, ", "))
		}
		cfg.Merge.Style = merge
	}
	// No validation, because there is nothing to validate against: any string is
	// a legal branch name to pin, and a name with no branch here is not an error
	// (`mkit branch scan` reports it as `keep_unknown=`). `repoconfig.Allowed`
	// returns nil for this key for the same reason, so flags and file agree.
	if len(keep) > 0 {
		cfg.Cleanup.Keep = keep
	}
	return nil
}

// fieldFlags are the flags that pin a field. Any one of them given means the
// caller is driving `init` from the command line, and the form stays shut.
var fieldFlags = []string{"gate", "spec-store", "spec-ref", "scope", "subject-max",
	"reviewer", "review-mode", "merge", "keep"}

func fieldFlagGiven(cmd *cobra.Command) bool {
	for _, name := range fieldFlags {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func emitInit(out io.Writer, opts Options, res initResult) error {
	if opts.JSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if res.Written {
		_, _ = fmt.Fprintf(out, "wrote %s\n", res.Path)
	} else {
		_, _ = fmt.Fprintf(out, "%s: %s\n", res.Path, res.State)
	}
	if res.Detail != "" {
		_, _ = fmt.Fprintf(out, "  %s\n", res.Detail)
	}
	if res.Remedy != "" {
		_, _ = fmt.Fprintf(out, "  remedy: %s\n", res.Remedy)
	}
	return nil
}

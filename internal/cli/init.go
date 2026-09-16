package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
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
		gate      []string
		specStore string
		specRef   string
		scopes    []string
		reviewers []string
		merge     string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write this repo's committed mkit config",
		Long: "Pin what discovery cannot establish, into `.mkit/config.toml`, committed.\n\n" +
			"Config is an input, never a permission: every mkit command runs with this file\n" +
			"absent, and `mkit init` only removes repeated discovery. It writes nothing\n" +
			"outside the repo, and is a no-op on a repo that already has a config — pass\n" +
			"--force to rewrite one.\n\n" +
			"Interactive on a terminal; every field is also a flag, so a skill can drive it.",
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

			p, err := profile.Build(repo)
			if err != nil {
				return err
			}

			cfg := existing
			if err := applyFlags(cfg, gate, specStore, specRef, scopes, reviewers, merge); err != nil {
				return err
			}

			// The TUI runs only when there is a terminal and the caller pinned
			// nothing on the command line. Flags win: a skill driving this must
			// never find an alt-screen in its pipe.
			if opts.Interactive && !opts.Yes && cfg.IsZero() {
				fields, save, err := tui.Run(initFields(p))
				if err != nil {
					return err
				}
				if !save {
					return emitInit(out, opts, initResult{
						Path: st.Path, State: "aborted", Written: false,
						Detail: "aborted — nothing written",
					})
				}
				if err := applyFields(cfg, fields); err != nil {
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
	cmd.Flags().StringArrayVar(&reviewers, "reviewer", nil, "pin a default reviewer (repeatable)")
	cmd.Flags().StringVar(&merge, "merge", "", "merge | squash | rebase")
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
)

func applyFlags(cfg *repoconfig.Config, gate []string, store, ref string, scopes, reviewers []string, merge string) error {
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
	if len(reviewers) > 0 {
		cfg.Review.Reviewers = reviewers
	}
	if merge != "" {
		if !repoconfig.OneOf(merge, mergeStyles) {
			return fmt.Errorf("--merge %q: expected one of %s", merge, strings.Join(mergeStyles, ", "))
		}
		cfg.Merge.Style = merge
	}
	return nil
}

// initFields shows the discovered answer beside every field, so the form is a
// choice to override rather than a blank to fill.
func initFields(p *profile.Profile) []tui.Field {
	var gate []string
	for _, s := range p.Gate.Steps {
		gate = append(gate, s.Step+"="+s.Command)
	}
	return []tui.Field{
		{Key: "gate", Label: "gate", Discovered: strings.Join(gate, "; "),
			Help: "step=command, separated by ; — pin when discovery picks the wrong check"},
		{Key: "spec-store", Label: "spec store", Discovered: p.Spec.Store.Value,
			Help: "github-issues | gitlab | files | none — where specs and task graphs live"},
		{Key: "spec-ref", Label: "spec ref", Discovered: p.Spec.Ref.Value,
			Help: "owner/repo for a tracker, or a path"},
		{Key: "scopes", Label: "scopes", Discovered: strings.Join(p.Scopes.Values, ", "),
			Help: "comma-separated conventional-commit scopes"},
		{Key: "reviewers", Label: "reviewers", Discovered: strings.Join(p.Review.Values, ", "),
			Help: "comma-separated; only needed where there is no CODEOWNERS"},
		{Key: "merge", Label: "merge style", Discovered: p.Merge.Value,
			Help: "merge | squash | rebase"},
	}
}

// The form validates the same enumerated fields as the flags. A typed answer is
// no more trustworthy than a flag, and the file it lands in is committed.
func applyFields(cfg *repoconfig.Config, fields []tui.Field) error {
	for _, f := range fields {
		v := strings.TrimSpace(f.Value)
		if v == "" {
			continue
		}
		switch f.Key {
		case "gate":
			for _, pair := range strings.Split(v, ";") {
				step, command, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if !ok || step == "" || command == "" {
					continue
				}
				if cfg.Gate.Commands == nil {
					cfg.Gate.Commands = map[string]string{}
				}
				cfg.Gate.Commands[strings.TrimSpace(step)] = strings.TrimSpace(command)
			}
		case "spec-store":
			if !repoconfig.OneOf(v, specStores) {
				return fmt.Errorf("spec store %q: expected one of %s", v, strings.Join(specStores, ", "))
			}
			cfg.Spec.Store = v
		case "spec-ref":
			cfg.Spec.Ref = v
		case "scopes":
			cfg.Commit.Scopes = splitList(v)
		case "reviewers":
			cfg.Review.Reviewers = splitList(v)
		case "merge":
			if !repoconfig.OneOf(v, mergeStyles) {
				return fmt.Errorf("merge style %q: expected one of %s", v, strings.Join(mergeStyles, ", "))
			}
			cfg.Merge.Style = v
		}
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
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

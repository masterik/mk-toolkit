package sessionaudit

import (
	"sort"

	"github.com/masterik/mk-toolkit/internal/core/claudecfg"
)

// AttachConfig reads the settings that govern the scanned projects and notes,
// on each sandbox-block bucket, whether the config already matches it.
//
// The settings read are the user's, under home, and the project and local files
// of every project root the events ran in. A bucket is judged against the user's
// settings and the files of the roots it occurred in — not another project's.
// What the match means for a remedy is the skill's call: this only reports it.
func (r *Report) AttachConfig(home string) {
	seen := map[string]bool{}
	var all []string
	for _, p := range r.Projects {
		for _, root := range claudecfg.Roots(p.Paths) {
			if !seen[root] {
				seen[root] = true
				all = append(all, root)
			}
		}
	}
	sort.Strings(all)
	cfg := claudecfg.Load(home, "", all)
	r.Config = cfg

	rootOf := map[string]string{} // cwd -> project root; "" = no longer exists
	for i := range r.BlockTargets {
		b := &r.BlockTargets[i]
		b.CoveredBy, b.Protected = judgeBucket(cfg, b, rootOf)
	}
}

// judgeBucket: a bucket is covered only when every root it occurred in is —
// one worktree's local settings covering a target says nothing about another
// still hitting it, and "covered" tells the skill to propose nothing. Protected
// is a property of the path, so any root's verdict stands for it.
func judgeBucket(cfg *claudecfg.Result, b *Bucket, rootOf map[string]string) (*claudecfg.Cover, bool) {
	cwds := make([]string, 0, len(b.cwds))
	for c := range b.cwds {
		cwds = append(cwds, c)
	}
	sort.Strings(cwds)
	seen := map[string]bool{}
	var roots []string
	for _, c := range cwds {
		root, ok := rootOf[c]
		if !ok {
			if rs := claudecfg.Roots([]string{c}); len(rs) == 1 {
				root = rs[0]
			}
			rootOf[c] = root
		}
		if root != "" && !seen[root] {
			seen[root] = true
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		v := cfg.Judge(b.Key, nil) // the user's settings alone
		return v.Covered, v.Protected
	}
	var cover *claudecfg.Cover
	for i, root := range roots {
		v := cfg.Judge(b.Key, []string{root})
		if v.Protected {
			return nil, true
		}
		if v.Covered == nil {
			return nil, false
		}
		if i == 0 {
			cover = v.Covered
		}
	}
	return cover, false
}

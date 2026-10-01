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
	rootsOf := map[string][]string{}
	seen := map[string]bool{}
	var all []string
	for _, p := range r.Projects {
		roots := claudecfg.Roots(p.Paths)
		rootsOf[p.Project] = roots
		for _, root := range roots {
			if !seen[root] {
				seen[root] = true
				all = append(all, root)
			}
		}
	}
	sort.Strings(all)
	cfg := claudecfg.Load(home, "", all)
	r.Config = cfg

	for i := range r.BlockTargets {
		b := &r.BlockTargets[i]
		b.CoveredBy, b.Protected = judgeBucket(cfg, b, rootsOf)
	}
}

// judgeBucket: a bucket is covered only when every project it occurred in is —
// one project's local settings covering a target says nothing about another
// project still hitting it, and "covered" tells the skill to propose nothing.
// Protected is a property of the path, so any project's verdict stands for it.
func judgeBucket(cfg *claudecfg.Result, b *Bucket, rootsOf map[string][]string) (*claudecfg.Cover, bool) {
	var cover *claudecfg.Cover
	for _, p := range b.Projects {
		v := cfg.Judge(b.Key, rootsOf[p])
		if v.Protected {
			return nil, true
		}
		if v.Covered == nil {
			cover = nil
			break
		}
		if cover == nil {
			cover = v.Covered
		}
	}
	return cover, false
}

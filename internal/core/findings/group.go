package findings

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// GroupOptions tune the fold. Both gate a rule, so both are strict numerics.
type GroupOptions struct {
	MaxGroups float64
	MinPer    float64
}

// Group is one verification unit: a directory, or several folded together.
type Group struct {
	Slug  string   `json:"slug"`
	Name  string   `json:"name"`
	N     int      `json:"n"`
	IDs   []string `json:"ids"`
	File  string   `json:"file"`
	items []*Record
}

// Grouped is what `mkit findings group` returns. `suggest` is the documented
// threshold applied, and stays a suggestion: the skill decides.
type Grouped struct {
	Groups   []Group `json:"groups"`
	Findings int     `json:"findings"`
	Files    int     `json:"files"`
	Suggest  string  `json:"suggest"`
}

// GroupRun splits a reconciled run into verification groups and writes one
// verify-<slug>.jsonl per group.
func GroupRun(runDir string, opts GroupOptions) (*Grouped, error) {
	file := filepath.Join(runDir, "reconciled.jsonl")
	if _, err := os.Stat(file); err != nil {
		return nil, &InputError{Msg: "no reconciled.jsonl in " + runDir + " — run reconcile first"}
	}
	records, _, err := ReadJSONL(file)
	if err != nil {
		return nil, err
	}
	var defects []*Record
	for _, v := range records {
		r, ok := v.(*Record)
		if !ok {
			continue
		}
		if cls, ok := r.Get("class"); ok && truthy(cls) && r.Str("class") != "finding" {
			continue
		}
		defects = append(defects, r)
	}

	var order []string
	byDir := map[string][]*Record{}
	for _, r := range defects {
		d := path.Dir(r.Str("file"))
		if d == "" {
			d = "."
		}
		if _, seen := byDir[d]; !seen {
			order = append(order, d)
		}
		byDir[d] = append(byDir[d], r)
	}

	// Fold the smallest directories together until every group is worth a round
	// trip and there are no more than max-groups of them. One verifier per
	// *finding* is the failure mode this avoids: the subagent round trip costs
	// more than the verification.
	groups := make([]Group, 0, len(order))
	for _, d := range order {
		groups = append(groups, Group{Name: d, items: byDir[d]})
	}
	bySize := func() {
		sort.SliceStable(groups, func(i, j int) bool { return len(groups[i].items) > len(groups[j].items) })
	}
	bySize()
	for len(groups) > 1 &&
		(float64(len(groups)) > opts.MaxGroups || float64(len(groups[len(groups)-1].items)) < opts.MinPer) {
		small := groups[len(groups)-1]
		groups = groups[:len(groups)-1]
		target := &groups[len(groups)-1]
		target.items = append(target.items, small.items...)
		target.Name += "+"
		bySize()
	}

	res := &Grouped{Findings: len(defects), Groups: []Group{}}
	for i := range groups {
		g := &groups[i]
		g.Slug = fmt.Sprintf("g%d", i+1)
		g.N = len(g.items)
		g.File = filepath.Join(runDir, "verify-"+g.Slug+".jsonl")
		g.IDs = make([]string, len(g.items))
		for j, r := range g.items {
			g.IDs[j] = r.Str("id")
		}
		if err := WriteJSONL(g.File, g.items); err != nil {
			return nil, err
		}
		res.Groups = append(res.Groups, *g)
	}

	files := map[string]bool{}
	for _, r := range defects {
		files[r.Str("file")] = true
	}
	res.Files = len(files)
	res.Suggest = "fanout"
	if len(defects) <= 5 && len(groups) == 1 {
		res.Suggest = "inline"
	}
	return res, nil
}

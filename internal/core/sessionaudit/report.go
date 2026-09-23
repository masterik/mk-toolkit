package sessionaudit

import "sort"

// examplesPerBucket bounds the sample commands a bucket carries: enough for a
// reader to recognise the pattern, few enough that a report stays a summary.
const examplesPerBucket = 3

// Report is a scan's result: the counts, the groupings a settings change is
// decided from, and every event behind them.
type Report struct {
	Root  string `json:"root"`
	Days  int    `json:"days"`
	Since string `json:"since"`
	// Transcripts is the files read; WithEvents is how many had at least one.
	Transcripts int      `json:"transcripts"`
	WithEvents  int      `json:"with_events"`
	Unreadable  []string `json:"unreadable,omitempty"`

	Counts Counts `json:"counts"`
	// OverrideOutcomes is what happened to the calls that ran unsandboxed.
	OverrideOutcomes map[string]int `json:"override_outcomes"`
	// AutoModeReasons groups the classifier's refusals by the reason it gave.
	AutoModeReasons []Bucket `json:"automode_reasons"`
	// BlockTargets groups sandbox blocks by what was denied: a host, a path, or
	// the normalized EPERM line when the result named neither.
	BlockTargets []Bucket `json:"block_targets"`
	// BlockHeads and OverrideHeads group by the command that ran.
	BlockHeads    []Bucket        `json:"block_heads"`
	OverrideHeads []Bucket        `json:"override_heads"`
	Projects      []ProjectCounts `json:"projects"`

	// Events is every event behind the counts, each transcript's in order.
	Events []Event `json:"events,omitempty"`
}

// Counts is the headline numbers.
type Counts struct {
	SandboxBlocks       int `json:"sandbox_blocks"`
	Overrides           int `json:"overrides"`
	OverridesPreemptive int `json:"overrides_preemptive"`
	OverridesReadOnly   int `json:"overrides_read_only"`
	AutoModeDenials     int `json:"automode_denials"`
	RuleDenials         int `json:"rule_denials"`
	UserDenials         int `json:"user_denials"`
}

// Bucket is one grouping key, how often it occurred, where, and a few of the
// commands behind it.
type Bucket struct {
	Key      string   `json:"key"`
	Count    int      `json:"count"`
	Projects []string `json:"projects"`
	Examples []string `json:"examples"`
}

// ProjectCounts is the headline numbers for one project, worktrees folded in.
type ProjectCounts struct {
	Project string `json:"project"`
	// Paths is every working directory its sessions ran in — the checkout and
	// its worktrees — which is where that project's settings files are.
	Paths []string `json:"paths"`
	Counts
}

func (r *Report) aggregate() {
	r.OverrideOutcomes = map[string]int{}
	reasons, tgts, bheads, oheads := newGrouper(), newGrouper(), newGrouper(), newGrouper()
	projects := map[string]*Counts{}
	paths := map[string]map[string]bool{}

	for _, e := range r.Events {
		pc := projects[e.Project]
		if pc == nil {
			pc = &Counts{}
			projects[e.Project] = pc
			paths[e.Project] = map[string]bool{}
		}
		if e.Cwd != "" {
			paths[e.Project][e.Cwd] = true
		}
		for _, c := range []*Counts{&r.Counts, pc} {
			switch e.Kind {
			case KindSandboxBlock:
				c.SandboxBlocks++
			case KindOverride:
				c.Overrides++
				if e.Preemptive {
					c.OverridesPreemptive++
				}
				if e.ReadOnly {
					c.OverridesReadOnly++
				}
			case KindAutoModeDeny:
				c.AutoModeDenials++
			case KindRuleDeny:
				c.RuleDenials++
			case KindUserDeny:
				c.UserDenials++
			}
		}
		switch e.Kind {
		case KindSandboxBlock:
			bheads.add(e.Head, e)
			for _, t := range e.Targets {
				tgts.add(t, e)
			}
			if len(e.Targets) == 0 {
				tgts.add("(unnamed)", e)
			}
		case KindOverride:
			r.OverrideOutcomes[e.Outcome]++
			oheads.add(e.Head, e)
		case KindAutoModeDeny:
			reasons.add(e.Reason, e)
		}
	}

	r.AutoModeReasons = reasons.buckets()
	r.BlockTargets = tgts.buckets()
	r.BlockHeads = bheads.buckets()
	r.OverrideHeads = oheads.buckets()
	for _, p := range sortedKeys(projects) {
		r.Projects = append(r.Projects, ProjectCounts{Project: p, Paths: sortedKeys(paths[p]), Counts: *projects[p]})
	}
	sort.SliceStable(r.Projects, func(i, j int) bool {
		return total(r.Projects[i].Counts) > total(r.Projects[j].Counts)
	})
}

func total(c Counts) int {
	return c.SandboxBlocks + c.Overrides + c.AutoModeDenials + c.RuleDenials + c.UserDenials
}

type grouper struct {
	order []string
	byKey map[string]*Bucket
	projs map[string]map[string]bool
}

func newGrouper() *grouper {
	return &grouper{byKey: map[string]*Bucket{}, projs: map[string]map[string]bool{}}
}

func (g *grouper) add(key string, e Event) {
	if key == "" {
		key = "(" + e.Tool + ")"
	}
	b := g.byKey[key]
	if b == nil {
		b = &Bucket{Key: key}
		g.byKey[key] = b
		g.projs[key] = map[string]bool{}
		g.order = append(g.order, key)
	}
	b.Count++
	g.projs[key][e.Project] = true
	if len(b.Examples) < examplesPerBucket {
		b.Examples = append(b.Examples, clip(e.Command, 160))
	}
}

// buckets is the groups, most frequent first; ties keep the order they were
// first seen in, so two runs over the same transcripts print the same report.
func (g *grouper) buckets() []Bucket {
	out := make([]Bucket, 0, len(g.order))
	for _, k := range g.order {
		b := *g.byKey[k]
		for p := range g.projs[k] {
			b.Projects = append(b.Projects, p)
		}
		sort.Strings(b.Projects)
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

// sortedKeys is the map's keys, sorted, for output that does not reorder
// between runs.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

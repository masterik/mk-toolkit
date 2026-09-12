package findings

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// ReconcileOptions are the tunables. Every one of them gates a rule, which is
// why the flag parser refuses a missing value rather than defaulting.
type ReconcileOptions struct {
	SourcesExpected float64
	Sim             float64
	Band            float64
	Window          float64
}

// LowSim flags a merge whose two members barely share wording.
type LowSim struct {
	Sim   float64 `json:"sim"`
	Title string  `json:"title"`
}

// Merge describes one cluster that absorbed more than one finding.
type Merge struct {
	ID      string   `json:"id"`
	File    string   `json:"file"`
	Lines   []*int   `json:"lines"`
	Sources []string `json:"sources"`
	LowSim  []LowSim `json:"low_sim,omitempty"`
}

// DroppedItem is a weak singleton the drop rule removed.
type DroppedItem struct {
	File   string `json:"file"`
	Line   *int   `json:"line"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// AsideItem is a record that was never a defect in the change.
type AsideItem struct {
	ID    string `json:"id"`
	Class string `json:"class"`
	File  string `json:"file"`
	Line  *int   `json:"line"`
	Title string `json:"title"`
}

// PairSide names one end of an undecided pair.
type PairSide struct {
	ID    *string `json:"id"`
	File  string  `json:"file,omitempty"`
	Line  *int    `json:"line"`
	Title string  `json:"title"`
}

// ReviewPair is two findings in one file with similar wording at different
// lines: one shape at two sites, or two problems. The binary does not decide.
type ReviewPair struct {
	Sim float64  `json:"sim"`
	A   PairSide `json:"a"`
	B   PairSide `json:"b"`
}

// Count is one [surface, severity] tally.
type Count struct {
	Surface  string `json:"surface"`
	Severity string `json:"severity"`
	N        int    `json:"n"`
}

// Reconciled is what `mkit findings reconcile` returns.
type Reconciled struct {
	Wrote           string        `json:"wrote"`
	SourcesPresent  []string      `json:"sources_present"`
	SourcesExpected float64       `json:"sources_expected"`
	Complete        bool          `json:"complete"`
	In              int           `json:"in"`
	Findings        int           `json:"findings"`
	Merged          int           `json:"merged"`
	Dropped         int           `json:"dropped"`
	Aside           int           `json:"aside"`
	Counts          []Count       `json:"counts"`
	DropRule        string        `json:"drop_rule"`
	Merges          []Merge       `json:"merges"`
	DroppedItems    []DroppedItem `json:"dropped_findings"`
	AsideItems      []AsideItem   `json:"aside_records"`
	ReviewPairs     []ReviewPair  `json:"review_pairs"`
}

// InvalidError carries every problem found across a run's findings files.
type InvalidError struct{ Problems []string }

func (e *InvalidError) Error() string { return fmt.Sprintf("invalid=%d", len(e.Problems)) }

// pendingPair is an undecided pair held by pointer until ids exist.
type pendingPair struct {
	sim float64
	a   *cluster
	b   *Record
}

type cluster struct {
	head    *Record
	members []*Record
	lowSim  []LowSim
	out     *Record
}

// Reconcile merges, scores and drops a run's findings, writes reconciled.jsonl,
// and returns what happened.
func Reconcile(runDir string, opts ReconcileOptions) (*Reconciled, error) {
	files, err := RunFiles(runDir, "findings-")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, &InputError{Msg: "no findings-*.jsonl in " + runDir}
	}

	var all []*Record
	var problems []string
	sourcesSeen := map[string]bool{}
	for _, f := range files {
		records, parseErrs, err := ReadJSONL(f)
		if err != nil {
			return nil, err
		}
		problems = append(problems, parseErrs...)
		base := filepath.Base(f)
		fallback := strings.TrimSuffix(strings.TrimPrefix(base, "findings-"), ".jsonl")
		// A present file means that source reported, even with zero findings:
		// absent and present-but-empty must not collapse, or the drop rule
		// re-arms on a missing source.
		sourcesSeen[fallback] = true
		for i, v := range records {
			rec, ok := v.(*Record)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s#%d: not a JSON object", base, i+1))
				continue
			}
			// `||=`, not `??=`: an empty-string source also takes the fallback.
			if src, ok := rec.Get("source"); !ok || !truthy(src) {
				rec.Set("source", fallback)
			}
			sourcesSeen[rec.Str("source")] = true
			problems = append(problems, Validate(rec, fmt.Sprintf("%s#%d", base, i+1))...)
			all = append(all, rec)
		}
	}
	if len(problems) > 0 {
		return nil, &InvalidError{Problems: problems}
	}

	// 1. Split out what is not a defect in the change, before anything is merged.
	var aside, defects []*Record
	for _, r := range all {
		if cls, ok := r.Get("class"); ok && truthy(cls) && r.Str("class") != "finding" {
			aside = append(aside, r)
			continue
		}
		defects = append(defects, r)
	}

	// 2. Merge on the documented key: same file, within ±window lines. The text
	//    does not gate the merge — two reviewers describing one missing `await`
	//    at lines 42 and 43 share 0.23 of their tokens, so any similarity gate
	//    safe enough to trust would let every real duplicate through. Location
	//    decides. Nothing is discarded: every member's title and body rides
	//    along on the survivor in `also`, and thin merges are flagged.
	var clusters []*cluster
	var review []pendingPair
	clusterOf := map[*Record]*cluster{}
	for _, r := range defects {
		placed := false
		for _, c := range clusters {
			if !samePath(c.head.Str("file"), r.Str("file")) {
				continue
			}
			// Integers only: `line: null` is permitted, and treating it as 0
			// would merge two unrelated findings in the same file.
			headLine, headOK := c.head.Int("line")
			recLine, recOK := r.Int("line")
			near := headOK && recOK && math.Abs(float64(headLine-recLine)) <= opts.Window
			sim := similarity(c.head.tmpl("title"), c.head.tmpl("body"), r.tmpl("title"), r.tmpl("body"))
			if near {
				c.members = append(c.members, r)
				clusterOf[r] = c
				if sim < opts.Band {
					c.lowSim = append(c.lowSim, LowSim{Sim: sim, Title: r.Str("title")})
				}
				placed = true
				break
			}
			if sim >= opts.Sim {
				// `a` is the cluster head and `b` the candidate; both ids are
				// only known once the drop rule has run, so keep the pointers.
				review = append(review, pendingPair{sim: sim, a: c, b: r})
			}
		}
		if !placed {
			c := &cluster{head: r, members: []*Record{r}}
			clusters = append(clusters, c)
			clusterOf[r] = c
		}
	}

	// 3-5. Confidence from distinct sources, severity from the highest claim,
	//      and the weak-singleton drop — disabled outright when a source is missing.
	complete := float64(len(sourcesSeen)) >= opts.SourcesExpected
	var survivors []*Record
	var dropped []DroppedItem
	for _, c := range clusters {
		srcs := uniqueStrings(c.members, sourcesOf)
		lenses := uniqueStrings(c.members, lensesOf)
		bestConf := math.Inf(-1)
		for _, m := range c.members {
			if f, ok := m.Num("confidence"); ok {
				bestConf = math.Max(bestConf, f)
			} else {
				bestConf = math.Max(bestConf, 50)
			}
		}
		severity := "minor"
		for _, m := range c.members {
			severity = maxSev(severity, m.Str("severity"))
		}
		confidence := math.Min(99, bestConf+10*float64(len(srcs)-1))

		// Primary = the member that claims the most; the rest ride along in `also`.
		ordered := append([]*Record(nil), c.members...)
		sort.SliceStable(ordered, func(i, j int) bool {
			if d := sevRank(ordered[i].Str("severity")) - sevRank(ordered[j].Str("severity")); d != 0 {
				return d > 0
			}
			return confOr0(ordered[i]) > confOr0(ordered[j])
		})

		merged := ordered[0].Clone()
		if len(ordered) > 1 {
			also := make([]any, 0, len(ordered)-1)
			for _, m := range ordered[1:] {
				also = append(also, alsoEntry(m))
			}
			merged.Set("also", also)
		}
		merged.Set("sources", toAnyList(srcs))
		merged.Set("lenses", toAnyList(lenses))
		merged.Set("severity", severity)
		merged.Set("confidence", jsNumber(confidence))
		merged.Set("merged_count", jsNumber(float64(len(c.members))))
		merged.Set("class", "finding")
		merged.Delete("source")
		merged.Delete("lens")

		weak := len(srcs) == 1 && confidence < 80 && sevRank(severity) == 0
		if weak && complete {
			dropped = append(dropped, DroppedItem{
				File:  merged.Str("file"),
				Line:  lineOf(merged),
				Title: merged.Str("title"),
				Reason: fmt.Sprintf("single source %s, conf %s, minor",
					srcs[0], jsNumber(confidence)),
			})
			continue
		}
		c.out = merged
		survivors = append(survivors, merged)
	}

	// Stable ids: severity, then confidence, then location. Same input, same ids.
	// Byte order on `file`, deliberately — the script used localeCompare, which
	// sorts "a.ts" before "A.ts"; nothing else reads that order, and a locale is
	// not something an id should depend on.
	sort.SliceStable(survivors, func(i, j int) bool {
		a, b := survivors[i], survivors[j]
		if d := sevRank(b.Str("severity")) - sevRank(a.Str("severity")); d != 0 {
			return d < 0
		}
		if d := confOr0(b) - confOr0(a); d != 0 {
			return d < 0
		}
		if d := strings.Compare(a.Str("file"), b.Str("file")); d != 0 {
			return d < 0
		}
		return lineOr0(a) < lineOr0(b)
	})
	for i, r := range survivors {
		r.Set("id", fmt.Sprintf("f%02d", i+1))
	}
	for i, r := range aside {
		r.Set("id", fmt.Sprintf("x%02d", i+1))
	}

	out := filepath.Join(runDir, "reconciled.jsonl")
	if err := WriteJSONL(out, append(append([]*Record(nil), survivors...), aside...)); err != nil {
		return nil, err
	}

	res := &Reconciled{
		Wrote:           out,
		SourcesPresent:  sortedKeys(sourcesSeen),
		SourcesExpected: opts.SourcesExpected,
		Complete:        complete,
		In:              len(all),
		Findings:        len(survivors),
		Merged:          len(defects) - len(clusters),
		Dropped:         len(dropped),
		Aside:           len(aside),
		Counts:          tally(survivors),
		DropRule:        "enabled",
		DroppedItems:    dropped,
		Merges:          []Merge{},
		AsideItems:      []AsideItem{},
		ReviewPairs:     []ReviewPair{},
	}
	if res.DroppedItems == nil {
		res.DroppedItems = []DroppedItem{}
	}
	if res.SourcesPresent == nil {
		res.SourcesPresent = []string{}
	}
	if !complete {
		res.DropRule = "disabled"
	}
	for _, c := range clusters {
		if c.out == nil || len(c.members) < 2 {
			continue
		}
		lines := make([]*int, len(c.members))
		for i, m := range c.members {
			lines[i] = lineOf(m)
		}
		res.Merges = append(res.Merges, Merge{
			ID:      c.out.Str("id"),
			File:    c.out.Str("file"),
			Lines:   lines,
			Sources: recordStrings(c.out, "sources"),
			LowSim:  c.lowSim,
		})
	}
	for _, r := range aside {
		res.AsideItems = append(res.AsideItems, AsideItem{
			ID: r.Str("id"), Class: r.Str("class"), File: r.Str("file"),
			Line: lineOf(r), Title: r.Str("title"),
		})
	}
	for _, p := range review {
		a, b := sideOf(p.a.head), sideOf(p.b)
		a.ID = idOfCluster(p.a)
		b.ID = idOfCluster(clusterOf[p.b])
		b.File = ""
		res.ReviewPairs = append(res.ReviewPairs, ReviewPair{Sim: p.sim, A: a, B: b})
	}
	return res, nil
}

// ---------------------------------------------------------------- helpers

// Three independent tools write these files, and absolute-vs-repo-relative is
// exactly where they diverge. A raw `!=` left the same defect from two sources
// as two findings, costing the merge and the +10 corroboration boost — the whole
// point of this stage. Compare on a normalized tail.
func normPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimPrefix(p, "./")
	return strings.TrimLeft(p, "/")
}

func samePath(a, b string) bool {
	x, y := normPath(a), normPath(b)
	if x == y {
		return true
	}
	shorter, longer := x, y
	if len(y) < len(x) {
		shorter, longer = y, x
	}
	return shorter != "" && strings.HasSuffix(longer, "/"+shorter)
}

// sevRank ranks an unknown severity as minor, deliberately: an unrecognised
// claim must not outrank a real one.
func sevRank(s string) int {
	for i, v := range Severities {
		if v == s {
			return i
		}
	}
	return 0
}

func maxSev(a, b string) string {
	if sevRank(a) >= sevRank(b) {
		return a
	}
	return b
}

func sourcesOf(r *Record) []string {
	if v, ok := r.Get("sources"); ok {
		if arr, isArr := v.([]any); isArr {
			out := make([]string, 0, len(arr))
			for _, e := range arr {
				out = append(out, jsString(e))
			}
			return out
		}
	}
	if v, ok := r.Get("source"); ok && truthy(v) {
		return []string{jsString(v)}
	}
	return nil
}

func lensesOf(r *Record) []string {
	v, ok := r.Get("lenses")
	if !ok || v == nil {
		v, ok = r.Get("lens")
		if !ok {
			return nil
		}
	}
	if arr, isArr := v.([]any); isArr {
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			out = append(out, jsString(e))
		}
		return out
	}
	if truthy(v) {
		return []string{jsString(v)}
	}
	return nil
}

func uniqueStrings(members []*Record, f func(*Record) []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range members {
		for _, s := range f(m) {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func toAnyList(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func recordStrings(r *Record, key string) []string {
	v, ok := r.Get(key)
	if !ok {
		return nil
	}
	arr, isArr := v.([]any)
	if !isArr {
		return nil
	}
	out := make([]string, len(arr))
	for i, e := range arr {
		out[i] = jsString(e)
	}
	return out
}

func confOr0(r *Record) float64 {
	if f, ok := r.Num("confidence"); ok {
		return f
	}
	return 0
}

func lineOr0(r *Record) int {
	if n, ok := r.Int("line"); ok {
		return n
	}
	return 0
}

func lineOf(r *Record) *int {
	if n, ok := r.Int("line"); ok {
		return &n
	}
	return nil
}

// alsoEntry keeps a merged-away member visible. Keys absent on the member stay
// absent here, the way JSON.stringify drops an undefined value.
func alsoEntry(m *Record) *Record {
	e := NewRecord()
	for _, k := range []string{"source", "title", "body", "line"} {
		if v, ok := m.Get(k); ok {
			e.Set(k, v)
		}
	}
	return e
}

func sideOf(r *Record) PairSide {
	return PairSide{File: r.Str("file"), Line: lineOf(r), Title: r.Str("title")}
}

func idOfCluster(c *cluster) *string {
	if c == nil || c.out == nil {
		return nil
	}
	id := c.out.Str("id")
	return &id
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func tally(rs []*Record) []Count {
	type key struct{ surface, severity string }
	counts := map[key]int{}
	for _, r := range rs {
		counts[key{r.Str("surface"), r.Str("severity")}]++
	}
	out := make([]Count, 0, len(counts))
	for k, n := range counts {
		out = append(out, Count{Surface: k.surface, Severity: k.severity, N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Surface != out[j].Surface {
			return out[i].Surface < out[j].Surface
		}
		return out[i].Severity < out[j].Severity
	})
	return out
}

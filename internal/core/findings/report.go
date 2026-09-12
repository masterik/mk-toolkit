package findings

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// VerdictCount is one verdict bucket, in the order the verdicts first appeared.
type VerdictCount struct {
	Verdict string `json:"verdict"`
	N       int    `json:"n"`
}

// ReportItem is one finding as the report lists it.
type ReportItem struct {
	ID         string   `json:"id"`
	Surface    string   `json:"surface"`
	Severity   string   `json:"severity"`
	Confidence *float64 `json:"confidence"`
	Verdict    string   `json:"verdict,omitempty"`
	Class      string   `json:"class,omitempty"`
	File       string   `json:"file"`
	Line       *int     `json:"line"`
	Title      string   `json:"title"`
	Reason     string   `json:"reason,omitempty"`
	Sources    []string `json:"sources,omitempty"`
}

// Report is what `mkit findings report` returns.
type Report struct {
	Wrote        string         `json:"wrote"`
	VerdictFiles []string       `json:"verdict_files"`
	Verdicts     []VerdictCount `json:"verdicts"`
	Reportable   int            `json:"reportable"`
	Counts       []Count        `json:"counts"`
	Gating       int            `json:"gating"`
	Unverified   []string       `json:"unverified"`
	Orphans      []string       `json:"orphans"`
	Kept         []ReportItem   `json:"kept"`
	Decided      []ReportItem   `json:"decided"`
	Aside        []ReportItem   `json:"aside"`
}

// BuildReport merges verdicts onto reconciled findings, writes final.jsonl and
// returns the tally.
func BuildReport(runDir string) (*Report, error) {
	rec := filepath.Join(runDir, "reconciled.jsonl")
	if _, err := os.Stat(rec); err != nil {
		return nil, &InputError{Msg: "no reconciled.jsonl in " + runDir}
	}
	records, _, err := ReadJSONL(rec)
	if err != nil {
		return nil, err
	}
	var ids []string
	findings := map[string]*Record{}
	for _, v := range records {
		r, ok := v.(*Record)
		if !ok {
			continue
		}
		id := r.Str("id")
		if _, seen := findings[id]; !seen {
			ids = append(ids, id)
		}
		findings[id] = r
	}

	verdicts := map[string]*Record{}
	var orphans []string
	vfiles, err := RunFiles(runDir, "verdicts-")
	if err != nil {
		return nil, err
	}
	for _, f := range vfiles {
		vrecords, _, err := ReadJSONL(f)
		if err != nil {
			return nil, err
		}
		base := filepath.Base(f)
		for _, raw := range vrecords {
			v, ok := raw.(*Record)
			if !ok {
				orphans = append(orphans, base+": verdict for unknown id (none)")
				continue
			}
			id, hasID := v.Get("id")
			if !hasID || !truthy(id) {
				orphans = append(orphans, base+": verdict for unknown id (none)")
				continue
			}
			if _, known := findings[jsString(id)]; !known {
				orphans = append(orphans, base+": verdict for unknown id "+jsString(id))
				continue
			}
			if verdict, ok := v.Get("verdict"); !ok || !truthy(verdict) || !contains(Verdicts, jsString(verdict)) {
				// Do not store it: an unrecognised verdict that lands on the
				// finding satisfies the unverified check while meaning nothing.
				// Leave the id unverified and say so.
				name := "(none)"
				if ok && verdict != nil {
					name = jsString(verdict)
				}
				orphans = append(orphans, fmt.Sprintf("%s: %s unknown verdict %s — not applied", base, jsString(id), name))
				continue
			}
			key := jsString(id)
			prev := verdicts[key]
			if prev == nil {
				prev = NewRecord()
			} else {
				prev = prev.Clone()
			}
			for _, k := range v.Keys() {
				val, _ := v.Get(k)
				prev.Set(k, val)
			}
			verdicts[key] = prev
		}
	}

	merged := make([]*Record, 0, len(ids))
	for _, id := range ids {
		f := findings[id].Clone()
		v := verdicts[id]
		verdict := any(nil)
		reason := any(nil)
		if v != nil {
			verdict, _ = v.Get("verdict")
			reason, _ = v.Get("reason")
			// A `refined` verdict may correct fields; omitted fields stand.
			if jsString(verdict) == "refined" {
				for _, k := range v.Keys() {
					if k == "id" || k == "verdict" || k == "reason" {
						continue
					}
					val, _ := v.Get(k)
					f.Set(k, val)
				}
			}
		}
		f.Set("verdict", verdict)
		f.Set("verdict_reason", reason)
		merged = append(merged, f)
	}
	out := filepath.Join(runDir, "final.jsonl")
	if err := WriteJSONL(out, merged); err != nil {
		return nil, err
	}

	res := &Report{
		Wrote: out, Orphans: orphans,
		VerdictFiles: []string{}, Verdicts: []VerdictCount{}, Counts: []Count{},
		Unverified: []string{}, Kept: []ReportItem{}, Decided: []ReportItem{}, Aside: []ReportItem{},
	}
	if res.Orphans == nil {
		res.Orphans = []string{}
	}
	for _, f := range vfiles {
		res.VerdictFiles = append(res.VerdictFiles, filepath.Base(f))
	}

	var defects []*Record
	for _, r := range merged {
		if cls, ok := r.Get("class"); ok && truthy(cls) && r.Str("class") != "finding" {
			res.Aside = append(res.Aside, itemOf(r))
			continue
		}
		defects = append(defects, r)
	}

	var vorder []string
	vcounts := map[string]int{}
	for _, r := range defects {
		name := "no-verdict"
		if v, ok := r.Get("verdict"); ok && truthy(v) {
			name = jsString(v)
		}
		if _, seen := vcounts[name]; !seen {
			vorder = append(vorder, name)
		}
		vcounts[name]++
	}
	for _, name := range vorder {
		res.Verdicts = append(res.Verdicts, VerdictCount{Verdict: name, N: vcounts[name]})
	}

	var kept []*Record
	for _, r := range defects {
		switch r.Str("verdict") {
		case "confirmed", "refined":
			kept = append(kept, r)
			res.Kept = append(res.Kept, itemOf(r))
			if sevRank(r.Str("severity")) > 0 {
				res.Gating++
			}
		default:
			if v, ok := r.Get("verdict"); ok && truthy(v) {
				res.Decided = append(res.Decided, itemOf(r))
				continue
			}
			res.Unverified = append(res.Unverified, r.Str("id"))
		}
	}
	res.Reportable = len(kept)
	res.Counts = tally(kept)
	sort.SliceStable(res.Counts, func(i, j int) bool {
		if res.Counts[i].Surface != res.Counts[j].Surface {
			return res.Counts[i].Surface < res.Counts[j].Surface
		}
		return res.Counts[i].Severity < res.Counts[j].Severity
	})
	return res, nil
}

func itemOf(r *Record) ReportItem {
	it := ReportItem{
		ID: r.Str("id"), Surface: r.Str("surface"), Severity: r.Str("severity"),
		File: r.Str("file"), Line: lineOf(r), Title: r.Str("title"),
		Sources: recordStrings(r, "sources"),
	}
	if v, ok := r.Get("verdict"); ok && truthy(v) {
		it.Verdict = jsString(v)
	}
	if v, ok := r.Get("verdict_reason"); ok && truthy(v) {
		it.Reason = jsString(v)
	}
	if v, ok := r.Get("class"); ok && truthy(v) {
		it.Class = jsString(v)
	}
	if f, ok := r.Num("confidence"); ok {
		it.Confidence = &f
	}
	return it
}

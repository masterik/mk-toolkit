package findings

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// The closed vocabularies. Order matters for Severities: it is the rank.
var (
	Severities = []string{"minor", "major", "critical"}
	Surfaces   = []string{"code", "comments", "docs", "tests", "config", "build"}
	Verdicts   = []string{"confirmed", "refined", "rejected", "immaterial", "pre_existing"}
	Classes    = []string{"finding", "open_question", "pre_existing"}
)

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Validate returns one message per problem with a record, prefixed by where it
// came from. A JSONL line of `null`, `42` or `[]` parses fine; indexing it must
// report, not panic.
func Validate(v any, where string) []string {
	rec, ok := v.(*Record)
	if !ok {
		return []string{where + ": not a JSON object"}
	}
	var problems []string
	need := func(k string) {
		val, present := rec.Get(k)
		if !present || val == nil || val == "" {
			problems = append(problems, "missing "+k)
		}
	}
	for _, k := range []string{"surface", "severity", "file", "title"} {
		need(k)
	}
	if sev, ok := rec.Get("severity"); ok && truthy(sev) && !contains(Severities, jsString(sev)) {
		problems = append(problems, fmt.Sprintf("severity not one of %s: %s", strings.Join(Severities, "|"), jsString(sev)))
	}
	if surf, ok := rec.Get("surface"); ok && truthy(surf) && !contains(Surfaces, jsString(surf)) {
		problems = append(problems, fmt.Sprintf("surface not one of %s: %s", strings.Join(Surfaces, "|"), jsString(surf)))
	}
	if cls, ok := rec.Get("class"); ok && truthy(cls) && !contains(Classes, jsString(cls)) {
		problems = append(problems, "class unknown: "+jsString(cls))
	}
	if c, ok := rec.Get("confidence"); ok {
		n, isNum := c.(json.Number)
		f, ferr := float64(0), error(nil)
		if isNum {
			f, ferr = n.Float64()
		}
		if !isNum || ferr != nil || f < 0 || f > 100 {
			problems = append(problems, "confidence must be 0-100: "+jsString(c))
		}
	}
	if l, ok := rec.Get("line"); ok && l != nil {
		if _, isInt := rec.Int("line"); !isInt {
			problems = append(problems, "line must be an integer: "+jsString(l))
		}
	}
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = where + ": " + p
	}
	return out
}

// FileReport is one findings-*.jsonl file's validation result.
type FileReport struct {
	File    string   `json:"file"`
	Records int      `json:"records"`
	Errors  []string `json:"errors"`
}

// ValidateReport is what `mkit findings validate` returns.
type ValidateReport struct {
	Files  []FileReport `json:"files"`
	Errors int          `json:"errors"`
}

// ValidateRun validates every findings-*.jsonl in a run directory.
func ValidateRun(runDir string) (*ValidateReport, error) {
	files, err := RunFiles(runDir, "findings-")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, &InputError{Msg: "no findings-*.jsonl in " + runDir}
	}
	report := &ValidateReport{Files: []FileReport{}}
	for _, f := range files {
		records, parseErrs, err := ReadJSONL(f)
		if err != nil {
			return nil, err
		}
		base := filepath.Base(f)
		probs := append([]string{}, parseErrs...)
		for i, r := range records {
			probs = append(probs, Validate(r, fmt.Sprintf("%s#%d", base, i+1))...)
		}
		report.Errors += len(probs)
		report.Files = append(report.Files, FileReport{File: base, Records: len(records), Errors: probs})
	}
	return report, nil
}

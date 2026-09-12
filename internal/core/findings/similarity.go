package findings

import "strings"

var stopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(
		"a an the is are was were be been being this that these those it its of to in on for and or " +
			"not no if then than when where which with without into from at by as but can could should " +
			"would may might will shall do does did done has have had here there we you i") {
		m[w] = true
	}
	return m
}()

// tokenize lowercases, splits on anything that is not [a-z0-9_], and keeps
// tokens longer than two characters that are not stop words.
func tokenize(s string) map[string]bool {
	out := map[string]bool{}
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}
	for _, t := range strings.Split(b.String(), " ") {
		if len(t) > 2 && !stopWords[t] {
			out[t] = true
		}
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for t := range a {
		if b[t] {
			shared++
		}
	}
	return float64(shared) / float64(len(a)+len(b)-shared)
}

// similarity is the rounded Jaccard overlap of two findings' title+body.
func similarity(aTitle, aBody, bTitle, bBody string) float64 {
	return toFixed2(jaccard(tokenize(aTitle+" "+aBody), tokenize(bTitle+" "+bBody)))
}

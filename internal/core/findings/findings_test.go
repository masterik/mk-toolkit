package findings

import (
	"encoding/json"
	"testing"
)

// toFixed2 must round half away from zero on the exact binary value, the way
// `Number(x.toFixed(2))` does. Go's own FormatFloat rounds half to even, which
// turns a similarity of exactly 0.125 into 0.12 — and `sim` is compared against
// --sim and --band, so the boundary decides a merge.
func TestToFixed2RoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0.125, 0.13},
		{0.375, 0.38},
		{1.0 / 3.0, 0.33},
		{2.0 / 3.0, 0.67},
		{0, 0},
		{1, 1},
	}
	for _, c := range cases {
		if got := toFixed2(c.in); got != c.want {
			t.Errorf("toFixed2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSamePathAcceptsASegmentSuffixOnly(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"src/a.ts", "/repo/src/a.ts", true},
		{"./src/a.ts", "src/a.ts", true},
		{"src/a.ts", "src/a.ts", true},
		{"a.ts", "src/aa.ts", false},
		{"", "src/a.ts", false},
		{"src/a.ts", "src/b.ts", false},
	}
	for _, c := range cases {
		if got := samePath(c.a, c.b); got != c.want {
			t.Errorf("samePath(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// An unknown severity ranks as minor, deliberately: an unrecognised claim must
// not outrank a real one.
func TestSevRankFloorsUnknownSeverities(t *testing.T) {
	if sevRank("apocalyptic") != 0 || sevRank("critical") != 2 {
		t.Fatalf("ranks: %d %d", sevRank("apocalyptic"), sevRank("critical"))
	}
}

// A record keeps its key order and its number literals: order so a diff of
// final.jsonl stays readable, literals so `line: 42.5` is still not an integer.
func TestRecordRoundTripsOrderAndNumbers(t *testing.T) {
	const src = `{"z":1,"a":{"nested":true},"line":42.0,"big":1e3,"html":"a<b&c"}`
	var r Record
	if err := json.Unmarshal([]byte(src), &r); err != nil {
		t.Fatal(err)
	}
	got, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Fatalf("round trip: %s", got)
	}
	if n, ok := r.Int("line"); !ok || n != 42 {
		t.Fatalf("line: %v %v", n, ok)
	}
	r.Set("line", json.Number("42.5"))
	if _, ok := r.Int("line"); ok {
		t.Fatal("42.5 must not read as an integer")
	}
}

// Setting an existing key keeps its position; a new key appends — the positions
// a JS object spread produces, which is what makes the artefacts diffable.
func TestRecordSetKeepsPosition(t *testing.T) {
	r := NewRecord()
	r.Set("a", "1")
	r.Set("b", "2")
	r.Set("a", "3")
	r.Set("c", "4")
	if got := r.Keys(); got[0] != "a" || got[1] != "b" || got[2] != "c" || len(got) != 3 {
		t.Fatalf("keys: %v", got)
	}
	r.Delete("b")
	if got := r.Keys(); len(got) != 2 || got[1] != "c" {
		t.Fatalf("keys after delete: %v", got)
	}
}

// An absent body interpolates as the literal "undefined" — a token both sides
// of a comparison share, and the thresholds were measured with it present.
func TestTemplateInterpolationMatchesJS(t *testing.T) {
	r := NewRecord()
	r.Set("title", "x")
	if got := r.tmpl("body"); got != "undefined" {
		t.Fatalf("tmpl: %q", got)
	}
	r.Set("body", nil)
	if got := r.tmpl("body"); got != "null" {
		t.Fatalf("tmpl: %q", got)
	}
}

// JSON.parse rejects anything after the value; the decoder's More() does not see
// a stray closing delimiter, so `{"a":1}]` used to parse clean. A reviewer file
// that lost its enclosing array must be reported, not half-read.
func TestTrailingContentIsRejectedLikeJSONParse(t *testing.T) {
	for _, bad := range []string{`{"a":1}]`, `{"a":1}}`, `[1,2]]`, `{"a":1} {"b":2}`} {
		if _, err := parseJSON([]byte(bad)); err == nil {
			t.Fatalf("accepted trailing content: %s", bad)
		}
	}
	for _, ok := range []string{`{"a":1}`, `[1,2]`, `null`, `42`, ` {"a":1} `} {
		if _, err := parseJSON([]byte(ok)); err != nil {
			t.Fatalf("rejected valid JSON %s: %v", ok, err)
		}
	}
}

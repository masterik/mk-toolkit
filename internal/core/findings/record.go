// Package findings does the mechanical work on a review run's findings:
// validate, reconcile, group, report. It returns data and writes the run
// directory's artefacts; it never prints and never judges.
//
// What it does NOT do, on purpose: it never decides whether two findings are
// the same *problem* (it merges only on location and hands back a review list
// for the rest), never assigns severity, never judges materiality, and never
// edits a file. Those are the reviewer's and the skill's calls.
package findings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Record is a JSON object that remembers the order its keys arrived in.
//
// Records round-trip wholesale into reconciled.jsonl and final.jsonl, so a
// struct with known fields would silently drop `fix`, `also`, or anything a
// reviewer chose to add. Numbers stay json.Number: the line check must still
// reject 42.5 and accept 42, which float64 alone cannot tell apart.
type Record struct {
	keys []string
	vals map[string]any
}

// NewRecord returns an empty record.
func NewRecord() *Record { return &Record{vals: map[string]any{}} }

// Set stores a value, keeping an existing key in place and appending a new one —
// the same positions a JS object spread would produce.
func (r *Record) Set(key string, v any) {
	if r.vals == nil {
		r.vals = map[string]any{}
	}
	if _, ok := r.vals[key]; !ok {
		r.keys = append(r.keys, key)
	}
	r.vals[key] = v
}

// Get reports the value and whether the key is present at all. Absent is not the
// same as null: `line: null` is permitted, `line` missing is not a located finding.
func (r *Record) Get(key string) (any, bool) {
	if r == nil {
		return nil, false
	}
	v, ok := r.vals[key]
	return v, ok
}

// Delete removes a key and its position.
func (r *Record) Delete(key string) {
	if _, ok := r.vals[key]; !ok {
		return
	}
	delete(r.vals, key)
	for i, k := range r.keys {
		if k == key {
			r.keys = append(r.keys[:i], r.keys[i+1:]...)
			break
		}
	}
}

// Keys returns the key order.
func (r *Record) Keys() []string { return append([]string(nil), r.keys...) }

// Clone returns a shallow copy: same values, independent key order.
func (r *Record) Clone() *Record {
	out := NewRecord()
	for _, k := range r.keys {
		out.Set(k, r.vals[k])
	}
	return out
}

// Str returns the value as a string using JS `String(v)` semantics, or "" when
// the key is absent.
func (r *Record) Str(key string) string {
	v, ok := r.Get(key)
	if !ok {
		return ""
	}
	return jsString(v)
}

// Num returns the value as a float64 when it is a JSON number.
func (r *Record) Num(key string) (float64, bool) {
	v, ok := r.Get(key)
	if !ok {
		return 0, false
	}
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

// Int returns the value when it is a JSON number with no fractional part —
// `Number.isInteger`, which is what decides whether a finding is located.
func (r *Record) Int(key string) (int, bool) {
	f, ok := r.Num(key)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int(f), true
}

// MarshalJSON writes the keys in order, without Go's HTML escaping — a title
// containing `<` must survive the round trip unchanged.
func (r *Record) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range r.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := marshalValue(k)
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		vb, err := marshalValue(r.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON decodes an object, preserving key order and number literals.
func (r *Record) UnmarshalJSON(data []byte) error {
	v, err := parseJSON(data)
	if err != nil {
		return err
	}
	rec, ok := v.(*Record)
	if !ok {
		return fmt.Errorf("not a JSON object")
	}
	*r = *rec
	return nil
}

func marshalValue(v any) ([]byte, error) {
	switch t := v.(type) {
	case *Record:
		return t.MarshalJSON()
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			eb, err := marshalValue(e)
			if err != nil {
				return nil, err
			}
			b.Write(eb)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	default:
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimRight(b.Bytes(), "\n"), nil
	}
}

// parseJSON parses one complete JSON value, rejecting trailing content the way
// JSON.parse does.
func parseJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	// More() is false at a stray `]` or `}`, so it alone lets `{"a":1}]` through;
	// JSON.parse rejects it. Require the stream to actually be finished.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected trailing content")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return tok, nil
	}
	switch delim {
	case '{':
		rec := NewRecord()
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := kt.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			rec.Set(key, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return rec, nil
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("unexpected %v", delim)
}

// ---------------------------------------------------------------- js semantics

// jsString mirrors JS `String(v)` for the values a JSONL line can hold — what
// the messages and the source/severity comparisons were written against.
func jsString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			if e == nil {
				parts[i] = ""
				continue
			}
			parts[i] = jsString(e)
		}
		return strings.Join(parts, ",")
	case *Record:
		return "[object Object]"
	}
	return fmt.Sprint(v)
}

// truthy mirrors JS truthiness, which `||=` and `.filter(Boolean)` depend on:
// an empty-string source takes the fallback, a null lens contributes nothing.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case json.Number:
		f, err := t.Float64()
		return err == nil && f != 0
	}
	return true
}

// jsNumber renders a computed number the way JSON.stringify would — 80, not 80.0.
func jsNumber(f float64) json.Number {
	return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
}

// toFixed2 rounds to two decimals half-away-from-zero, on the exact binary value,
// the way JS `Number(x.toFixed(2))` does. Go's FormatFloat rounds half-to-even,
// which flips a similarity of exactly 0.125 to 0.12 — and `sim` is compared
// against --sim and --band, so a boundary value changes a merge.
func toFixed2(x float64) float64 {
	r := new(big.Rat).SetFloat64(x)
	if r == nil {
		return 0
	}
	r.Mul(r, big.NewRat(100, 1))
	neg := r.Sign() < 0
	if neg {
		r.Neg(r)
	}
	r.Add(r, big.NewRat(1, 2))
	n := new(big.Int).Quo(r.Num(), r.Denom())
	out := float64(n.Int64()) / 100
	if neg {
		return -out
	}
	return out
}

// ---------------------------------------------------------------- io

// ReadJSONL parses a JSONL file, returning the records it could read and one
// message per line it could not. Blank lines and `//` comments are skipped.
func ReadJSONL(path string) ([]any, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var out []any
	var errs []string
	base := filepath.Base(path)
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		v, err := parseJSON([]byte(line))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s:%d: %s", base, i+1, err))
			continue
		}
		out = append(out, v)
	}
	return out, errs, nil
}

// WriteJSONL writes one record per line, always ending with a newline.
func WriteJSONL(path string, records []*Record) error {
	var b bytes.Buffer
	for _, r := range records {
		line, err := r.MarshalJSON()
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// RunFiles lists a run directory's `<prefix>*.jsonl` files in name order.
func RunFiles(runDir, prefix string) ([]string, error) {
	info, err := os.Stat(runDir)
	if err != nil || !info.IsDir() {
		return nil, &UsageError{Msg: fmt.Sprintf("run directory does not exist: %s", runDir)}
	}
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, ".jsonl") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	for i, n := range out {
		out[i] = filepath.Join(runDir, n)
	}
	return out, nil
}

// UsageError is a caller mistake: exit 2, not 1.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// InputError is malformed or missing input: exit 1.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

// tmpl renders a field the way a JS template literal would: an absent key
// interpolates as the literal "undefined". That is not a quirk to clean up —
// it is a token both sides of a comparison share, and the similarity numbers
// the --sim and --band thresholds were measured against include it.
func (r *Record) tmpl(key string) string {
	v, ok := r.Get(key)
	if !ok {
		return "undefined"
	}
	return jsString(v)
}

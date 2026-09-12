package findings

// FieldDoc is one field of a record shape.
type FieldDoc struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
	Note     string   `json:"note,omitempty"`
}

// ShapeDoc is one file shape a reviewer or verifier writes.
type ShapeDoc struct {
	File   string     `json:"file"`
	Fields []FieldDoc `json:"fields"`
	Notes  []string   `json:"notes,omitempty"`
}

// SchemaDoc is the machine-readable contract `mkit findings` reads and writes.
// It doubles as the presence probe: a `mkit` too old to know `findings` fails
// here, at step 0, instead of halfway through a run.
type SchemaDoc struct {
	Finding ShapeDoc `json:"finding"`
	Verdict ShapeDoc `json:"verdict"`
}

// Schema returns the contract.
func Schema() SchemaDoc {
	return SchemaDoc{
		Finding: ShapeDoc{
			File: "findings-<source>.jsonl",
			Fields: []FieldDoc{
				{Name: "surface", Type: "string", Required: true, Enum: Surfaces},
				{Name: "severity", Type: "string", Required: true, Enum: Severities},
				{Name: "file", Type: "string", Required: true, Note: "repo-relative; an absolute path still merges, but one form across sources is what lets two reviewers corroborate"},
				{Name: "title", Type: "string", Required: true, Note: "names the mechanism"},
				{Name: "body", Type: "string", Note: "trigger + consequence"},
				{Name: "fix", Type: "string", Note: "concrete fix"},
				{Name: "line", Type: "integer"},
				{Name: "confidence", Type: "number", Note: "0-100; defaults to 50"},
				{Name: "lens", Type: "string|string[]"},
				{Name: "class", Type: "string", Enum: Classes, Note: `defaults to "finding"`},
				{Name: "source", Type: "string", Note: "defaults to the filename suffix"},
			},
			Notes: []string{
				"one JSON object per line, written by each reviewer",
				"unknown fields are preserved through reconcile and report",
			},
		},
		Verdict: ShapeDoc{
			File: "verdicts-<group>.jsonl",
			Fields: []FieldDoc{
				{Name: "id", Type: "string", Required: true, Note: "a finding id from reconciled.jsonl"},
				{Name: "verdict", Type: "string", Required: true, Enum: Verdicts},
				{Name: "reason", Type: "string", Note: "one clause"},
			},
			Notes: []string{
				"one file per verifier",
				`on "refined", any other field you include replaces the original; omitted fields stand`,
				"an unknown id or an unrecognised verdict is reported as an orphan and not applied",
			},
		},
	}
}

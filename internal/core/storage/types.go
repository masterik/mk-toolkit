package storage

import "time"

// Entry is one prunable thing: a file (KindFiles) or a whole stale directory
// (KindStaleDirs, IsDir true, Bytes the sum of every regular file beneath it).
type Entry struct {
	Path    string    `json:"path"`
	Bytes   int64     `json:"bytes"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

// PathError is one path Scan or Apply could not read or remove.
type PathError struct {
	Path string `json:"path"`
	Err  string `json:"error"`
}

// CategoryReport is one category's scan result: every prunable entry found,
// their total size, and any paths that could not be read.
type CategoryReport struct {
	Label   string      `json:"label"`
	Base    string      `json:"base"`
	Kind    Kind        `json:"kind"`
	Entries []Entry     `json:"entries"`
	Bytes   int64       `json:"bytes"`
	Errors  []PathError `json:"errors,omitempty"`
}

// ProviderReport is one provider's scan result across its category table.
// Categories whose base directory does not exist are omitted, not zeroed.
type ProviderReport struct {
	Name       string           `json:"name"`
	Home       string           `json:"home"`
	Present    bool             `json:"present"`
	Categories []CategoryReport `json:"categories"`
}

// Report is the full scan result: the --json payload, and what the TUI
// renders.
type Report struct {
	Days      int              `json:"days"`
	At        time.Time        `json:"at"`
	Providers []ProviderReport `json:"providers"`
}

// TotalBytes sums Bytes across every category in the report.
func (r *Report) TotalBytes() int64 {
	var total int64
	for _, p := range r.Providers {
		for _, c := range p.Categories {
			total += c.Bytes
		}
	}
	return total
}

// Package buildinfo holds version metadata set at build time via linker flags
// (-X), rather than in cmd/mkit/main, so the entrypoint stays free of it.
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

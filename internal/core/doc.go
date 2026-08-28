// Package core holds data-returning logic. It never prints and never assumes
// a terminal — that's cmd/'s job and internal/tui's job. Each subcommand's
// logic lives in its own subpackage (e.g. storage).
package core

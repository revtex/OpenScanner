// Package static embeds the frontend dist/ directory into the Go binary.
//
// The dist/ directory is expected to be copied into this package's directory
// before building (e.g. via `make build` in the project root).
// If dist/ does not exist at build time, the embedded FS will be empty.
package static

import "embed"

//go:embed all:dist
var DistFS embed.FS

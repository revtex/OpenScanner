// Package migrations embeds the SQL migration files so they can be used by the
// migration runner embedded in the db package.
package migrations

import "embed"

// FS contains all *.sql migration files in this directory.
//
//go:embed *.sql
var FS embed.FS

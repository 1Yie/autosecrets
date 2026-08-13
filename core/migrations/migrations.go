// Package migrations embeds the Core database schema migrations so the
// database layer can apply them at startup while Core remains a single
// binary (ADR-0001). The .sql files are numbered and applied in order.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

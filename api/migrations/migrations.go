// Package migrations embeds the goose SQL migrations.
package migrations

import "embed"

//go:embed *
var FS embed.FS

// Package migrations embeds the reviewed SQL migration history in the binary.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS

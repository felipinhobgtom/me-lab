package migrations

import "embed"

// FS embeds all SQL migration files so binary deployments and tests are self-contained.
//
//go:embed *.sql
var FS embed.FS

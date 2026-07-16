package migrations

import "embed"

// Files is embedded into the schema migration binary so deployments do not
// depend on copying a separate SQL directory to the server.
//
//go:embed *.up.sql *.down.sql
var Files embed.FS

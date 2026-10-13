package postgres

import (
	_ "embed"
)

//go:embed migrations/000001_init_schema.up.sql
var SchemaMigrationSQL string

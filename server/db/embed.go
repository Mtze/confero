package db

import "embed"

// Migrations contains the SQL migration files.
//
//go:embed migrations
var Migrations embed.FS

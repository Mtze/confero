//go:build tools

// tools.go pins tool dependencies so `go run` uses a reproducible version.
package main

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
)

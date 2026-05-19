// Package version exposes build-time metadata injected via -ldflags.
package version

// Version is the semantic version string, injected at build time.
var Version = ""

// Commit is the Git commit SHA, injected at build time.
var Commit = ""

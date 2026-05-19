package version_test

import (
	"testing"

	"confero/internal/version"
)

func TestVersionNotEmptyWhenSet(t *testing.T) {
	if version.Version == "" {
		t.Skip("Version not set via -ldflags; CI passes -X confero/internal/version.Version=<sha>")
	}
	if version.Version == "" {
		t.Fatal("version.Version must not be empty when injected via -ldflags")
	}
}

func TestCommitNotEmptyWhenSet(t *testing.T) {
	if version.Commit == "" {
		t.Skip("Commit not set via -ldflags; CI passes -X confero/internal/version.Commit=<sha>")
	}
	if version.Commit == "" {
		t.Fatal("version.Commit must not be empty when injected via -ldflags")
	}
}

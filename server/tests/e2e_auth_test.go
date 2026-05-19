//go:build compose

package tests_test

// End-to-end OIDC flow tests against a running Keycloak instance.
// Run with: go test -tags compose ./tests/... -run TestE2E
//
// These tests require the full docker-compose stack to be running:
//   make dev-services
//
// They are excluded from standard CI (go test ./...) and run only
// in the nightly compose job.

import (
	"net/http"
	"testing"
)

func TestE2EAuthLogin_RedirectsToKeycloak(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get("http://localhost:8080/auth/login")
	if err != nil {
		t.Skipf("compose stack not running: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	t.Logf("redirected to: %s", loc)
}

// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package mock

import (
	"testing"

	"github.com/rplevka/j9s/internal/client"
	"github.com/rplevka/j9s/internal/config"
)

// NewMockConfig builds a *config.Config with a single context named "test"
// pointing at the given Jenkins URL with TLS verification disabled. Mirrors
// the k9s internal/config/mock.NewMockConfig helper.
func NewMockConfig(t testing.TB, jenkinsURL string) *config.Config {
	t.Helper()
	cfg := config.NewConfig()
	cfg.J9s.CurrentContext = "test"
	cfg.J9s.Contexts = []config.Context{{
		Name:     "test",
		URL:      jenkinsURL,
		Insecure: true,
	}}
	return cfg
}

// NewClient constructs a *client.Client wired to the given mock server's URL.
// CSRF crumb is fetched (the mock server always responds), so the client is
// ready to make authenticated requests.
func NewClient(t testing.TB, srv *JenkinsServer) *client.Client {
	t.Helper()
	cfg := NewMockConfig(t, srv.URL())
	ctx, err := cfg.ActiveContext()
	if err != nil {
		t.Fatalf("active context: %v", err)
	}
	c, err := client.NewClient(ctx)
	if err != nil {
		t.Fatalf("client.NewClient: %v", err)
	}
	return c
}

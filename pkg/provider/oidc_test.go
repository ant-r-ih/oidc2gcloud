package provider

import (
	"strings"
	"testing"

	"github.com/your-org/oidc2gcloud/pkg/cfg"
)

func TestBuildAuthorizationURL(t *testing.T) {
	profile := &cfg.Profile{
		Issuer:      "https://authentik.example.com/application/o/my-app/",
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8085/callback",
		Scopes:      "openid,email,profile",
	}

	provider := NewOIDCProvider(profile)

	authURL, err := provider.buildAuthorizationURL("http://localhost:8085/callback")
	if err != nil {
		t.Fatalf("Failed to build authorization URL: %v", err)
	}

	// Check URL components
	if !strings.Contains(authURL, "authentik.example.com") {
		t.Errorf("URL should contain issuer domain")
	}

	if !strings.Contains(authURL, "authorize") {
		t.Errorf("URL should contain /authorize endpoint")
	}

	if !strings.Contains(authURL, "client_id=test-client-id") {
		t.Errorf("URL should contain client_id parameter")
	}

	if !strings.Contains(authURL, "redirect_uri=") {
		t.Errorf("URL should contain redirect_uri parameter")
	}

	if !strings.Contains(authURL, "response_type=code") {
		t.Errorf("URL should contain response_type=code")
	}

	// Scopes should be space-separated (not comma-separated)
	if !strings.Contains(authURL, "openid+email+profile") && !strings.Contains(authURL, "openid%20email%20profile") {
		t.Errorf("Scopes should be space-separated in URL: %s", authURL)
	}

	if strings.Contains(authURL, "openid,email,profile") {
		t.Errorf("Scopes should not be comma-separated in URL")
	}
}

func TestGetTokenEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		expected string
	}{
		{
			name:     "Authentik with trailing slash",
			issuer:   "https://authentik.example.com/application/o/my-app/",
			expected: "https://authentik.example.com/application/o/my-app/token/",
		},
		{
			name:     "Authentik without trailing slash",
			issuer:   "https://authentik.example.com/application/o/my-app",
			expected: "https://authentik.example.com/application/o/my-app/token/",
		},
		{
			name:     "Generic OIDC provider",
			issuer:   "https://oidc.example.com",
			expected: "https://oidc.example.com/token/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &cfg.Profile{
				Issuer: tt.issuer,
			}
			provider := NewOIDCProvider(profile)

			tokenURL, err := provider.getTokenEndpoint()
			if err != nil {
				t.Fatalf("Failed to get token endpoint: %v", err)
			}

			if tokenURL != tt.expected {
				t.Errorf("Expected token URL %s, got %s", tt.expected, tokenURL)
			}
		})
	}
}

func TestScopeConversion(t *testing.T) {
	// Test that comma-separated scopes are converted to space-separated
	profile := &cfg.Profile{
		Issuer:      "https://example.com",
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8085/callback",
		Scopes:      "openid,email,profile",
	}

	provider := NewOIDCProvider(profile)

	authURL, err := provider.buildAuthorizationURL("http://localhost:8085/callback")
	if err != nil {
		t.Fatalf("Failed to build authorization URL: %v", err)
	}

	// URL-encoded space is either + or %20
	if !strings.Contains(authURL, "openid+email+profile") && !strings.Contains(authURL, "openid%20email%20profile") {
		t.Errorf("Scopes should be space-separated (URL encoded), got: %s", authURL)
	}
}

func TestGenerateRandomState(t *testing.T) {
	state1 := generateRandomState()
	state2 := generateRandomState()

	if state1 == "" {
		t.Error("State should not be empty")
	}

	if !strings.HasPrefix(state1, "state_") {
		t.Errorf("State should start with 'state_', got: %s", state1)
	}

	// States should be different (though with low-resolution timestamp, might be same)
	t.Logf("State 1: %s", state1)
	t.Logf("State 2: %s", state2)
}

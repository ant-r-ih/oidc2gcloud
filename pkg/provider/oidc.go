package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/your-org/oidc2gcloud/pkg/browser"
	"github.com/your-org/oidc2gcloud/pkg/cfg"
	"github.com/your-org/oidc2gcloud/pkg/creds"
)

// OIDCProvider implements OIDC authentication flow
type OIDCProvider struct {
	profile        *cfg.Profile
	client         *http.Client
	browserAutofill bool
}

// NewOIDCProvider creates a new OIDC provider
func NewOIDCProvider(profile *cfg.Profile) *OIDCProvider {
	return &OIDCProvider{
		profile:        profile,
		browserAutofill: os.Getenv("OIDC2GCLOUD_BROWSER_AUTOFILL") != "",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate performs the OIDC authentication flow
func (p *OIDCProvider) Authenticate() (*creds.OIDCToken, error) {
	// Start callback server
	callbackServer, err := browser.NewCallbackServer(8085)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start callback server")
	}
	defer callbackServer.Shutdown()

	callbackServer.Start()

	// Build authorization URL
	authURL, err := p.buildAuthorizationURL(callbackServer.GetRedirectURI())
	if err != nil {
		return nil, errors.Wrap(err, "failed to build authorization URL")
	}

	// Open browser
	fmt.Printf("Opening browser for authentication...\n")
	if err := p.openBrowser(authURL); err != nil {
		fmt.Printf("⚠️  Failed to open browser: %v\n", err)
		fmt.Printf("\nPlease open this URL manually:\n%s\n\n", authURL)
	}

	// Wait for authorization code
	fmt.Println("Waiting for authentication callback...")
	code, err := callbackServer.WaitForCode(5 * time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive authorization code")
	}

	fmt.Println("✓ Authorization code received")

	// Exchange code for tokens
	token, err := p.exchangeCodeForToken(code, callbackServer.GetRedirectURI())
	if err != nil {
		return nil, errors.Wrap(err, "failed to exchange code for token")
	}

	fmt.Println("✓ Tokens obtained successfully")

	return token, nil
}

func (p *OIDCProvider) buildAuthorizationURL(redirectURI string) (string, error) {
	authURL, err := url.Parse(p.profile.Issuer)
	if err != nil {
		return "", errors.Wrap(err, "invalid issuer URL")
	}

	// Typically, authorization endpoint is at /authorize
	// For Authentik, it's at the application URL
	if !strings.HasSuffix(authURL.Path, "/authorize") && !strings.Contains(authURL.Path, "/authorize/") {
		if strings.HasSuffix(authURL.Path, "/") {
			authURL.Path = authURL.Path + "authorize/"
		} else {
			authURL.Path = authURL.Path + "/authorize/"
		}
	}

	params := url.Values{}
	params.Set("client_id", p.profile.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	// Convert comma-separated scopes to space-separated (OIDC standard)
	params.Set("scope", strings.ReplaceAll(p.profile.Scopes, ",", " "))

	// Add state for CSRF protection (optional but recommended)
	params.Set("state", generateRandomState())

	authURL.RawQuery = params.Encode()

	return authURL.String(), nil
}

func (p *OIDCProvider) exchangeCodeForToken(code, redirectURI string) (*creds.OIDCToken, error) {
	tokenURL, err := p.getTokenEndpoint()
	if err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", p.profile.ClientID)
	data.Set("client_secret", p.profile.ClientSecret)
	// Convert comma-separated scopes to space-separated (OIDC standard)
	data.Set("scope", strings.ReplaceAll(p.profile.Scopes, ",", " "))

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exchange code for token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read token response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token creds.OIDCToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.Wrap(err, "failed to parse token response")
	}

	return &token, nil
}

func (p *OIDCProvider) getTokenEndpoint() (string, error) {
	tokenURL, err := url.Parse(p.profile.Issuer)
	if err != nil {
		return "", errors.Wrap(err, "invalid issuer URL")
	}

	// Typically, token endpoint is at /token
	if !strings.HasSuffix(tokenURL.Path, "/token") && !strings.Contains(tokenURL.Path, "/token/") {
		if strings.HasSuffix(tokenURL.Path, "/") {
			tokenURL.Path = tokenURL.Path + "token/"
		} else {
			tokenURL.Path = tokenURL.Path + "/token/"
		}
	}

	return tokenURL.String(), nil
}

func generateRandomState() string {
	// Simple state generation - in production, use crypto/rand
	return fmt.Sprintf("state_%d", time.Now().Unix())
}

// openBrowser opens the authorization URL in the default browser
func (p *OIDCProvider) openBrowser(authURL string) error {
	var cmd *exec.Cmd

	// Check if browser autofill is enabled
	if p.browserAutofill {
		// Use custom browser command with autofill
		browserCmd := os.Getenv("OIDC2GCLOUD_BROWSER_CMD")
		if browserCmd == "" {
			return errors.New("OIDC2GCLOUD_BROWSER_AUTOFILL is set but OIDC2GCLOUD_BROWSER_CMD is not defined")
		}

		fmt.Printf("Using custom browser command for autofill\n")
		// Execute the custom command with URL as argument
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s '%s'", browserCmd, authURL))
	} else {
		// Standard browser opening
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", authURL)
		case "linux":
			cmd = exec.Command("xdg-open", authURL)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", authURL)
		default:
			return errors.New("unsupported platform")
		}
	}

	cmd.Stderr = os.Stderr
	return cmd.Start()
}
